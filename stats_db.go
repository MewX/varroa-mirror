package varroa

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/jinzhu/now"
	"github.com/pkg/errors"
	"github.com/wcharczuk/go-chart"
	"gitlab.com/catastrophic/assistance/logthis"
	"gitlab.com/catastrophic/assistance/strslice"
)

var statsDB *StatsDB
var onceStatsDB sync.Once

type StatsDB struct {
	db *Database
}

func NewStatsDB(path string) (*StatsDB, error) {
	var returnErr error
	onceStatsDB.Do(func() {
		// db should be opened already
		db, err := NewDatabase(path)
		if err != nil {
			returnErr = errors.Wrap(err, "Error opening stats database")
			return
		}
		statsDB = &StatsDB{db: db}
		if returnErr = statsDB.init(); returnErr != nil {
			return
		}

		conf, err := NewConfig(DefaultConfigurationFile)
		if err != nil {
			logthis.Error(err, logthis.NORMAL)
			returnErr = err
			return
		} else {
			// try to import <v19
			migratedSomething := false
			for _, label := range conf.TrackerLabels() {
				migrated, err := statsDB.migrate(label)
				if err != nil && err != storm.ErrNotFound {
					logthis.Error(errors.Wrap(err, "Error migrating database to a new schema, for tracker "+label), logthis.VERBOSEST)
				} else if migrated {
					migratedSomething = migrated
				}
			}
			if migratedSomething {
				logthis.Info("Updating stats after migration", logthis.NORMAL)
				returnErr = statsDB.Update()
				return
			}
		}
	})
	return statsDB, returnErr
}

func (sdb *StatsDB) init() error {
	if err := sdb.db.DB.Init(&StatsEntry{}); err != nil {
		return err
	}
	if err := sdb.db.DB.Init(&SnatchStatsEntry{}); err != nil {
		return err
	}
	return sdb.db.DB.Init(&Release{})
}

func (sdb *StatsDB) migrate(tracker string) (bool, error) {
	var migratedSchema bool
	var err error

	// updating schema for stats
	var allEntries []StatsEntry
	if err = sdb.db.DB.Find("Tracker", tracker, &allEntries); err != nil {
		if err == storm.ErrNotFound {
			return migratedSchema, nil
		}
		return migratedSchema, errors.Wrap(err, "Error reading stats values from db")
	}

	// transaction for quicker results
	txSchemaUpdate, err := sdb.db.DB.Begin(true)
	if err != nil {
		return migratedSchema, err
	}
	defer txSchemaUpdate.Rollback()

	for _, e := range allEntries {
		if e.SchemaVersion != currentStatsDBSchemaVersion {
			migratedSchema = true
			// Update multiple fields
			if err = txSchemaUpdate.Update(&StatsEntry{ID: e.ID, SchemaVersion: currentStatsDBSchemaVersion, TimestampUnix: e.Timestamp.Unix()}); err != nil {
				return migratedSchema, errors.Wrap(err, "Error updating stats database to new schema")
			}
		}
	}
	if migratedSchema {
		err = txSchemaUpdate.Commit()
	}
	return migratedSchema, err
}

