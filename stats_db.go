package varroa

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"time"

	"path/filepath"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/jinzhu/now"
	"github.com/pkg/errors"
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
		if err := statsDB.init(); err != nil {
			returnErr = err
			return
		}

		config, err := NewConfig(DefaultConfigurationFile)
		if err != nil {
			logThis.Error(err, NORMAL)
			returnErr = err
			return
		} else {
			// try to import <v19
			for _, label := range config.TrackerLabels() {
				if err := statsDB.migrate(label, filepath.Join(StatsDir, label+"_"+statsFile+csvExt)); err != nil {
					logThis.Error(errors.Wrap(err, "Error migrating stats csv to the new database, for tracker "+label), NORMAL)
				}
			}
		}
		returnErr = statsDB.Update()
	})
	return statsDB, returnErr
}

func (sdb *StatsDB) init() error {
	return sdb.db.DB.Init(&StatsEntry{})
}

func (sdb *StatsDB) migrate(tracker, csvFile string) error {
	if !FileExists(csvFile) {
		return nil // already migrated
	}
	logThis.Info("Migrating stats for tracker "+tracker, NORMAL)

	// load history file
	f, errOpening := os.OpenFile(csvFile, os.O_RDONLY, 0644)
	if errOpening != nil {
		return errors.New(errorMigratingFile + csvFile)
	}

	w := csv.NewReader(f)
	records, errReading := w.ReadAll()
	if errReading != nil {
		return errors.Wrap(errReading, "Error loading old history file")
	}
	if err := f.Close(); err != nil {
		return errors.Wrap(err, "Error closing old history file")
	}

	// transaction for quicker results
	tx, err := sdb.db.DB.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, record := range records {
		r := &StatsEntry{Tracker: tracker, Collected: true}
		if err := r.FromSlice(record); err != nil {
			logThis.Error(errors.Wrap(err, fmt.Sprintf(errorLoadingLine, i)), NORMAL)
		} else {
			if err := tx.Save(r); err != nil {
				return errors.Wrap(err, "Error saving CSV entry to the new database")
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// checks
	var allEntries []StatsEntry
	if err := sdb.db.DB.Find("Tracker", tracker, &allEntries); err != nil {
		return errors.Wrap(err, "Error reading back stats values from db")
	}
	if len(allEntries) != len(records) {
		return fmt.Errorf("error reading back stats, got %d instead of %d entries", len(allEntries), len(records))
	}
	// ok
	logThis.Info(fmt.Sprintf("Migrated %d records for tracker %s", len(allEntries), tracker), NORMAL)
	// once successful, rename csvFile to csv.v18
	return os.Rename(csvFile, csvFile+".v18")
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
					if err := sdb.db.DB.Select(q.And(q.Eq("Tracker", t), q.Eq("Collected", true), q.Lte("Timestamp", currentStartOfDay))).OrderBy("Timestamp").Reverse().First(&previous); err != nil {
						if err == storm.ErrNotFound {
							// first day, use the first known stats as the reference
							previous = firstTrackerStats
						} else {
							logThis.Error(err, VERBOSEST)
							continue
						}
					}
					// get following collected stats
					var next StatsEntry
					if err := sdb.db.DB.Select(q.And(q.Eq("Tracker", t), q.Eq("Collected", true), q.Gte("Timestamp", currentStartOfDay))).OrderBy("Timestamp").First(&next); err != nil {
						if err == storm.ErrNotFound {
							// last day, missing information to create the daily stats
							continue
						} else {
							logThis.Error(err, VERBOSEST)
							continue
						}
					}

					// calculate the stats at start of day
					newDailyStats := &StatsEntry{}
					var err error
					if previous.Timestamp.Equal(next.Timestamp) {
						// if they are the same, it's probably the first day.
						// creating first day without interpolation.
						newDailyStats = &StatsEntry{Tracker: previous.Tracker, Timestamp: currentStartOfDay, Up: previous.Up, Down: previous.Down, Ratio: previous.Ratio, StartOfDay: true}
					} else {
						// interpolate stats at the start of the day being considered
						newDailyStats, err = InterpolateStats(previous, next, currentStartOfDay)
						if err != nil {
							logThis.Error(err, VERBOSE)
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
					if err := tx.Save(newDailyStats); err != nil {
						return errors.Wrap(err, "error saving daily stats")
					}
					logThis.Info("Added daily stats for "+t+"/"+currentStartOfDay.String(), VERBOSEST)
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
	if err := generateGraphs(tracker, "overall", allStatsEntries, firstStats.Timestamp); err != nil {
		return err
	}

	// 2. collect stats since last week
	// get the timestamp for one week earlier
	firstWeekTimestamp := time.Now().Add(-7 * 24 * time.Hour)
	lastWeekQuery := q.And(q.Eq("Collected", true), q.Eq("Tracker", tracker), q.Gte("Timestamp", firstWeekTimestamp))
	var lastWeekStatsEntries []StatsEntry
	if err := sdb.db.DB.Select(lastWeekQuery).OrderBy("Timestamp").Find(&lastWeekStatsEntries); err != nil {
		return errors.Wrap(err, "error querying database")
	}
	if err := generateGraphs(tracker, "lastweek", lastWeekStatsEntries, lastWeekStatsEntries[0].Timestamp); err != nil {
		logThis.Error(err, NORMAL)
		atLeastOneFailed = true
	}

	// 3. collect stats since last month
	// get the timestamp for one month earlier
	firstMonthTimestamp := time.Now().Add(-30 * 24 * time.Hour)
	lastMonthQuery := q.And(q.Eq("Collected", true), q.Eq("Tracker", tracker), q.Gte("Timestamp", firstMonthTimestamp))
	var lastMonthStatsEntries []StatsEntry
	if err := sdb.db.DB.Select(lastMonthQuery).OrderBy("Timestamp").Find(&lastMonthStatsEntries); err != nil {
		return errors.Wrap(err, "error querying database")
	}
	if err := generateGraphs(tracker, "lastmonth", lastMonthStatsEntries, lastMonthStatsEntries[0].Timestamp); err != nil {
		logThis.Error(err, NORMAL)
		atLeastOneFailed = true
	}

	// 4. stats/day
	// get all daily stats
	var allDailyStats []StatsEntry
	dailyStatsQuery := q.And(q.Eq("StartOfDay", true), q.Eq("Tracker", tracker))
	if err := sdb.db.DB.Select(dailyStatsQuery).OrderBy("Timestamp").Find(&allDailyStats); err != nil {
		return errors.Wrap(err, "error querying database")
	}
	allDailyDeltas := CalculateDeltas(allDailyStats)
	// generate graphs
	if err := generateDeltaGraphs(tracker, "overall_per_day", allDailyDeltas, allDailyDeltas[0].Timestamp); err != nil {
		logThis.Error(err, NORMAL)
		atLeastOneFailed = true
	}

	// 5. stats/week
	// get all weekly stats
	var allWeeklyStats []StatsEntry
	weeklyStatsQuery := q.And(q.Eq("StartOfWeek", true), q.Eq("Tracker", tracker))
	if err := sdb.db.DB.Select(weeklyStatsQuery).OrderBy("Timestamp").Find(&allWeeklyStats); err != nil {
		return errors.Wrap(err, "error querying database")
	}
	allWeeklyDeltas := CalculateDeltas(allWeeklyStats)
	// generate graphs
	if err := generateDeltaGraphs(tracker, "overall_per_week", allWeeklyDeltas, allWeeklyDeltas[0].Timestamp); err != nil {
		logThis.Error(err, NORMAL)
		atLeastOneFailed = true
	}

	// 6. stats/month
	// get all monthly stats
	var allMonthlyStats []StatsEntry
	monthlyStatsQuery := q.And(q.Eq("StartOfMonth", true), q.Eq("Tracker", tracker))
	if err := sdb.db.DB.Select(monthlyStatsQuery).OrderBy("Timestamp").Find(&allMonthlyStats); err != nil {
		return errors.Wrap(err, "error querying database")
	}
	allMonthlyDeltas := CalculateDeltas(allMonthlyStats)
	// generate graphs
	if err := generateDeltaGraphs(tracker, "overall_per_month", allMonthlyDeltas, allMonthlyDeltas[0].Timestamp); err != nil {
		logThis.Error(err, NORMAL)
		atLeastOneFailed = true
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
