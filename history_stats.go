package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/wcharczuk/go-chart"
)

type TrackerStatsHistory struct {
	TrackerStatsPath string
	TrackerStats     []*TrackerStats
}

func (t *TrackerStatsHistory) Load(statsFile string, statsConfig *ConfigStats) error {
	t.TrackerStatsPath = statsFile
	// load tracker stats
	f, err := os.OpenFile(t.TrackerStatsPath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	w := csv.NewReader(f)
	trackerStats, err := w.ReadAll()
	if err != nil {
		return err
	}

	if len(trackerStats) == 0 {
		return nil
	}

	// detecting the old CSV format that included buffer & warning buffer values
	saveStats := false
	if len(trackerStats[0]) == 6 {
		logThis.Info(fmt.Sprintf("The CSV file %s seems to use an old format. Migrating to new format.", t.TrackerStatsPath), NORMAL)
		// closing file
		f.Close()
		// flag to resave the stats
		saveStats = true
		// moving it to backup name
		if err := os.Rename(t.TrackerStatsPath, t.TrackerStatsPath+"_old"); err != nil {
			return errors.New("Error while trying to migrate CSV formats")
		}
	}

	// load stats to in-memory slice
	for i, stats := range trackerStats {
		r := &TrackerStats{}
		if err := r.FromSlice(stats, statsConfig); err != nil {
			logThis.Error(errors.Wrap(err, fmt.Sprintf(errorLoadingLine, i)), NORMAL)
		} else {
			t.TrackerStats = append(t.TrackerStats, r)
		}
	}

	// if necessary, resave the whole file
	if saveStats {
		// since the file was renamed, the original filename should not exist anymore.
		// adding the stats will create a new file.
		for _, s := range t.TrackerStats {
			if err := t.Add(s); err != nil {
				return errors.New("Error migrating the CSV file to the new format: backup was previously made to " + t.TrackerStatsPath + "_old")
			}
		}
		logThis.Info(fmt.Sprintf("The CSV file %s for stats has been migrated, the old file has been renamed %s_old", t.TrackerStatsPath, t.TrackerStatsPath), NORMAL)
	} else {
		f.Close()
	}
	return nil
}

func (t *TrackerStatsHistory) Add(stats *TrackerStats) error {
	t.TrackerStats = append(t.TrackerStats, stats)
	// append to file
	f, err := os.OpenFile(t.TrackerStatsPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(stats.ToSlice()); err != nil {
		return err
	}
	w.Flush()
	return nil
}

func (t *TrackerStatsHistory) StatsPerDay(firstTimestamp time.Time) ([]time.Time, []float64, []float64, []float64, []float64, error) {
	if len(t.TrackerStats) == 0 {
		return nil, nil, nil, nil, nil, errors.New(errorNoHistory)
	}
	// all snatches should already be available in-memory
	// get all times
	allTimes := []time.Time{}
	for _, record := range t.TrackerStats {
		allTimes = append(allTimes, time.Unix(record.Timestamp, 0))
	}
	// slice snatches data per day
	dayTimes := allDaysSince(firstTimestamp)
	statsAtStartOfDay := []*TrackerStats{}
	previousVirtualStats := &TrackerStats{}
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

		if beforeIndex == -1 || afterIndex == -1 {
			// adding previous stats since we don't have stats for this day
			statsAtStartOfDay = append(statsAtStartOfDay, previousVirtualStats)
		} else {
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
			virtualStats.Buffer = int64(bufferSlope*float64(d.Unix()) + bufferOffset)
			ratioSlope := float64((t.TrackerStats[afterIndex].Ratio - t.TrackerStats[beforeIndex].Ratio) / float64(allTimes[afterIndex].Unix()-allTimes[beforeIndex].Unix()))
			ratioOffset := t.TrackerStats[beforeIndex].Ratio - ratioSlope*float64(allTimes[beforeIndex].Unix())
			virtualStats.Ratio = ratioSlope*float64(d.Unix()) + ratioOffset
			// keep the virtual stats in memory
			statsAtStartOfDay = append(statsAtStartOfDay, virtualStats)
			previousVirtualStats = virtualStats
		}
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

func (t *TrackerStatsHistory) GenerateStatsGraphs(firstOverallTimestamp time.Time, updatePeriod int, uploadFile, downloadFile, bufferFile, ratioFile, uploadPerDayFile, downloadPerDayFile, bufferPerDayFile, ratioPerDayFile string) error {
	// generate tracker stats graphs
	if len(t.TrackerStats) <= 2 {
		// not enough data points yet
		return errors.New("Empty stats history")
	}
	//  generate data slices
	timestamps := []time.Time{}
	ups := []float64{}
	downs := []float64{}
	buffers := []float64{}
	ratios := []float64{}
	for _, stats := range t.TrackerStats {
		timestamps = append(timestamps, time.Unix(stats.Timestamp, 0))
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
		timestamps = append([]time.Time{firstOverallTimestamp, timestamps[0].Add(time.Duration(-updatePeriod) * time.Hour)}, timestamps...)
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
	if err := writeTimeSeriesChart(upSeries, "Upload (Gb)", uploadFile, false); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for upload"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(downSeries, "Download (Gb)", downloadFile, false); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for download"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(bufferSeries, "Buffer (Gb)", bufferFile, false); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for buffer"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(ratioSeries, "Ratio", ratioFile, false); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for ratio"), NORMAL)
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
	if err := writeTimeSeriesChart(upPerDaySeries, "Upload/day (Gb)", uploadPerDayFile, true); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for upload/day"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(downPerDaySeries, "Download/day (Gb)", downloadPerDayFile, true); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for download/day"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(bufferPerDaySeries, "Buffer/day (Gb)", bufferPerDayFile, true); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for buffer/day"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(ratioPerDaySeries, "Ratio/day", ratioPerDayFile, true); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for ratio/day"), NORMAL)
		atLeastOneFailed = true
	}
	if atLeastOneFailed {
		return errors.New(errorGeneratingGraph)
	}

	return nil
}
