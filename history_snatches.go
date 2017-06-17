package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/wcharczuk/go-chart"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

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
		logThis.Error(errors.Wrap(err, "Error reading history file"), NORMAL)
		return err
	}
	if len(bytes) == 0 {
		// newly created file
		return nil
	}

	s.SnatchesPacked = bytes
	// load releases from history to in-memory slice
	err = msgpack.Unmarshal(bytes, &s.SnatchedReleases)
	if err != nil {
		logThis.Error(errors.Wrap(err, "Error loading releases from history file"), NORMAL)
	}
	// fix empty filters, if any
	for i := range s.SnatchedReleases {
		if s.SnatchedReleases[i].Filter == "" {
			s.SnatchedReleases[i].Filter = "remote"
		}
	}
	return err
}

func (s *SnatchHistory) AddSnatch(r *Release, filter string) error {
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

func (s *SnatchHistory) HasReleaseFromGroup(r *Release) bool {
	// check if a torrent of the same torrent group is already in history
	for _, hr := range s.SnatchedReleases {
		if r.IsInSameGroup(hr) {
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

func (s *SnatchHistory) GenerateDailyGraphs(firstOverallTimestamp time.Time, sizeSnatchedFile, numberSnatched, totalByFilter, topTags string) error {
	if len(s.SnatchedReleases) == s.LastGeneratedPerDay {
		// no additional snatch since the graphs were last generated, nothing needs to be done
		return errors.New(errorNoFurtherSnatches)
	}
	// keep total number of snatches as reference for later
	s.LastGeneratedPerDay = len(s.SnatchedReleases)

	// generate filters chart
	filterHits := map[string]float64{}
	for _, r := range s.SnatchedReleases {
		filterHits[r.Filter]++
	}
	pieSlices := []chart.Value{}
	for k, v := range filterHits {
		pieSlices = append(pieSlices, chart.Value{Value: v, Label: fmt.Sprintf("%s (%d)", k, int(v))})
	}
	if err := writePieChart(pieSlices, "Total snatches by filter", totalByFilter); err != nil {
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
	if err := writePieChart(top10tags, "Top tags", topTags); err != nil {
		return err
	}

	// generate snatches/day stats
	// get slices of relevant data
	timestamps, numberOfSnatchesPerDay, sizeSnatchedPerDay, err := s.SnatchedPerDay(firstOverallTimestamp)
	if err != nil {
		if err.Error() == errorNoHistory {
			logThis.Error(err, NORMAL)
			return nil // nothing to do yet
		}
		return err
	}
	if len(timestamps) < 2 {
		logThis.Info(errorNotEnoughDays, NORMAL)
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
	if err := writeTimeSeriesChart(sizeSnatchedSeries, "Size snatched/day (Gb)", sizeSnatchedFile, true); err != nil {
		return err
	}
	return writeTimeSeriesChart(numberSnatchedSeries, "Snatches/day", numberSnatched, true)
}
