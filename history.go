package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/wcharczuk/go-chart"
)

const (
	errorLoadingLine      = "Error loading line %d of history file"
	errorNoHistory        = "No history yet"
	errorInvalidTimestamp = "Error parsing timestamp"
	errorNotEnoughDays    = "Not enough days in history to generate daily graphs"
)

var (
	statsDir                  = "stats"
	uploadStatsFile           = filepath.Join(statsDir, "up.png")
	downloadStatsFile         = filepath.Join(statsDir, "down.png")
	ratioStatsFile            = filepath.Join(statsDir, "ratio.png")
	bufferStatsFile           = filepath.Join(statsDir, "buffer.png")
	overallStatsFile          = filepath.Join(statsDir, "stats.png")
	numberSnatchedPerDayFile  = filepath.Join(statsDir, "snatches_per_day.png")
	sizeSnatchedPerDayFile    = filepath.Join(statsDir, "size_snatched_per_day.png")
	totalSnatchesByFilterFile = filepath.Join(statsDir, "total_snatched_by_filter.png")
	toptagsFile               = filepath.Join(statsDir, "top_tags.png")
)

// History manages stats and generates graphs.
type History struct {
	SnatchHistory
	TrackerStatsHistory
}

func (h *History) LoadAll(statsFile, snatchesFile string) error {
	if err := h.TrackerStatsHistory.Load(statsFile); err != nil {
		return err
	}
	if err := h.SnatchHistory.Load(snatchesFile); err != nil {
		return err
	}
	return nil
}

func (h *History) GenerateGraphs() error {
	// prepare directory for pngs if necessary
	if !DirectoryExists(statsDir) {
		if err := os.MkdirAll(statsDir, 0777); err != nil {
			return err
		}
	}
	// get first overall timestamp in all history sources
	firstOverallTimestamp := h.getFirstTimestamp()
	if firstOverallTimestamp.After(time.Now()) {
		return errors.New(errorInvalidTimestamp)
	}

	// generate history graphs if necessary
	if err := h.GenerateDailyGraphs(firstOverallTimestamp); err != nil {
		return err
	}
	// generate stats graphs
	if err := h.GenerateStatsGraphs(firstOverallTimestamp); err != nil {
		return err
	}
	// combine graphs into overallStatsFile
	return combineAllGraphs(overallStatsFile, uploadStatsFile, downloadStatsFile, bufferStatsFile, ratioStatsFile, numberSnatchedPerDayFile, sizeSnatchedPerDayFile, totalSnatchesByFilterFile, toptagsFile)
}

func (h *History) getFirstTimestamp() time.Time {
	snatchTimestamp, err := strconv.ParseInt(h.SnatchesRecords[0][0], 0, 64)
	if err != nil {
		snatchTimestamp = math.MaxInt32 // max timestamp
	}
	statsTimestamp, err := strconv.ParseInt(h.TrackerStatsRecords[0][0], 0, 64)
	if err != nil {
		statsTimestamp = math.MaxInt32 // max timestamp
	}
	if snatchTimestamp < statsTimestamp {
		return time.Unix(snatchTimestamp, 0)
	}
	return time.Unix(statsTimestamp, 0)
}

//----------------------------------------------------------------------------------------------------------------------

type SnatchHistory struct {
	SnatchesPath        string
	SnatchedReleases    []*Release
	SnatchesRecords     [][]string
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
	w := csv.NewReader(f)
	records, err := w.ReadAll()
	if err != nil {
		return err
	}
	s.SnatchesRecords = records
	// load releases from history to in-memory slice
	for i, record := range records {
		r := &Release{}
		if err := r.FromSlice(record); err != nil {
			logThis(fmt.Sprintf(errorLoadingLine, i), NORMAL)
		} else {
			s.SnatchedReleases = append(s.SnatchedReleases, r)
		}
	}
	return nil
}