// Update needs to be called everyday at midnight (add cron job)
// It creates StatsEntries for each start of day (which might also be start of week/month)
// That way, it'll be quicker to create stats and recalculate StatsDeltas.
func (sdb *StatsDB) Update() error {
	defer TimeTrack(time.Now(), "UPDATE")
	// TODO: add cron job

	// get tracker labels from config.
	conf, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		return err
	}
	allTrackers := conf.TrackerLabels()

	// transaction for quicker results
	tx, err := sdb.db.DB.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// get start of today
	startOfToday := now.BeginningOfDay()
	for _, t := range allTrackers {
		logthis.Info("Updating stats for tracker "+t, logthis.VERBOSEST)

		// find first collected timestamp for this tracker
		firstTrackerStats, err := sdb.getFirstStatsForTracker(t)
		if err != nil {
			logthis.Info("Could not find stats for tracker "+t, logthis.VERBOSEST)
			continue
		}
		startOfFirstDay := now.New(firstTrackerStats.Timestamp).BeginningOfDay()
		if startOfFirstDay.After(startOfToday) {
			logthis.Info("Incoherent daily stats: some stats in the future for tracker "+t, logthis.VERBOSEST)
			continue
		}

		// check stats exist from the first day to today
		for currentStartOfDay := startOfFirstDay; !currentStartOfDay.Equal(startOfToday); currentStartOfDay = now.New(currentStartOfDay).AddDate(0, 0, 1) {
			// look for daily entries in the db.
			var entryForThisDay StatsEntry
			if err := sdb.db.DB.Select(q.And(q.Eq("StartOfDay", true), q.Eq("Tracker", t), q.Eq("Timestamp", currentStartOfDay))).First(&entryForThisDay); err != nil {
				if err == storm.ErrNotFound {
					// if not found, create
					entryForThisDay = StatsEntry{Tracker: t, StartOfDay: true}

					// get closest collected stats before midnight & after, and do some simple linear interpolation
					// get previous collected stats
					var previous StatsEntry
					if selectErr := sdb.db.DB.Select(q.And(q.Eq("Tracker", t), q.Eq("Collected", true), q.Lte("Timestamp", currentStartOfDay))).OrderBy("TimestampUnix").Reverse().First(&previous); selectErr != nil {
						if selectErr == storm.ErrNotFound {
							// first day, use the first known stats as the reference
							previous = firstTrackerStats
						} else {
							logthis.Error(selectErr, logthis.VERBOSEST)
							continue
						}
					}
					// get following collected stats
					var next StatsEntry
					if selectErr := sdb.db.DB.Select(q.And(q.Eq("Tracker", t), q.Eq("Collected", true), q.Gte("Timestamp", currentStartOfDay))).OrderBy("TimestampUnix").First(&next); selectErr != nil {
						if selectErr == storm.ErrNotFound {
							// last day, missing information to create the daily stats
							continue
						} else {
							logthis.Error(selectErr, logthis.VERBOSEST)
							continue
						}
					}

					// calculate the stats at start of day
					var newDailyStats *StatsEntry
					var statsErr error
					if previous.Timestamp.Equal(next.Timestamp) {
						// if they are the same, it's probably the first day.
						// creating first day without interpolation.
						newDailyStats = &StatsEntry{Tracker: previous.Tracker, Timestamp: currentStartOfDay, TimestampUnix: currentStartOfDay.Unix(), Up: previous.Up, Down: previous.Down, Ratio: previous.Ratio, StartOfDay: true}
					} else {
						// interpolate stats at the start of the day being considered
						newDailyStats, statsErr = InterpolateStats(previous, next, currentStartOfDay)
						if statsErr != nil {
							logthis.Error(statsErr, logthis.VERBOSE)
							continue
						} else {
							newDailyStats.StartOfDay = true
						}
					}
					// if the new day stats is the start of an iso week, StartOfWeek = true
					if currentStartOfDay.Equal(now.New(currentStartOfDay).BeginningOfWeek()) {
						newDailyStats.StartOfWeek = true
					}
					// if the new day stats is the start of a month, StartOfMonth = true
					if currentStartOfDay.Equal(now.New(currentStartOfDay).BeginningOfMonth()) {
						newDailyStats.StartOfMonth = true
					}
					// save new entry
					if saveErr := tx.Save(newDailyStats); saveErr != nil {
						return errors.Wrap(saveErr, "error saving daily stats")
					}
					logthis.Info("Added daily stats for "+t+"/"+currentStartOfDay.String(), logthis.VERBOSEST)
				} else {
					logthis.Error(err, logthis.VERBOSEST)
				}
			}

			// look for snatch daily entries in the db.
			var snatchEntryForThisDay SnatchStatsEntry
			if err := sdb.db.DB.Select(q.And(q.Eq("StartOfDay", true), q.Eq("Tracker", t), q.Eq("Timestamp", currentStartOfDay))).First(&snatchEntryForThisDay); err != nil {
				if err == storm.ErrNotFound {
					// if not found, create
					snatchEntryForThisDay = SnatchStatsEntry{Tracker: t, StartOfDay: true, Timestamp: currentStartOfDay}

					// get snatches for this day
					var newSnatches []Release
					if selectErr := sdb.db.DB.Select(q.And(q.Eq("Tracker", t), q.Gte("Timestamp", currentStartOfDay), q.Lte("Timestamp", now.New(currentStartOfDay).AddDate(0, 0, 1)))).Find(&newSnatches); selectErr != nil {
						// if nothing found, no snatches for this day, empty entry will be added
						if selectErr != storm.ErrNotFound {
							logthis.Error(selectErr, logthis.VERBOSEST)
							continue
						}
					} else {
						// calculating stats for this day
						snatchEntryForThisDay.Number = len(newSnatches)
						for _, s := range newSnatches {
							snatchEntryForThisDay.Size += s.Size
						}
					}
					// if the new day stats is the start of an iso week, StartOfWeek = true
					if currentStartOfDay.Equal(now.New(currentStartOfDay).BeginningOfWeek()) {
						snatchEntryForThisDay.StartOfWeek = true
					}
					// if the new day stats is the start of a month, StartOfMonth = true
					if currentStartOfDay.Equal(now.New(currentStartOfDay).BeginningOfMonth()) {
						snatchEntryForThisDay.StartOfMonth = true
					}
					// save new entry
					if saveErr := tx.Save(&snatchEntryForThisDay); saveErr != nil {
						return errors.Wrap(saveErr, "error saving daily snatch stats")
					}

					logthis.Info("Added daily snatch stats for "+t+"/"+currentStartOfDay.String(), logthis.VERBOSEST)
				} else {
					logthis.Error(err, logthis.VERBOSEST)
				}
			}
		}
	}
	// committing to db
	return tx.Commit()
}

