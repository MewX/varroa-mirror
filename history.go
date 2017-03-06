package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/wcharczuk/go-chart"
)

const (
	errorLoadingLine      = "Error loading line %d of history file"
	errorNoHistory        = "No history yet"
	errorInvalidTimestamp = "Error parsing timestamp"
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
)

type History struct {
	SnatchesPath        string
	SnatchedReleases    []*Release
	SnatchesRecords     [][]string
	LastGeneratedPerDay int

	TrackerStatsPath    string
	TrackerStatsRecords [][]string
	TrackerStats        []*TrackerStats
}

func (h *History) AddSnatch(r *Release, filter string) error {
	// preparing info
	timestamp := time.Now().Unix()
	// timestamp;filter;artist;title;year;size;type;quality;source;format;tags
	info := []string{fmt.Sprintf("%d", timestamp), filter}
	info = append(info, r.ToSlice()...)

	// write to history file
	f, err := os.OpenFile(h.SnatchesPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
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
	h.SnatchedReleases = append(h.SnatchedReleases, r)
	h.SnatchesRecords = append(h.SnatchesRecords, info)
	return nil
}

func (h *History) Load(statsFile, snatchesFile string) error {
	h.SnatchesPath = snatchesFile
	h.TrackerStatsPath = statsFile
	// load history file
	f, err := os.OpenFile(h.SnatchesPath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewReader(f)
	records, err := w.ReadAll()
	if err != nil {
		return err
	}
	h.SnatchesRecords = records
	// load releases from history to in-memory slice
	for i, record := range records {
		r := &Release{}
		if err := r.FromSlice(record); err != nil {
			logThis(fmt.Sprintf(errorLoadingLine, i), NORMAL)
		} else {
			h.SnatchedReleases = append(h.SnatchedReleases, r)
		}
	}
	// load tracker stats
	f2, err := os.OpenFile(h.TrackerStatsPath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f2.Close()
	w2 := csv.NewReader(f2)
	trackerStats, err := w2.ReadAll()
	if err != nil {
		return err
	}
	h.TrackerStatsRecords = trackerStats
	// load stats to in-memory slice
	for i, stats := range trackerStats {
		r := &TrackerStats{}
		if err := r.FromSlice(stats); err != nil {
			logThis(fmt.Sprintf(errorLoadingLine, i), NORMAL)
		} else {
			h.TrackerStats = append(h.TrackerStats, r)
		}
	}
	return nil
}

func (h *History) HasDupe(r *Release) bool {
	// check if r is already in history
	for _, hr := range h.SnatchedReleases {
		if r.IsDupe(hr) {
			return true
		}
	}
	return false
}

func (h *History) SnatchedPerDay() ([]time.Time, []float64, []float64, error) {
	if len(h.SnatchesRecords) == 0 {
		return nil, nil, nil, errors.New(errorNoHistory)
	}
	// all snatches should already be available in-memory
	// get all times
	allTimes := []time.Time{}
	for _, record := range h.SnatchesRecords {
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
			sizePerDay[len(sizePerDay)-1] += float64(h.SnatchedReleases[i].size)
		}
	}
	return dayTimes, snatchesPerDay, sizePerDay, nil
}

func (h *History) GenerateDailyGraphs() error {
	if len(h.SnatchedReleases) == h.LastGeneratedPerDay {
		// no additional snatch since the graphs were last generated, nothing needs to be done
		return nil
	}
	// get slices of relevant data
	timestamps, numberOfSnatchesPerDay, sizeSnatchedPerDay, err := h.SnatchedPerDay()
	if err != nil {
		if err.Error() == errorNoHistory {
			return nil // nothing to do yet
		} else {
			return err
		}
	}
	if len(timestamps) < 2 {
		return nil // not enough days yet
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

	// generate pie chart
	filterHits := map[string]float64{}
	for _, r := range h.SnatchesRecords {
		filterHits[r[1]] += 1
	}
	if err := writePieChart(filterHits, "Total snatches by filter", totalSnatchesByFilterFile); err != nil {
		return err
	}

	// keep total number of snatches as reference for later
	h.LastGeneratedPerDay = len(h.SnatchedReleases)
	return nil
}

func (h *History) AddStats(stats *TrackerStats) error {
	h.TrackerStats = append(h.TrackerStats, stats)
	// prepare csv fields
	timestamp := time.Now().Unix()
	newStats := []string{fmt.Sprintf("%d", timestamp)}
	newStats = append(newStats, stats.ToSlice()...)
	h.TrackerStatsRecords = append(h.TrackerStatsRecords, newStats)
	// append to file
	f, err := os.OpenFile(h.TrackerStatsPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
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

func (h *History) GenerateStatsGraphs() error {
	// generate tracker stats graphs
	if len(h.TrackerStatsRecords) < 2 {
		return nil // not enough data points yet
	}
	if len(h.TrackerStatsRecords) != len(h.TrackerStats) {
		return errors.New("Incoherent in-memory stats")
	}
	//  generate data slices
	timestamps := []time.Time{}
	ups := []float64{}
	downs := []float64{}
	buffers := []float64{}
	ratios := []float64{}
	for _, stats := range h.TrackerStatsRecords {
		timestamp, err := strconv.ParseInt(stats[0], 10, 64)
		if err != nil {
			return errors.New(errorInvalidTimestamp)
		}
		timestamps = append(timestamps, time.Unix(timestamp, 0))
	}
	if len(timestamps) < 2 {
		return errors.New(errorNotEnoughDataPoints)
	}
	for _, stats := range h.TrackerStats {
		ups = append(ups, float64(stats.Up))
		downs = append(downs, float64(stats.Down))
		buffers = append(buffers, float64(stats.Buffer))
		ratios = append(ratios, float64(stats.Ratio))
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

func (h *History) GenerateGraphs() error {
	// prepare directory for pngs if necessary
	if !DirectoryExists(statsDir) {
		if err := os.MkdirAll(statsDir, 0777); err != nil {
			return err
		}
	}
	// generate history graphs if necessary
	if err := h.GenerateDailyGraphs(); err != nil {
		return err
	}
	// generate stats graphs
	if err := h.GenerateStatsGraphs(); err != nil {
		return err
	}
	// combine graphs into overallStatsFile
	return combineAllGraphs(overallStatsFile, uploadStatsFile, downloadStatsFile, bufferStatsFile, ratioStatsFile, numberSnatchedPerDayFile, sizeSnatchedPerDayFile, totalSnatchesByFilterFile)
}
