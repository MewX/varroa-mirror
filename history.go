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
	htlmIndex = `
<html>
  <head>
    <title>varroa musica</title>
    <meta content="">
    <style></style>
  </head>
  <body>
    <h1 style="text-align:center;">Varroa Musica</h1>
    <p style="text-align:center;">Last updated: %s | <a href="%s">csv</a></p>
    <p style="text-align:center;">Latest stats: %s</p>
    <p style="text-align:center;"><a href="#buffer">Buffer</a> | <a href="#up">Upload</a> | <a href="#down">Download</a> | <a href="#ratio">Ratio</a> | <a href="#buffer_per_day">Buffer/day</a> | <a href="#up_per_day">Upload/day</a> | <a href="#down_per_day">Download/day</a> | <a href="#ratio_per_day">Ratio/day</a> | <a href="#snatches_per_day">Snatches/day</a> | <a href="#size_snatched_per_day">Size Snatched/day</a></p>
    <p id="buffer" style="text-align:center;"><img src="buffer.svg" alt="stats" style="align:center"></p>
    <p id="up" style="text-align:center;"><img src="up.svg" alt="stats" style="align:center"></p>
    <p id="down" style="text-align:center;"><img src="down.svg" alt="stats" style="align:center"></p>
    <p id="ratio" style="text-align:center;"><img src="ratio.svg" alt="stats" style="align:center"></p>
    <p id="buffer_per_day" style="text-align:center;"><img src="buffer_per_day.svg" alt="stats" style="align:center"></p>
    <p id="up_per_day" style="text-align:center;"><img src="up_per_day.svg" alt="stats" style="align:center"></p>
    <p id="down_per_day" style="text-align:center;"><img src="down_per_day.svg" alt="stats" style="align:center"></p>
    <p id="ratio_per_day" style="text-align:center;"><img src="ratio_per_day.svg" alt="stats" style="align:center"></p>
    <p id="snatches_per_day" style="text-align:center;"><img src="snatches_per_day.svg" alt="stats" style="align:center"></p>
    <p id="size_snatched_per_day" style="text-align:center;"><img src="size_snatched_per_day.svg" alt="stats" style="align:center"></p>
  </body>
</html>`
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
	if statsOK {
		if dailyStatsOK {
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
		// create/update index.html
		if err := ioutil.WriteFile(filepath.Join(statsDir, htmlIndexFile), []byte(fmt.Sprintf(htlmIndex, time.Now().Format("2006-01-02 15:04:05"), filepath.Base(h.getPath(statsFile))+csvExt, h.TrackerStats[len(h.TrackerStats)-1].String())), 0666); err != nil {
			return err
		}
		// deploy automatically, if at least the StatsGraphs have been generated
		return h.Deploy(e)
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

// Deploy to gitlab pages with git wrapper
func (h *History) Deploy(e *Environment) error {
	if !e.config.gitlabPagesConfigured {
		return nil
	}
	if len(h.TrackerStats) == 0 {
		return nil
	}
	git := NewGit(statsDir, e.Trackers[h.Tracker].User, e.Trackers[h.Tracker].User+"+varroa@redacted")
	if git == nil {
		return errors.New("Error setting up git")
	}
	// make sure we're going back to cwd
	defer git.getBack()

	// init repository if necessary
	if !git.Exists() {
		if err := git.Init(); err != nil {
			return errors.Wrap(err, errorGitInit)
		}
		// create .gitlab-ci.yml
		if err := ioutil.WriteFile(filepath.Join(statsDir, gitlabCIYamlFile), []byte(gitlabCI), 0666); err != nil {
			return err
		}
	}
	// add overall stats and other files
	if err := git.Add("*"+svgExt, filepath.Base(h.getPath(statsFile)+csvExt), filepath.Base(gitlabCIYamlFile), filepath.Base(htmlIndexFile)); err != nil {
		return errors.Wrap(err, errorGitAdd)
	}
	// commit
	if err := git.Commit("varroa musica stats update."); err != nil {
		return errors.Wrap(err, errorGitCommit)
	}
	// push
	if !git.HasRemote("origin") {
		if err := git.AddRemote("origin", e.config.GitlabPages.GitHTTPS); err != nil {
			return errors.Wrap(err, errorGitAddRemote)
		}
	}
	if err := git.Push("origin", e.config.GitlabPages.GitHTTPS, e.config.GitlabPages.User, e.config.GitlabPages.Password); err != nil {
		return errors.Wrap(err, errorGitPush)
	}
	logThis.Info("Pushed new stats to "+e.config.GitlabPages.URL, NORMAL)
	return nil
}