func (sdb *StatsDB) FilterByTracker(tracker string, statsType string) ([]StatsEntry, error) {
	// TODO to avoid making huge lists, use Limit(n).Skip(i*n)?

	if !strslice.Contains([]string{"Collected", "StartOfDay", "StartOfWeek", "StartOfMonth"}, statsType) {
		return []StatsEntry{}, errors.New("Unknown stats type: " + statsType)
	}

	var entries []StatsEntry
	err := sdb.db.DB.Select(q.And(q.Eq(statsType, true), q.Eq("Tracker", tracker))).OrderBy("TimestampUnix").Find(&entries)
	return entries, err
}

func (sdb *StatsDB) Save(entry *StatsEntry) error {
	return sdb.db.DB.Save(entry)
}

func (sdb *StatsDB) getFirstStatsForTracker(tracker string) (StatsEntry, error) {
	var firstEntry StatsEntry
	err := sdb.db.DB.Select(q.And(q.Eq("Collected", true), q.Eq("Tracker", tracker))).OrderBy("TimestampUnix").First(&firstEntry)
	return firstEntry, err
}

func (sdb *StatsDB) GetLastCollected(tracker string, limit int) ([]StatsEntry, error) {
	var lastEntries []StatsEntry
	err := sdb.db.DB.Select(q.And(q.Eq("Collected", true), q.Eq("Tracker", tracker))).Limit(limit).OrderBy("TimestampUnix").Reverse().Find(&lastEntries)
	return lastEntries, err
}

