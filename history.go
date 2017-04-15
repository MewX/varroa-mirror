package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/wcharczuk/go-chart"
	"gopkg.in/vmihailenco/msgpack.v2"
)

const (
	errorLoadingLine       = "Error loading line %d of history file"
	errorNoHistory         = "No history yet"
	errorInvalidTimestamp  = "Error parsing timestamp"
	errorNoFurtherSnatches = "No additional snatches since last time, not regenerating daily graphs."
	errorNotEnoughDays     = "Not enough days in history to generate daily graphs"
	errorGitInit           = "Error running git init: "
	errorGitAdd            = "Error running git add: "
	errorGitCommit         = "Error running git commit: "
	errorGitAddRemote      = "Error running git remote add: "
	errorGitPush           = "Error running git push: "
	errorMovingFile        = "Error moving file to stats folder: "
	errorMigratingFile     = "Error migrating file to latest format: "
	errorCreatingGraphs    = "Could not generate any graph."
	errorGeneratingGraph   = "Error generating graph: "

	statsDir   = "stats"
	pngExt     = ".png"
	svgExt     = ".svg"
	csvExt     = ".csv"
	msgpackExt = ".db"
	jsonExt    = ".json"
	gitlabCI   = `# plain-htlm CI
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

var (
	uploadStatsFile           = filepath.Join(statsDir, "up")
	uploadPerDayStatsFile     = filepath.Join(statsDir, "up_per_day")
	downloadStatsFile         = filepath.Join(statsDir, "down")
	downloadPerDayStatsFile   = filepath.Join(statsDir, "down_per_day")
	ratioStatsFile            = filepath.Join(statsDir, "ratio")
	ratioPerDayStatsFile      = filepath.Join(statsDir, "ratio_per_day")
	bufferStatsFile           = filepath.Join(statsDir, "buffer")
	bufferPerDayStatsFile     = filepath.Join(statsDir, "buffer_per_day")
	overallStatsFile          = filepath.Join(statsDir, "stats")
	numberSnatchedPerDayFile  = filepath.Join(statsDir, "snatches_per_day")
	sizeSnatchedPerDayFile    = filepath.Join(statsDir, "size_snatched_per_day")
	totalSnatchesByFilterFile = filepath.Join(statsDir, "total_snatched_by_filter")
	toptagsFile               = filepath.Join(statsDir, "top_tags")
	gitlabCIYamlFile          = filepath.Join(statsDir, ".gitlab-ci.yml")
	htmlIndexFile             = filepath.Join(statsDir, "index.html")
	historyFile               = filepath.Join(statsDir, "history")
	statsFile                 = filepath.Join(statsDir, "stats")
)

// History manages stats and generates graphs.
type History struct {
	SnatchHistory
	TrackerStatsHistory
}

func (h *History) migrateOldFormats(statsFile, snatchesFile string) {
	// if upgrading from v5, trying to move the csv files to the stats folder, their new home
	if FileExists(filepath.Base(statsFile+csvExt)) && !FileExists(statsFile+csvExt) {
		logThis("Migrating tracker stats file to the stats folder.", NORMAL)
		if err := os.Rename(filepath.Base(statsFile+csvExt), statsFile+csvExt); err != nil {
			logThis(errorMovingFile, NORMAL)
		}
	}

	if FileExists(filepath.Base(snatchesFile+csvExt)) && !FileExists(snatchesFile+csvExt) {
		logThis("Migrating sntach history file to the stats folder.", NORMAL)
		if err := os.Rename(filepath.Base(snatchesFile+csvExt), snatchesFile+csvExt); err != nil {
			logThis(errorMovingFile, NORMAL)
		}
	}

	// if upgrading from v8, converting history.csv to history.db (msgpack)
	if !FileExists(snatchesFile+msgpackExt) && FileExists(snatchesFile+csvExt) {
		logThis("Migrating sntach history file to the latest format (csv -> msgpack).", NORMAL)
		// load history file
		f, errOpening := os.OpenFile(snatchesFile+csvExt, os.O_RDONLY, 0644)
		if errOpening != nil {
			logThis(errorMigratingFile+snatchesFile+csvExt, NORMAL)
			return
		}

		w := csv.NewReader(f)
		records, errReading := w.ReadAll()
		if errReading != nil {
			logThis("Error loading old history file: "+errReading.Error(), NORMAL)
			return
		}
		if err := f.Close(); err != nil {
			logThis("Error closing old history file: "+err.Error(), NORMAL)
		}

		releases := []Release{}
		// load releases from history to in-memory slice
		for i, record := range records {
			r := &Release{}
			if err := r.FromSlice(record); err != nil {
				logThis(fmt.Sprintf(errorLoadingLine, i)+err.Error(), NORMAL)
			} else {
				releases = append(releases, *r)
			}
		}

		// save to new file
		b, err := msgpack.Marshal(releases)
		if err != nil {
			logThis(errorMigratingFile+snatchesFile+msgpackExt+" :"+err.Error(), NORMAL)
			return
		}
		if err := ioutil.WriteFile(snatchesFile+msgpackExt, b, 0640); err != nil {
			logThis(errorMigratingFile+snatchesFile+msgpackExt+" :"+err.Error(), NORMAL)
			return
		}
		// renaming old file
		if err := os.Rename(snatchesFile+csvExt, snatchesFile+".csv.migrated"); err != nil {
			logThis("Error renaming old history.csv file, please remove or move it elsewhere.", NORMAL)
		} else {
			logThis("Old history file renamed to "+snatchesFile+".csv.migrated", NORMAL)
		}
	}
}

func (h *History) LoadAll(statsFile, snatchesFile string) error {
	// make sure we're using the latest format, convert if necessary
	h.migrateOldFormats(statsFile, snatchesFile)

	if err := h.TrackerStatsHistory.Load(statsFile + csvExt); err != nil {
		return err
	}
	if err := h.SnatchHistory.Load(snatchesFile + msgpackExt); err != nil {
		return err
	}
	return nil
}

func (h *History) GenerateGraphs() error {
	// get first overall timestamp in all history sources
	firstOverallTimestamp := h.getFirstTimestamp()
	if firstOverallTimestamp.After(time.Now()) {
		return errors.New(errorInvalidTimestamp)
	}
	statsOK := true
	dailyStatsOK := true
	// generate stats graphs
	if err := h.GenerateStatsGraphs(firstOverallTimestamp); err != nil {
		logThis(errorGeneratingGraphs+err.Error(), NORMAL)
		statsOK = false
	}
	// generate history graphs if necessary
	if err := h.GenerateDailyGraphs(firstOverallTimestamp); err != nil {
		if err.Error() == errorNoFurtherSnatches {
			logThis(errorNoFurtherSnatches, VERBOSE)
		} else {
			logThis(errorGeneratingDailyGraphs+err.Error(), NORMAL)
			dailyStatsOK = false
		}
	}
	if statsOK {
		if dailyStatsOK {
			// combine graphs into overallStatsFile
			if err := combineAllPNGs(overallStatsFile, uploadStatsFile, uploadPerDayStatsFile, downloadStatsFile, downloadPerDayStatsFile, bufferStatsFile, bufferPerDayStatsFile, ratioStatsFile, ratioPerDayStatsFile, numberSnatchedPerDayFile, sizeSnatchedPerDayFile, totalSnatchesByFilterFile, toptagsFile); err != nil {
				logThis(errorGeneratingGraphs+err.Error(), NORMAL)
			}
		}
		// create/update index.html
		if err := ioutil.WriteFile(htmlIndexFile, []byte(fmt.Sprintf(htlmIndex, time.Now().Format("2006-01-02 15:04:05"), filepath.Base(statsFile)+csvExt, h.TrackerStats[len(h.TrackerStats)-1].String())), 0666); err != nil {
			return err
		}
		// deploy automatically, if at least the StatsGraphs have been generated
		return h.Deploy()
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
func (h *History) Deploy() error {
	if !env.config.gitlabPagesConfigured() {
		return nil
	}
	if len(h.TrackerStats) == 0 {
		return nil
	}
	git := NewGit(statsDir, env.config.user, env.config.user+"+varroa@redacted")
	if git == nil {
		return errors.New("Error setting up git")
	}
	// make sure we're going back to cwd
	defer git.getBack()

	// init repository if necessary
	if !git.Exists() {
		if err := git.Init(); err != nil {
			return errors.New(errorGitInit + err.Error())
		}
		// create .gitlab-ci.yml
		if err := ioutil.WriteFile(gitlabCIYamlFile, []byte(gitlabCI), 0666); err != nil {
			return err
		}
	}
	// add overall stats and other files
	if err := git.Add("*"+svgExt, filepath.Base(statsFile+csvExt), filepath.Base(gitlabCIYamlFile), filepath.Base(htmlIndexFile)); err != nil {
		return errors.New(errorGitAdd + err.Error())
	}
	// commit
	if err := git.Commit("varroa musica stats update."); err != nil {
		return errors.New(errorGitCommit + err.Error())
	}
	// push
	if !git.HasRemote("origin") {
		if err := git.AddRemote("origin", env.config.gitlab.pagesGitURL); err != nil {
			return errors.New(errorGitAddRemote + err.Error())
		}
	}
	if err := git.Push("origin", env.config.gitlab.pagesGitURL, env.config.gitlab.user, env.config.gitlab.password); err != nil {
		return errors.New(errorGitPush + err.Error())
	}
	logThis("Pushed new stats to "+env.config.gitlab.pagesURL, NORMAL)
	return nil
}

//----------------------------------------------------------------------------------------------------------------------

type SnatchHistory struct {
	SnatchesPath        string
	SnatchedReleases    []Release
	SnatchesPacked      []byte
	LastGeneratedPerDay int
}

func (s *SnatchHistory) Load(snatchesFile string) error {
	s.SnatchesPath = snatchesFile
	// load history file
	f, err := os.OpenFile(s.SnatchesPath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	bytes, err := ioutil.ReadFile(snatchesFile)
	if err != nil {
		logThis("Error reading history file", NORMAL)
		return err
	}
	s.SnatchesPacked = bytes

	// load releases from history to in-memory slice
	err = msgpack.Unmarshal(bytes, &s.SnatchedReleases)
	if err != nil {
		logThis("Error loading releases from history file", NORMAL)
	}
	// fix empty filters, if any
	for i := range s.SnatchedReleases {
		if s.SnatchedReleases[i].Filter == "" {
			s.SnatchedReleases[i].Filter = "remote"
		}
	}
	return err
}

func (s *SnatchHistory) Add(r *Release, filter string) error {
	// saving association with filter
	r.Filter = filter
	// add to in memory slice
	s.SnatchedReleases = append(s.SnatchedReleases, *r)
	// saving to msgpack
	b, err := msgpack.Marshal(s.SnatchedReleases)
	if err != nil {
		return err
	}
	s.SnatchesPacked = b
	// write to history file
	return ioutil.WriteFile(s.SnatchesPath, b, 0640)
}

func (s *SnatchHistory) HasDupe(r *Release) bool {
	// check if r is already in history
	for _, hr := range s.SnatchedReleases {
		if r.IsDupe(hr) {
			return true
		}
	}
	return false
}

func (s *SnatchHistory) SnatchedPerDay(firstTimestamp time.Time) ([]time.Time, []float64, []float64, error) {
	if len(s.SnatchedReleases) == 0 {
		return nil, nil, nil, errors.New(errorNoHistory)
	}
	// all snatches should already be available in-memory
	// get all times
	allTimes := []time.Time{}
	for _, record := range s.SnatchedReleases {
		allTimes = append(allTimes, record.Timestamp)
	}
	// slice snatches data per day
	dayTimes := allDaysSince(firstTimestamp)
	snatchesPerDay := []float64{}
	sizePerDay := []float64{}
	for _, t := range dayTimes {
		snatchesPerDay = append(snatchesPerDay, 0)
		sizePerDay = append(sizePerDay, 0)
		// find releases snatched that day and add to stats
		for i, recordTime := range allTimes {
			if recordTime.Before(t) {
				// continue until we get to start of day
				continue
			}
			if recordTime.After(nextDay(t)) {
				// after the end of day for this slice, no use going further
				break
			}
			// increment number of snatched and size snatched
			snatchesPerDay[len(snatchesPerDay)-1] += 1
			sizePerDay[len(sizePerDay)-1] += float64(s.SnatchedReleases[i].Size)
		}
	}
	return dayTimes, snatchesPerDay, sizePerDay, nil
}

func (s *SnatchHistory) GenerateDailyGraphs(firstOverallTimestamp time.Time) error {
	if len(s.SnatchedReleases) == s.LastGeneratedPerDay {
		// no additional snatch since the graphs were last generated, nothing needs to be done
		return errors.New(errorNoFurtherSnatches)
	}
	// get slices of relevant data
	timestamps, numberOfSnatchesPerDay, sizeSnatchedPerDay, err := s.SnatchedPerDay(firstOverallTimestamp)
	if err != nil {
		if err.Error() == errorNoHistory {
			logThis(errorNoHistory, NORMAL)
			return nil // nothing to do yet
		}
		return err
	}
	if len(timestamps) < 2 {
		logThis(errorNotEnoughDays, NORMAL)
		return nil // not enough days yet
	}
	if !firstOverallTimestamp.Equal(timestamps[0]) {
		// if the first overall timestamp isn't in the snatch history, artificially add it
		timestamps = append([]time.Time{firstOverallTimestamp, previousDay(timestamps[0])}, timestamps...)
		numberOfSnatchesPerDay = append([]float64{0, 0}, numberOfSnatchesPerDay...)
		sizeSnatchedPerDay = append([]float64{0, 0}, sizeSnatchedPerDay...)
	}

	sizeSnatchedSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: timestamps,
		YValues: sliceByteToGigabyte(sizeSnatchedPerDay),
	}
	numberSnatchedSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: timestamps,
		YValues: numberOfSnatchesPerDay,
	}

	// generate graphs
	if err := writeTimeSeriesChart(sizeSnatchedSeries, "Size snatched/day (Gb)", sizeSnatchedPerDayFile, true); err != nil {
		return err
	}
	if err := writeTimeSeriesChart(numberSnatchedSeries, "Snatches/day", numberSnatchedPerDayFile, true); err != nil {
		return err
	}

	// generate filters chart
	filterHits := map[string]float64{}
	for _, r := range s.SnatchedReleases {
		filterHits[r.Filter]++
	}
	pieSlices := []chart.Value{}
	for k, v := range filterHits {
		pieSlices = append(pieSlices, chart.Value{Value: v, Label: fmt.Sprintf("%s (%d)", k, int(v))})
	}
	if err := writePieChart(pieSlices, "Total snatches by filter", totalSnatchesByFilterFile); err != nil {
		return err
	}

	// generate top 10 tags chart
	popularTags := map[string]int{}
	for _, r := range s.SnatchedReleases {
		for _, t := range r.Tags {
			popularTags[t]++
		}
	}
	top10tags := []chart.Value{}
	for k, v := range popularTags {
		top10tags = append(top10tags, chart.Value{Label: k, Value: float64(v)})
	}
	sort.Slice(top10tags, func(i, j int) bool { return top10tags[i].Value > top10tags[j].Value })
	if len(top10tags) > 10 {
		top10tags = top10tags[:10]
	}
	if err := writePieChart(top10tags, "Top tags", toptagsFile); err != nil {
		return err
	}

	// keep total number of snatches as reference for later
	s.LastGeneratedPerDay = len(s.SnatchedReleases)
	return nil
}

//----------------------------------------------------------------------------------------------------------------------

type TrackerStatsHistory struct {
	TrackerStatsPath    string
	TrackerStatsRecords [][]string
	TrackerStats        []*TrackerStats
}

func (t *TrackerStatsHistory) Load(statsFile string) error {
	t.TrackerStatsPath = statsFile
	// load tracker stats
	f, err := os.OpenFile(t.TrackerStatsPath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewReader(f)
	trackerStats, err := w.ReadAll()
	if err != nil {
		return err
	}
	t.TrackerStatsRecords = trackerStats
	// load stats to in-memory slice
	for i, stats := range trackerStats {
		r := &TrackerStats{}
		if err := r.FromSlice(stats); err != nil {
			logThis(fmt.Sprintf(errorLoadingLine, i), NORMAL)
		} else {
			t.TrackerStats = append(t.TrackerStats, r)
		}
	}
	return nil
}

func (t *TrackerStatsHistory) Add(stats *TrackerStats) error {
	t.TrackerStats = append(t.TrackerStats, stats)
	// prepare csv fields
	timestamp := time.Now().Unix()
	newStats := []string{fmt.Sprintf("%d", timestamp)}
	newStats = append(newStats, stats.ToSlice()...)
	t.TrackerStatsRecords = append(t.TrackerStatsRecords, newStats)
	// append to file
	f, err := os.OpenFile(t.TrackerStatsPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(newStats); err != nil {
		return err
	}
	w.Flush()
	return nil
}

func (t *TrackerStatsHistory) StatsPerDay(firstTimestamp time.Time) ([]time.Time, []float64, []float64, []float64, []float64, error) {
	if len(t.TrackerStatsRecords) == 0 {
		return nil, nil, nil, nil, nil, errors.New(errorNoHistory)
	}
	// all snatches should already be available in-memory
	// get all times
	allTimes := []time.Time{}
	for _, record := range t.TrackerStatsRecords {
		timestamp, err := strconv.ParseInt(record[0], 0, 64)
		if err != nil {
			return nil, nil, nil, nil, nil, errors.New(errorInvalidTimestamp)
		}
		allTimes = append(allTimes, time.Unix(timestamp, 0))
	}
	// slice snatches data per day
	dayTimes := allDaysSince(firstTimestamp)
	statsAtStartOfDay := []*TrackerStats{}
	// no sense getting stats for the last dayTimes == start of tomorrow
	for _, d := range dayTimes[:len(dayTimes)-1] {
		beforeIndex := -1
		afterIndex := -1
		// find the timestamps just before & after start of day
		for i, recordTime := range allTimes {
			if recordTime.Before(d) {
				// continue until we get to start of day
				continue
			}
			if i > 0 && beforeIndex == -1 && (recordTime.Equal(d) || recordTime.After(d)) {
				beforeIndex = i - 1
				afterIndex = i
				break
			}
		}
		// extrapolation using stats before & after the start of day to get virtual stats at that time
		virtualStats := &TrackerStats{}
		upSlope := float64((float64(t.TrackerStats[afterIndex].Up) - float64(t.TrackerStats[beforeIndex].Up)) / float64(allTimes[afterIndex].Unix()-allTimes[beforeIndex].Unix()))
		upOffset := float64(t.TrackerStats[beforeIndex].Up) - upSlope*float64(allTimes[beforeIndex].Unix())
		virtualStats.Up = uint64(upSlope*float64(d.Unix()) + upOffset)
		downSlope := float64((float64(t.TrackerStats[afterIndex].Down) - float64(t.TrackerStats[beforeIndex].Down)) / float64(allTimes[afterIndex].Unix()-allTimes[beforeIndex].Unix()))
		downOffset := float64(t.TrackerStats[beforeIndex].Down) - downSlope*float64(allTimes[beforeIndex].Unix())
		virtualStats.Down = uint64(downSlope*float64(d.Unix()) + downOffset)
		bufferSlope := float64((float64(t.TrackerStats[afterIndex].Buffer) - float64(t.TrackerStats[beforeIndex].Buffer)) / float64(allTimes[afterIndex].Unix()-allTimes[beforeIndex].Unix()))
		bufferOffset := float64(t.TrackerStats[beforeIndex].Buffer) - bufferSlope*float64(allTimes[beforeIndex].Unix())
		virtualStats.Buffer = uint64(bufferSlope*float64(d.Unix()) + bufferOffset)
		ratioSlope := float64((t.TrackerStats[afterIndex].Ratio - t.TrackerStats[beforeIndex].Ratio) / float64(allTimes[afterIndex].Unix()-allTimes[beforeIndex].Unix()))
		ratioOffset := t.TrackerStats[beforeIndex].Ratio - ratioSlope*float64(allTimes[beforeIndex].Unix())
		virtualStats.Ratio = ratioSlope*float64(d.Unix()) + ratioOffset
		// keep the virtual stats in memory
		statsAtStartOfDay = append(statsAtStartOfDay, virtualStats)
	}

	// now calculating differences one day from the other
	upPerDay := []float64{}
	downPerDay := []float64{}
	bufferPerDay := []float64{}
	ratioPerDay := []float64{}
	for i, s := range statsAtStartOfDay {
		if i == 0 {
			continue
		}
		up, down, buffer, _, ratio := s.Diff(statsAtStartOfDay[i-1])
		upPerDay = append(upPerDay, float64(up))
		downPerDay = append(downPerDay, float64(down))
		bufferPerDay = append(bufferPerDay, float64(buffer))
		ratioPerDay = append(ratioPerDay, float64(ratio))
	}
	// adding 0s for today's and tomorrow's stats (which are still unknown)
	upPerDay = append(upPerDay, 0, 0)
	downPerDay = append(downPerDay, 0, 0)
	bufferPerDay = append(bufferPerDay, 0, 0)
	ratioPerDay = append(ratioPerDay, 0, 0)
	return dayTimes, upPerDay, downPerDay, bufferPerDay, ratioPerDay, nil
}

func (t *TrackerStatsHistory) GenerateStatsGraphs(firstOverallTimestamp time.Time) error {
	// generate tracker stats graphs
	if len(t.TrackerStatsRecords) <= 2 {
		// not enough data points yet
		return errors.New("Empty stats history")
	}
	if len(t.TrackerStatsRecords) != len(t.TrackerStats) {
		return errors.New("Incoherent in-memory stats")
	}
	//  generate data slices
	timestamps := []time.Time{}
	ups := []float64{}
	downs := []float64{}
	buffers := []float64{}
	ratios := []float64{}
	for _, stats := range t.TrackerStatsRecords {
		timestamp, err := strconv.ParseInt(stats[0], 10, 64)
		if err != nil {
			return errors.New(errorInvalidTimestamp)
		}
		timestamps = append(timestamps, time.Unix(timestamp, 0))
	}
	if len(timestamps) < 2 {
		return errors.New(errorNotEnoughDataPoints)
	}
	for _, stats := range t.TrackerStats {
		ups = append(ups, float64(stats.Up))
		downs = append(downs, float64(stats.Down))
		buffers = append(buffers, float64(stats.Buffer))
		ratios = append(ratios, float64(stats.Ratio))
	}
	if !firstOverallTimestamp.Equal(timestamps[0]) {
		// if the first overall timestamp isn't in the snatch history, artificially add it
		timestamps = append([]time.Time{firstOverallTimestamp, timestamps[0].Add(time.Duration(-env.config.statsUpdatePeriod) * time.Hour)}, timestamps...)
		ups = append([]float64{0, 0}, ups...)
		downs = append([]float64{0, 0}, downs...)
		buffers = append([]float64{0, 0}, buffers...)
		ratios = append([]float64{0, 0}, ratios...)
	}

	upSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: timestamps,
		YValues: sliceByteToGigabyte(ups),
	}
	downSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: timestamps,
		YValues: sliceByteToGigabyte(downs),
	}
	bufferSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: timestamps,
		YValues: sliceByteToGigabyte(buffers),
	}
	ratioSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: timestamps,
		YValues: ratios,
	}

	// write individual graphs
	atLeastOneFailed := false
	if err := writeTimeSeriesChart(upSeries, "Upload (Gb)", uploadStatsFile, false); err != nil {
		logThis(errorGeneratingGraph+"for upload: "+err.Error(), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(downSeries, "Download (Gb)", downloadStatsFile, false); err != nil {
		logThis(errorGeneratingGraph+"for download: "+err.Error(), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(bufferSeries, "Buffer (Gb)", bufferStatsFile, false); err != nil {
		logThis(errorGeneratingGraph+"for buffer: "+err.Error(), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(ratioSeries, "Ratio", ratioStatsFile, false); err != nil {
		logThis(errorGeneratingGraph+"for ratio: "+err.Error(), NORMAL)
		atLeastOneFailed = true
	}
	if atLeastOneFailed {
		return errors.New(errorGeneratingGraph)
	}

	// generating stats per day graphs
	dayTimes, upPerDay, downPerDay, bufferPerDay, ratioPerDay, err := t.StatsPerDay(firstOverallTimestamp)
	if err != nil {
		return err
	}

	upPerDaySeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: dayTimes,
		YValues: sliceByteToGigabyte(upPerDay),
	}
	downPerDaySeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: dayTimes,
		YValues: sliceByteToGigabyte(downPerDay),
	}
	bufferPerDaySeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: dayTimes,
		YValues: sliceByteToGigabyte(bufferPerDay),
	}
	ratioPerDaySeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: dayTimes,
		YValues: ratioPerDay,
	}

	// write individual graphs
	if err := writeTimeSeriesChart(upPerDaySeries, "Upload/day (Gb)", uploadPerDayStatsFile, true); err != nil {
		return errors.New("Error generating chart for upload/day: " + err.Error())
	}
	if err := writeTimeSeriesChart(downPerDaySeries, "Download/day (Gb)", downloadPerDayStatsFile, true); err != nil {
		return errors.New("Error generating chart for download/day: " + err.Error())
	}
	if err := writeTimeSeriesChart(bufferPerDaySeries, "Buffer/day (Gb)", bufferPerDayStatsFile, true); err != nil {
		return errors.New("Error generating chart for buffer/day: " + err.Error())
	}
	if err := writeTimeSeriesChart(ratioPerDaySeries, "Ratio/day", ratioPerDayStatsFile, true); err != nil {
		return errors.New("Error generating chart for ratio/day: " + err.Error())
	}

	return nil
}
