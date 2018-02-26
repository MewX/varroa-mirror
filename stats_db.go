package varroa

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/jinzhu/now"
	"github.com/pkg/errors"
	"github.com/wcharczuk/go-chart"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
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

		config, err := NewConfig(DefaultConfigurationFile)
		if err != nil {
			logThis.Error(err, NORMAL)
			returnErr = err
			return
		} else {
			// try to import <v19
			migratedSomething := false
			for _, label := range config.TrackerLabels() {
				migrated, err := statsDB.migrate(label, filepath.Join(StatsDir, label+"_"+statsFile+csvExt), filepath.Join(StatsDir, label+"_"+historyFile+msgpackExt))
				if err != nil {
					logThis.Error(errors.Wrap(err, "Error migrating stats csv to the new database, for tracker "+label), NORMAL)
				} else if migrated {
					migratedSomething = migrated
				}
			}
			if migratedSomething {
				logThis.Info("Updating stats after migration", NORMAL)
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

func (sdb *StatsDB) migrate(tracker, csvFile, msgpackFile string) (bool, error) {
	migratedSomething := false
	if FileExists(csvFile) {
		logThis.Info("Migrating stats for tracker "+tracker, NORMAL)

		// load history file
		f, errOpening := os.OpenFile(csvFile, os.O_RDONLY, 0644)
		if errOpening != nil {
			return migratedSomething, errors.New(errorMigratingFile + csvFile)
		}

		w := csv.NewReader(f)
		records, errReading := w.ReadAll()
		if errReading != nil {
			return migratedSomething, errors.Wrap(errReading, "Error loading old history file")
		}
		if err := f.Close(); err != nil {
			return migratedSomething, errors.Wrap(err, "Error closing old history file")
		}

		// transaction for quicker results
		tx, err := sdb.db.DB.Begin(true)
		if err != nil {
			return migratedSomething, err
		}
		defer tx.Rollback()

		for i, record := range records {
			r := &StatsEntry{Tracker: tracker, Collected: true}
			if err := r.FromSlice(record); err != nil {
				logThis.Error(errors.Wrap(err, fmt.Sprintf(errorLoadingLine, i)), NORMAL)
			} else {
				if err := tx.Save(r); err != nil {
					return migratedSomething, errors.Wrap(err, "Error saving CSV entry to the new database")
				}
			}
		}
		if err := tx.Commit(); err != nil {
			return migratedSomething, err
		}

		// checks
		var allEntries []StatsEntry
		if err := sdb.db.DB.Find("Tracker", tracker, &allEntries); err != nil {
			return migratedSomething, errors.Wrap(err, "Error reading back stats values from db")
		}
		if len(allEntries) != len(records) {
			return migratedSomething, fmt.Errorf("error reading back stats, got %d instead of %d entries", len(allEntries), len(records))
		}
		// ok
		migratedSomething = true
		logThis.Info(fmt.Sprintf("Migrated %d records for tracker %s", len(allEntries), tracker), NORMAL)
		// once successful, rename csvFile to csv.v18
		if err := os.Rename(csvFile, csvFile+".v18"); err != nil {
			return migratedSomething, err
		}
	}

	if FileExists(msgpackFile) {
		logThis.Info("Migrating snatch history for tracker "+tracker, NORMAL)

		// load history file
		bytes, err := ioutil.ReadFile(msgpackFile)
		if err != nil {
			logThis.Error(errors.Wrap(err, "Error reading old history file"), NORMAL)
			return migratedSomething, err
		}
		if len(bytes) == 0 {
			// newly created file
			return migratedSomething, nil
		}

		var oldReleases []Release
		// load releases from history to in-memory slice
		err = msgpack.Unmarshal(bytes, &oldReleases)
		if err != nil {
			logThis.Error(errors.Wrap(err, "Error loading releases from old history file"), NORMAL)
		}

		// transaction for quicker results
		tx, err := sdb.db.DB.Begin(true)
		if err != nil {
			return migratedSomething, err
		}
		defer tx.Rollback()

		for _, release := range oldReleases {
			release.Tracker = tracker
			if err := tx.Save(&release); err != nil {
				return migratedSomething, errors.Wrap(err, "Error saving snatch entry to the new database")
			}
		}
		if err := tx.Commit(); err != nil {
			return migratedSomething, err
		}

		// checks
		var allSnatches []Release
		if err := sdb.db.DB.Find("Tracker", tracker, &allSnatches); err != nil {
			return migratedSomething, errors.Wrap(err, "Error reading back snatch entries from db")
		}
		if len(allSnatches) != len(oldReleases) {
			return migratedSomething, fmt.Errorf("error reading back snatches, got %d instead of %d entries", len(allSnatches), len(oldReleases))
		}
		// ok
		migratedSomething = true
		logThis.Info(fmt.Sprintf("Migrated %d records for tracker %s", len(allSnatches), tracker), NORMAL)
		// once successful, rename csvFile to csv.v18
		if err := os.Rename(msgpackFile, msgpackFile+".v18"); err != nil {
			return migratedSomething, err
		}
	}
	return migratedSomething, nil
}

// Update needs to be called everyday at midnight (add cron job)
// It creates StatsEntries for each start of day (which might also be start of week/month)
// That way, it'll be quicker to create stats and recalculate StatsDeltas.
func (sdb *StatsDB) Update() error {
	defer TimeTrack(time.Now(), "UPDATE")
	// TODO: add cron job

	// get tracker labels from config.
	config, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		return err
	}
	allTrackers := config.TrackerLabels()

	// transaction for quicker results
	tx, err := sdb.db.DB.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// get start of today
	startOfToday := now.BeginningOfDay()
	for _, t := range allTrackers {
		logThis.Info("Updating stats for tracker "+t, VERBOSEST)

		// find first collected timestamp for this tracker
		firstTrackerStats, err := sdb.getFirstStatsForTracker(t)
		if err != nil {
			logThis.Info("Could not find stats for tracker "+t, VERBOSEST)
			continue
		}
		startOfFirstDay := now.New(firstTrackerStats.Timestamp).BeginningOfDay()
		if startOfFirstDay.After(startOfToday) {
			logThis.Info("Incoherent daily stats: some stats in the future for tracker "+t, VERBOSEST)
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
					if selectErr := sdb.db.DB.Select(q.And(q.Eq("Tracker", t), q.Eq("Collected", true), q.Lte("Timestamp", currentStartOfDay))).OrderBy("Timestamp").Reverse().First(&previous); selectErr != nil {
						if selectErr == storm.ErrNotFound {
							// first day, use the first known stats as the reference
							previous = firstTrackerStats
						} else {
							logThis.Error(selectErr, VERBOSEST)
							continue
						}
					}
					// get following collected stats
					var next StatsEntry
					if selectErr := sdb.db.DB.Select(q.And(q.Eq("Tracker", t), q.Eq("Collected", true), q.Gte("Timestamp", currentStartOfDay))).OrderBy("Timestamp").First(&next); selectErr != nil {
						if selectErr == storm.ErrNotFound {
							// last day, missing information to create the daily stats
							continue
						} else {
							logThis.Error(selectErr, VERBOSEST)
							continue
						}
					}

					// calculate the stats at start of day
					newDailyStats := &StatsEntry{}
					var statsErr error
					if previous.Timestamp.Equal(next.Timestamp) {
						// if they are the same, it's probably the first day.
						// creating first day without interpolation.
						newDailyStats = &StatsEntry{Tracker: previous.Tracker, Timestamp: currentStartOfDay, Up: previous.Up, Down: previous.Down, Ratio: previous.Ratio, StartOfDay: true}
					} else {
						// interpolate stats at the start of the day being considered
						newDailyStats, statsErr = InterpolateStats(previous, next, currentStartOfDay)
						if statsErr != nil {
							logThis.Error(statsErr, VERBOSE)
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
					logThis.Info("Added daily stats for "+t+"/"+currentStartOfDay.String(), VERBOSEST)
				} else {
					logThis.Error(err, VERBOSEST)
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
							logThis.Error(selectErr, VERBOSEST)
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

					logThis.Info("Added daily snatch stats for "+t+"/"+currentStartOfDay.String(), VERBOSEST)
				} else {
					logThis.Error(err, VERBOSEST)
				}
			}
		}
	}
	// committing to db
	return tx.Commit()
}

func (sdb *StatsDB) FilterByTracker(tracker string, statsType string) ([]StatsEntry, error) {
	// TODO to avoid making huge lists, use Limit(n).Skip(i*n)?

	if !StringInSlice(statsType, []string{"Collected", "StartOfDay", "StartOfWeek", "StartOfMonth"}) {
		return []StatsEntry{}, errors.New("Unknown stats type: " + statsType)
	}

	var entries []StatsEntry
	err := sdb.db.DB.Select(q.And(q.Eq(statsType, true), q.Eq("Tracker", tracker))).OrderBy("Timestamp").Find(&entries)
	return entries, err
}

func (sdb *StatsDB) Save(entry *StatsEntry) error {
	return sdb.db.DB.Save(entry)
}

func (sdb *StatsDB) getFirstTimestamp() (time.Time, error) {
	var firstEntry StatsEntry
	err := sdb.db.DB.Select(q.Eq("Collected", true)).OrderBy("Timestamp").First(&firstEntry)
	return firstEntry.Timestamp, err
}

func (sdb *StatsDB) getFirstStatsForTracker(tracker string) (StatsEntry, error) {
	var firstEntry StatsEntry
	err := sdb.db.DB.Select(q.And(q.Eq("Collected", true), q.Eq("Tracker", tracker))).OrderBy("Timestamp").First(&firstEntry)
	return firstEntry, err
}

func (sdb *StatsDB) GetLastCollected(tracker string, limit int) ([]StatsEntry, error) {
	var lastEntries []StatsEntry
	err := sdb.db.DB.Select(q.And(q.Eq("Collected", true), q.Eq("Tracker", tracker))).Limit(limit).OrderBy("Timestamp").Reverse().Find(&lastEntries)
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
	if err := sdb.db.DB.Select(lastWeekQuery).OrderBy("Timestamp").Find(&lastWeekStatsEntries); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	if len(lastWeekStatsEntries) != 0 {
		if err := generateGraphs(tracker, lastWeekPrefix, lastWeekStatsEntries, lastWeekStatsEntries[0].Timestamp); err != nil {
			logThis.Error(err, NORMAL)
			atLeastOneFailed = true
		}
	}

	// 3. collect stats since last month
	// get the timestamp for one month earlier
	firstMonthTimestamp := time.Now().Add(-30 * 24 * time.Hour)
	lastMonthQuery := q.And(q.Eq("Collected", true), q.Eq("Tracker", tracker), q.Gte("Timestamp", firstMonthTimestamp))
	var lastMonthStatsEntries []StatsEntry
	if err := sdb.db.DB.Select(lastMonthQuery).OrderBy("Timestamp").Find(&lastMonthStatsEntries); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	if len(lastMonthStatsEntries) != 0 {
		if err := generateGraphs(tracker, lastMonthPrefix, lastMonthStatsEntries, lastMonthStatsEntries[0].Timestamp); err != nil {
			logThis.Error(err, NORMAL)
			atLeastOneFailed = true
		}
	}

	// 4. stats/day
	// get all daily stats
	var allDailyStats []StatsEntry
	dailyStatsQuery := q.And(q.Eq("StartOfDay", true), q.Eq("Tracker", tracker))
	if err := sdb.db.DB.Select(dailyStatsQuery).OrderBy("Timestamp").Find(&allDailyStats); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	allDailyDeltas := CalculateDeltas(allDailyStats)
	// generate graphs
	if len(allDailyDeltas) != 0 {
		if err := generateDeltaGraphs(tracker, overallPrefix+"_per_day", allDailyDeltas, allDailyDeltas[0].Timestamp); err != nil {
			logThis.Error(err, NORMAL)
			atLeastOneFailed = true
		}
	}

	// 5. stats/week
	// get all weekly stats
	var allWeeklyStats []StatsEntry
	weeklyStatsQuery := q.And(q.Eq("StartOfWeek", true), q.Eq("Tracker", tracker))
	if err := sdb.db.DB.Select(weeklyStatsQuery).OrderBy("Timestamp").Find(&allWeeklyStats); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	allWeeklyDeltas := CalculateDeltas(allWeeklyStats)
	// generate graphs
	if len(allWeeklyDeltas) != 0 {
		if err := generateDeltaGraphs(tracker, overallPrefix+"_per_week", allWeeklyDeltas, allWeeklyDeltas[0].Timestamp); err != nil {
			logThis.Error(err, NORMAL)
			atLeastOneFailed = true
		}
	}

	// 6. stats/month
	// get all monthly stats
	var allMonthlyStats []StatsEntry
	monthlyStatsQuery := q.And(q.Eq("StartOfMonth", true), q.Eq("Tracker", tracker))
	if err := sdb.db.DB.Select(monthlyStatsQuery).OrderBy("Timestamp").Find(&allMonthlyStats); err != nil {
		if err != storm.ErrNotFound {
			return errors.Wrap(err, "error querying database")
		}
	}
	allMonthlyDeltas := CalculateDeltas(allMonthlyStats)
	// generate graphs
	if len(allMonthlyDeltas) != 0 {
		if err := generateDeltaGraphs(tracker, overallPrefix+"_per_month", allMonthlyDeltas, allMonthlyDeltas[0].Timestamp); err != nil {
			logThis.Error(err, NORMAL)
			atLeastOneFailed = true
		}
	}

	// 7. release stats: top tags
	var allSnatches []Release
	if err := sdb.db.DB.Find("Tracker", tracker, &allSnatches); err != nil {
		return errors.Wrap(err, "Error reading back snatch entries from db")
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
			logThis.Error(err, NORMAL)
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
			logThis.Error(err, NORMAL)
			atLeastOneFailed = true
		}
	}

	// 9. Snatch history stats
	var allSnatchStats []SnatchStatsEntry
	if err := sdb.db.DB.Find("Tracker", tracker, &allSnatchStats); err != nil {
		return errors.Wrap(err, "Error reading back snatch stats entries from db")
	}

	snatchStatsSeries := SnatchStatsSeries{}
	snatchStatsSeries.AddStats(allSnatchStats...)
	// generate graphs
	if err := snatchStatsSeries.GenerateGraphs(StatsDir, tracker+"_", firstStats.Timestamp, true); err != nil {
		logThis.Error(err, NORMAL)
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
	logThis.Info("Generating "+graphType+" graphs for tracker "+tracker, VERBOSEST)
	overallStats := StatsSeries{Tracker: tracker}
	if err := overallStats.AddStats(entries...); err != nil {
		return err
	}
	return overallStats.GenerateGraphs(StatsDir, tracker+"_"+graphType+"_", firstTimestamp, false)
}

// generateDeltaGraphs for data deltas
// graphType == daily, weekly, monthly, etc
func generateDeltaGraphs(tracker, graphType string, entries []StatsDelta, firstTimestamp time.Time) error {
	logThis.Info("Generating "+graphType+" delta graphs for tracker "+tracker, VERBOSEST)
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
			logThis.Error(errors.Wrap(err, "error looking for duplicate releases"), NORMAL)
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
			logThis.Error(errors.Wrap(err, "error looking for releases from same torrent group"), NORMAL)
		}
		return false
	}
	return true
}