func (sdb *StatsDB) GenerateAllGraphsForTracker(tracker string) error {
	atLeastOneFailed := false

	firstStats, err := sdb.getFirstStatsForTracker(tracker)
	if err != nil {
		return errors.Wrap(err, "error getting tracker stats")
	}

	// 1. overall collected stats
	allStatsEntries, err := sdb.FilterByTracker(tracker, "Collected")
	if err != nil {
		return errors.New("could not get all collected stats for " + tracker)
	}
	if err := generateGraphs(tracker, overallPrefix, allStatsEntries, firstStats.Timestamp); err != nil {
		return err
	}
	// 2. collect stats since last week
	// get the timestamp for one week earlier
	firstWeekTimestamp := time.Now().Add(-7 * 24 * time.Hour)
	lastWeekQuery := q.And(q.Eq("Collected", true), q.Eq("Tracker", tracker), q.Gte("Timestamp", firstWeekTimestamp))
	var lastWeekStatsEntries []StatsEntry
	if err := sdb.db.DB.Select(lastWeekQuery).OrderBy("TimestampUnix").Find(&lastWeekStatsEntries); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	if len(lastWeekStatsEntries) != 0 {
		if err := generateGraphs(tracker, lastWeekPrefix, lastWeekStatsEntries, lastWeekStatsEntries[0].Timestamp); err != nil {
			logthis.Error(err, logthis.NORMAL)
			atLeastOneFailed = true
		}
	}

	// 3. collect stats since last month
	// get the timestamp for one month earlier
	firstMonthTimestamp := time.Now().Add(-30 * 24 * time.Hour)
	lastMonthQuery := q.And(q.Eq("Collected", true), q.Eq("Tracker", tracker), q.Gte("Timestamp", firstMonthTimestamp))
	var lastMonthStatsEntries []StatsEntry
	if err := sdb.db.DB.Select(lastMonthQuery).OrderBy("TimestampUnix").Find(&lastMonthStatsEntries); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	if len(lastMonthStatsEntries) != 0 {
		if err := generateGraphs(tracker, lastMonthPrefix, lastMonthStatsEntries, lastMonthStatsEntries[0].Timestamp); err != nil {
			logthis.Error(err, logthis.NORMAL)
			atLeastOneFailed = true
		}
	}

	// 4. stats/day
	// get all daily stats
	var allDailyStats []StatsEntry
	dailyStatsQuery := q.And(q.Eq("StartOfDay", true), q.Eq("Tracker", tracker))
	if err := sdb.db.DB.Select(dailyStatsQuery).OrderBy("TimestampUnix").Find(&allDailyStats); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	allDailyDeltas := CalculateDeltas(allDailyStats)
	// generate graphs
	if len(allDailyDeltas) != 0 {
		if err := generateDeltaGraphs(tracker, overallPrefix+"_per_day", allDailyDeltas, allDailyDeltas[0].Timestamp); err != nil {
			logthis.Error(err, logthis.NORMAL)
			atLeastOneFailed = true
		}
	}

	// 5. stats/week
	// get all weekly stats
	var allWeeklyStats []StatsEntry
	weeklyStatsQuery := q.And(q.Eq("StartOfWeek", true), q.Eq("Tracker", tracker))
	if err := sdb.db.DB.Select(weeklyStatsQuery).OrderBy("TimestampUnix").Find(&allWeeklyStats); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	allWeeklyDeltas := CalculateDeltas(allWeeklyStats)
	// generate graphs
	if len(allWeeklyDeltas) != 0 {
		if err := generateDeltaGraphs(tracker, overallPrefix+"_per_week", allWeeklyDeltas, allWeeklyDeltas[0].Timestamp); err != nil {
			logthis.Error(err, logthis.NORMAL)
			atLeastOneFailed = true
		}
	}

	// 6. stats/month
	// get all monthly stats
	var allMonthlyStats []StatsEntry
	monthlyStatsQuery := q.And(q.Eq("StartOfMonth", true), q.Eq("Tracker", tracker))
	if err := sdb.db.DB.Select(monthlyStatsQuery).OrderBy("TimestampUnix").Find(&allMonthlyStats); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	allMonthlyDeltas := CalculateDeltas(allMonthlyStats)
	// generate graphs
	if len(allMonthlyDeltas) != 0 {
		if err := generateDeltaGraphs(tracker, overallPrefix+"_per_month", allMonthlyDeltas, allMonthlyDeltas[0].Timestamp); err != nil {
			logthis.Error(err, logthis.NORMAL)
			atLeastOneFailed = true
		}
	}

	// 7. release stats: top tags
	var allSnatches []Release
	if err := sdb.db.DB.Find("Tracker", tracker, &allSnatches); err != nil {
		if err == storm.ErrNotFound {
			logthis.Info("could not find snatched releases in database", logthis.VERBOSE)
		} else {
			return errors.Wrap(err, "Error reading back snatch entries from db for tracker "+tracker)
		}
	}
	if len(allSnatches) != 0 {
		popularTags := map[string]int{}
		for _, r := range allSnatches {
			for _, t := range r.Tags {
				popularTags[t]++
			}
		}
		var top10tags []chart.Value
		for k, v := range popularTags {
			top10tags = append(top10tags, chart.Value{Label: k, Value: float64(v)})
		}
		sort.Slice(top10tags, func(i, j int) bool { return top10tags[i].Value > top10tags[j].Value })
		if len(top10tags) > 10 {
			top10tags = top10tags[:10]
		}
		if err := writePieChart(top10tags, "Top tags", filepath.Join(StatsDir, tracker+"_"+toptagsFile)); err != nil {
			logthis.Error(err, logthis.NORMAL)
			atLeastOneFailed = true
		}

		// 8. release stats: top filters
		// generate filters chart
		filterHits := map[string]float64{}
		for _, r := range allSnatches {
			filterHits[r.Filter]++
		}
		var pieSlices []chart.Value
		for k, v := range filterHits {
			pieSlices = append(pieSlices, chart.Value{Value: v, Label: fmt.Sprintf("%s (%d)", k, int(v))})
		}
		if err := writePieChart(pieSlices, "Total snatches by filter", filepath.Join(StatsDir, tracker+"_"+totalSnatchesByFilterFile)); err != nil {
			logthis.Error(err, logthis.NORMAL)
			atLeastOneFailed = true
		}
	}

	// 9. Snatch history stats
	var allSnatchStats []SnatchStatsEntry
	if err := sdb.db.DB.Find("Tracker", tracker, &allSnatchStats); err != nil {
		return errors.Wrap(err, "Error reading back history stats entries from db")
	}

	snatchStatsSeries := SnatchStatsSeries{}
	snatchStatsSeries.AddStats(allSnatchStats...)
	// generate graphs
	if err := snatchStatsSeries.GenerateGraphs(StatsDir, tracker+"_", firstStats.Timestamp, true); err != nil {
		logthis.Error(err, logthis.NORMAL)
		atLeastOneFailed = true
	}

	// combine graphs into overallStatsFile
	if err := combineAllPNGs(filepath.Join(StatsDir, tracker+"_"+overallStatsFile),
		filepath.Join(StatsDir, tracker+"_overall_"+uploadStatsFile),
		filepath.Join(StatsDir, tracker+"_overall_per_day_"+uploadStatsFile),
		filepath.Join(StatsDir, tracker+"_overall_"+downloadStatsFile),
		filepath.Join(StatsDir, tracker+"_overall_per_day_"+downloadStatsFile),
		filepath.Join(StatsDir, tracker+"_overall_"+bufferStatsFile),
		filepath.Join(StatsDir, tracker+"_overall_per_day_"+bufferStatsFile),
		filepath.Join(StatsDir, tracker+"_overall_"+ratioStatsFile),
		filepath.Join(StatsDir, tracker+"_overall_per_day_"+ratioStatsFile),
		filepath.Join(StatsDir, tracker+"_"+totalSnatchesByFilterFile),
		filepath.Join(StatsDir, tracker+"_"+toptagsFile)); err != nil {
		return err
	}
	// return
	if atLeastOneFailed {
		return errors.New(errorGeneratingGraph)
	}
	return nil
}

