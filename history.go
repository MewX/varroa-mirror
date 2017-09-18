package varroa

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/vmihailenco/msgpack.v2"
)

// History manages stats and generates graphs.
type History struct {
	Tracker string
	SnatchHistory
}

func (h *History) getPath(file string) string {
	return filepath.Join(StatsDir, h.Tracker+"_"+file)
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

func (h *History) LoadAll(statsConfig *ConfigStats) error {
	// make sure we're using the latest format, convert if necessary
	h.migrateOldFormats(h.getPath(statsFile), h.getPath(historyFile))
	if err := h.SnatchHistory.Load(h.getPath(historyFile) + msgpackExt); err != nil {
		return err
	}
	return nil
}
