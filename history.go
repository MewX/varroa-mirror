package main

import (
	"encoding/csv"
	"fmt"
	"os"
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

type History struct {
	Path          string
	Releases      []*Release
	Records       [][]string
	LastGenerated int
}

func (h *History) Add(r *Release, filter string) error {
	// preparing info
	timestamp := time.Now().Unix()
	// timestamp;filter;artist;title;year;size;type;quality;source;format;tags
	info := []string{fmt.Sprintf("%d", timestamp), filter}
	info = append(info, r.ToSlice()...)

	// write to history file
	f, err := os.OpenFile(h.Path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
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
	h.Releases = append(h.Releases, r)
	h.Records = append(h.Records, info)
	return nil
}

func (h *History) Load(path string) error {
	h.Path = path
	// load history file
	f, err := os.OpenFile(h.Path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewReader(f)
	records, err := w.ReadAll()
	if err != nil {
		return err
	}
	h.Records = records
	// load releases from history to in-memory slice
	for i, record := range records {
		r := &Release{}
		if err := r.FromSlice(record); err != nil {
			logThis(fmt.Sprintf(errorLoadingLine, i), NORMAL)
		} else {
			h.Releases = append(h.Releases, r)
		}
	}
	return nil
}

func (h *History) HasDupe(r *Release) bool {
	// check if r is already in history
	for _, hr := range h.Releases {
		if r.IsDupe(hr) {
			return true
		}
	}
	return false
}

func (h *History) SnatchedPerDay() ([]time.Time, []float64, []float64, error) {
	if len(h.Records) == 0 {
		return nil, nil, nil, errors.New(errorNoHistory)
	}
	// all snatches should already be available in-memory
	// get all times
	allTimes := []time.Time{}
	for _, record := range h.Records {
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
			sizePerDay[len(sizePerDay)-1] += float64(h.Releases[i].size)
		}
	}
	return dayTimes, snatchesPerDay, sizePerDay, nil
}

func (h *History) GenerateGraphs() error {
	if len(h.Releases) == h.LastGenerated {
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

	xAxis := chart.XAxis{
		Style: chart.Style{
			Show: true,
		},
		Name:           "Time",
		NameStyle:      chart.StyleShow(),
		ValueFormatter: chart.TimeValueFormatter,
	}
	sizeSnatchedSeries := chart.TimeSeries{
		Style:   commonStyle,
		Name:    "Size of snatches",
		XValues: timestamps,
		YValues: sliceByteToGigabyte(sizeSnatchedPerDay),
	}
	numberSnatchedSeries := chart.TimeSeries{
		Style:   commonStyle,
		Name:    "Number of snatches",
		XValues: timestamps,
		YValues: numberOfSnatchesPerDay,
	}

	// generate graphs
	if err := writeTimeSeriesChart(xAxis, sizeSnatchedSeries, "Size snatched (Gb) per day", sizeSnatchedPerDayFile); err != nil {
		return err
	}
	if err := writeTimeSeriesChart(xAxis, numberSnatchedSeries, "Number of snatches per day", numberSnatchedPerDayFile); err != nil {
		return err
	}

	// generate pie chart
	filterHits := map[string]float64{}
	for _, r := range h.Records {
		filterHits[r[1]] += 1
	}
	if err := writePieChart(filterHits, "Total snatches by filter", totalSnatchesByFilterFile); err != nil {
		return err
	}

	// keep total number of snatches as reference for later
	h.LastGenerated = len(h.Releases)
	return nil
}