// generateGraphs for data points
// graphType == lastweek, lastmonth, overall, etc
func generateGraphs(tracker, graphType string, entries []StatsEntry, firstTimestamp time.Time) error {
	logthis.Info("Generating "+graphType+" graphs for tracker "+tracker, logthis.VERBOSEST)
	overallStats := StatsSeries{Tracker: tracker}
	if err := overallStats.AddStats(entries...); err != nil {
		return err
	}
	return overallStats.GenerateGraphs(StatsDir, tracker+"_"+graphType+"_", firstTimestamp, false)
}

// generateDeltaGraphs for data deltas
// graphType == daily, weekly, monthly, etc
func generateDeltaGraphs(tracker, graphType string, entries []StatsDelta, firstTimestamp time.Time) error {
	logthis.Info("Generating "+graphType+" delta graphs for tracker "+tracker, logthis.VERBOSEST)
	overallStats := StatsSeries{Tracker: tracker}
	if err := overallStats.AddDeltas(entries...); err != nil {
		return err
	}
	return overallStats.GenerateGraphs(StatsDir, tracker+"_"+graphType+"_", firstTimestamp, true)
}

func (sdb *StatsDB) AddSnatch(release Release) error {
	// save new entry
	return sdb.db.DB.Save(&release)
}

func (sdb *StatsDB) AlreadySnatchedDuplicate(release *Release) bool {
	duplicateQuery := q.And(
		q.Eq("Tracker", release.Tracker),
		q.Eq("Title", release.Title),
		q.Eq("Year", release.Year),
		q.Eq("ReleaseType", release.ReleaseType),
		q.Eq("Quality", release.Quality),
		q.Eq("Source", release.Source),
		q.Eq("Format", release.Format),
		q.Eq("IsScene", release.IsScene),
		InSlice("Artists", release.Artists[0]),
	)

	var firstHit Release
	err := sdb.db.DB.Select(duplicateQuery).First(&firstHit)
	if err != nil {
		if err != storm.ErrNotFound {
			logthis.Error(errors.Wrap(err, "error looking for duplicate releases"), logthis.NORMAL)
		}
		return false
	}
	return true
}

func (sdb *StatsDB) AlreadySnatchedFromGroup(release *Release) bool {
	// try to find a release from same tracker+groupid
	var releaseFromGroup Release
	err := sdb.db.DB.Select(q.And(q.Eq("GroupID", release.GroupID), q.Eq("Tracker", release.Tracker))).First(&releaseFromGroup)
	if err != nil {
		if err != storm.ErrNotFound {
			logthis.Error(errors.Wrap(err, "error looking for releases from same torrent group"), logthis.NORMAL)
		}
		return false
	}
	return true
}