func (s *SnatchHistory) Add(r *Release, filter string) error {
	// preparing info
	timestamp := time.Now().Unix()
	// timestamp;filter;artist;title;year;size;type;quality;source;format;tags
	info := []string{fmt.Sprintf("%d", timestamp), filter}
	info = append(info, r.ToSlice()...)

	// write to history file
	f, err := os.OpenFile(s.SnatchesPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(info); err != nil {
		return err
	}
	w.Flush()
	// add to in-memory slices
	s.SnatchedReleases = append(s.SnatchedReleases, r)
	s.SnatchesRecords = append(s.SnatchesRecords, info)
	return nil
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

func (s *SnatchHistory) SnatchedPerDay() ([]time.Time, []float64, []float64, error) {
	if len(s.SnatchesRecords) == 0 {
		return nil, nil, nil, errors.New(errorNoHistory)
	}
	// all snatches should already be available in-memory
	// get all times
	allTimes := []time.Time{}
	for _, record := range s.SnatchesRecords {
		timestamp, err := strconv.ParseInt(record[0], 0, 64)
		if err != nil {
			return nil, nil, nil, errors.New(errorInvalidTimestamp)
		}
		allTimes = append(allTimes, time.Unix(timestamp, 0))
	}
	// slice snatches data per day
	firstDay := startOfDay(allTimes[0])
	tomorrow := nextDay(startOfDay(time.Now()))
	dayTimes := []time.Time{}
	snatchesPerDay := []float64{}
	sizePerDay := []float64{}
	for t := firstDay; t.Before(tomorrow); t = nextDay(t) {
		dayTimes = append(dayTimes, t)
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
			sizePerDay[len(sizePerDay)-1] += float64(s.SnatchedReleases[i].size)
		}
	}
	return dayTimes, snatchesPerDay, sizePerDay, nil
}

func (s *SnatchHistory) GenerateDailyGraphs(firstOverallTimestamp time.Time) error {
	if len(s.SnatchedReleases) == s.LastGeneratedPerDay {
		// no additional snatch since the graphs were last generated, nothing needs to be done
		return nil
	}
	// get slices of relevant data
	timestamps, numberOfSnatchesPerDay, sizeSnatchedPerDay, err := s.SnatchedPerDay()
	if err != nil {
		if err.Error() == errorNoHistory {
			logThis(errorNoHistory, NORMAL)
			return nil // nothing to do yet
		} else {
			return err
		}
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
	if err := writeTimeSeriesChart(sizeSnatchedSeries, "Size snatched (Gb) per day", sizeSnatchedPerDayFile); err != nil {
		return err
	}
	if err := writeTimeSeriesChart(numberSnatchedSeries, "Number of snatches per day", numberSnatchedPerDayFile); err != nil {
		return err
	}

	// generate filters chart
	filterHits := map[string]float64{}
	for _, r := range s.SnatchesRecords {
		filterHits[r[1]] += 1
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
		for _, t := range r.tags {
			popularTags[t] += 1
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

func (t *TrackerStatsHistory) GenerateStatsGraphs(firstOverallTimestamp time.Time) error {
	// generate tracker stats graphs
	if len(t.TrackerStatsRecords) < 2 {
		return nil // not enough data points yet
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
		timestamps = append([]time.Time{firstOverallTimestamp, timestamps[0].Add(time.Duration(-conf.statsUpdatePeriod) * time.Hour)}, timestamps...)
		ups = append([]float64{0}, ups...)
		downs = append([]float64{0}, downs...)
		buffers = append([]float64{0}, buffers...)
		ratios = append([]float64{0}, ratios...)
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
	if err := writeTimeSeriesChart(upSeries, "Upload (Gb)", uploadStatsFile); err != nil {
		return err
	}
	if err := writeTimeSeriesChart(downSeries, "Download (Gb)", downloadStatsFile); err != nil {
		return err
	}
	if err := writeTimeSeriesChart(bufferSeries, "Buffer (Gb)", bufferStatsFile); err != nil {
		return err
	}
	if err := writeTimeSeriesChart(ratioSeries, "Ratio", ratioStatsFile); err != nil {
		return err
	}
	return nil
}
