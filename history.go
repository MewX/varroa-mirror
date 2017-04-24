package main

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/vmihailenco/msgpack.v2"
)

const (
	gitlabCI = `# plain-htlm CI
pages:
  stage: deploy
  script:
  - mkdir .public
  - cp -r * .public
  - mv .public public
  artifacts:
    paths:
    - public
  only:
  - master
`
)

// History manages stats and generates graphs.
type History struct {
	Tracker string
	SnatchHistory
	TrackerStatsHistory
}

func (h *History) getPath(file string) string {
	return filepath.Join(statsDir, h.Tracker+"_"+file)
}

func (h *History) migrateOldFormats(statsFile, snatchesFile string) {
	// if upgrading from v5, trying to move the csv files to the stats folder, their new home
	if FileExists(filepath.Base(statsFile+csvExt)) && !FileExists(statsFile+csvExt) {
		logThis.Info("Migrating tracker stats file to the stats folder.", NORMAL)
		if err := os.Rename(filepath.Base(statsFile+csvExt), statsFile+csvExt); err != nil {
			logThis.Error(errors.Wrap(err, errorMovingFile), NORMAL)
		}
	}

	if FileExists(filepath.Base(snatchesFile+csvExt)) && !FileExists(snatchesFile+csvExt) {
		logThis.Info("Migrating sntach history file to the stats folder.", NORMAL)
		if err := os.Rename(filepath.Base(snatchesFile+csvExt), snatchesFile+csvExt); err != nil {
			logThis.Error(errors.Wrap(err, errorMovingFile), NORMAL)
		}
	}

	// if upgrading from v8, converting history.csv to history.db (msgpack)
	if !FileExists(snatchesFile+msgpackExt) && FileExists(snatchesFile+csvExt) {
		logThis.Info("Migrating sntach history file to the latest format (csv -> msgpack).", NORMAL)
		// load history file
		f, errOpening := os.OpenFile(snatchesFile+csvExt, os.O_RDONLY, 0644)
		if errOpening != nil {
			logThis.Info(errorMigratingFile+snatchesFile+csvExt, NORMAL)
			return
		}

		w := csv.NewReader(f)
		records, errReading := w.ReadAll()
		if errReading != nil {
			logThis.Error(errors.Wrap(errReading, "Error loading old history file"), NORMAL)
			return
		}
		if err := f.Close(); err != nil {
			logThis.Error(errors.Wrap(err, "Error closing old history file"), NORMAL)
		}

		releases := []Release{}
		// load releases from history to in-memory slice
		for i, record := range records {
			r := &Release{}
			if err := r.FromSlice(record); err != nil {
				logThis.Error(errors.Wrap(err, fmt.Sprintf(errorLoadingLine, i)), NORMAL)
			} else {
				releases = append(releases, *r)
			}
		}

		// save to new file
		b, err := msgpack.Marshal(releases)
		if err != nil {
			logThis.Error(errors.Wrap(err, errorMigratingFile+snatchesFile+msgpackExt), NORMAL)
			return
		}
		if err := ioutil.WriteFile(snatchesFile+msgpackExt, b, 0640); err != nil {
			logThis.Error(errors.Wrap(err, errorMigratingFile+snatchesFile+msgpackExt), NORMAL)
			return
		}
		// renaming old file
		if err := os.Rename(snatchesFile+csvExt, snatchesFile+".csv.migrated"); err != nil {
			logThis.Info("Error renaming old history.csv file, please remove or move it elsewhere.", NORMAL)
		} else {
			logThis.Info("Old history file renamed to "+snatchesFile+".csv.migrated", NORMAL)
		}
	}
}

func (h *History) LoadAll() error {
	// make sure we're using the latest format, convert if necessary
	h.migrateOldFormats(h.getPath(statsFile), h.getPath(historyFile))

	if err := h.TrackerStatsHistory.Load(h.getPath(statsFile) + csvExt); err != nil {
		return err
	}
	if err := h.SnatchHistory.Load(h.getPath(historyFile) + msgpackExt); err != nil {
		return err
	}
	return nil
}

func (h *History) GenerateGraphs(e *Environment) error {
	// get first overall timestamp in all history sources
	firstOverallTimestamp := h.getFirstTimestamp()
	if firstOverallTimestamp.After(time.Now()) {
		return errors.New(errorInvalidTimestamp)
	}
	statsConfig, err := e.config.GetStats(h.Tracker)
	if err != nil {
		return errors.Wrap(err, "Error getting stats for "+h.Tracker)
	}
	statsOK := true
	dailyStatsOK := true
	// generate stats graphs
	if err := h.GenerateStatsGraphs(firstOverallTimestamp,
		statsConfig.UpdatePeriodH,
		h.getPath(uploadStatsFile),
		h.getPath(downloadStatsFile),
		h.getPath(bufferStatsFile),
		h.getPath(ratioStatsFile),
		h.getPath(uploadPerDayStatsFile),
		h.getPath(downloadPerDayStatsFile),
		h.getPath(bufferPerDayStatsFile),
		h.getPath(ratioPerDayStatsFile)); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
		statsOK = false
	}
	// generate history graphs if necessary
	if err := h.GenerateDailyGraphs(firstOverallTimestamp,
		h.getPath(sizeSnatchedPerDayFile),
		h.getPath(numberSnatchedPerDayFile),
		h.getPath(totalSnatchesByFilterFile),
		h.getPath(toptagsFile)); err != nil {
		if err.Error() == errorNoFurtherSnatches {
			logThis.Info(errorNoFurtherSnatches, VERBOSE)
		} else {
			logThis.Error(errors.Wrap(err, errorGeneratingDailyGraphs), NORMAL)
			dailyStatsOK = false
		}
	}
	if statsOK && dailyStatsOK {
		// combine graphs into overallStatsFile
		if err := combineAllPNGs(h.getPath(overallStatsFile),
			h.getPath(uploadStatsFile),
			h.getPath(uploadPerDayStatsFile),
			h.getPath(downloadStatsFile),
			h.getPath(downloadPerDayStatsFile),
			h.getPath(bufferStatsFile),
			h.getPath(bufferPerDayStatsFile),
			h.getPath(ratioStatsFile),
			h.getPath(ratioPerDayStatsFile),
			h.getPath(numberSnatchedPerDayFile),
			h.getPath(sizeSnatchedPerDayFile),
			h.getPath(totalSnatchesByFilterFile),
			h.getPath(toptagsFile)); err != nil {
			logThis.Error(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
		}
	}

	return errors.New(errorCreatingGraphs)
}

func (h *History) getFirstTimestamp() time.Time {
	// assuming timestamps are in chronological order.
	snatchTimestamp, statsTimestamp := int64(math.MaxInt32), int64(math.MaxInt32)

	if len(h.SnatchedReleases) != 0 {
		snatchTimestamp = h.SnatchedReleases[0].Timestamp.Unix()
	}
	if len(h.TrackerStatsRecords) != 0 && len(h.TrackerStatsRecords[0]) > 0 {
		if timestamp, err := strconv.ParseInt(h.TrackerStatsRecords[0][0], 0, 64); err == nil {
			statsTimestamp = timestamp
		}
	}
	if snatchTimestamp < statsTimestamp {
		return time.Unix(snatchTimestamp, 0)
	}
	return time.Unix(statsTimestamp, 0)
}
