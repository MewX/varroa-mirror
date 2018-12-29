package varroa

import (
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/wcharczuk/go-chart"
)

// StatsSeries is a struct that holds the Stats data needed to generate time series graphs.
// It can then draw and save the graphs (for raw stats or stats/time unit)
type StatsSeries struct {
	Tracker       string
	Time          []time.Time
	Up            []float64
	Down          []float64
	Ratio         []float64
	Buffer        []float64
	WarningBuffer []float64
}

// AddStats for all entries or a selection to get the correct timeseries
func (ss *StatsSeries) AddStats(entries ...StatsEntry) error {
	conf, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		return err
	}
	statsConfig, err := conf.GetStats(ss.Tracker)
	if err != nil {
		return err
	}
	// accumulate entries, converting to Gb directly
	for _, e := range entries {
		ss.Time = append(ss.Time, e.Timestamp)
		ss.Up = append(ss.Up, float64(e.Up)/(1024*1024*1024))
		ss.Down = append(ss.Down, float64(e.Down)/(1024*1024*1024))
		ss.Ratio = append(ss.Ratio, e.Ratio)
		ss.Buffer = append(ss.Buffer, (float64(e.Up)/statsConfig.TargetRatio-float64(e.Down))/(1024*1024*1024))
		ss.WarningBuffer = append(ss.WarningBuffer, (float64(e.Up)/warningRatio-float64(e.Down))/(1024*1024*1024))
	}
	return nil
}

// AddDeltas accumulates the data from a selection of StatsDeltas
func (ss *StatsSeries) AddDeltas(entries ...StatsDelta) error {
	// accumulate entries, converting to Gb directly
	for _, e := range entries {
		ss.Time = append(ss.Time, e.Timestamp)
		ss.Up = append(ss.Up, float64(e.Up)/(1024*1024*1024))
		ss.Down = append(ss.Down, float64(e.Down)/(1024*1024*1024))
		ss.Ratio = append(ss.Ratio, e.Ratio)
		ss.Buffer = append(ss.Buffer, float64(e.Buffer)/(1024*1024*1024))
		ss.WarningBuffer = append(ss.WarningBuffer, float64(e.WarningBuffer)/(1024*1024*1024))
	}
	return nil
}

// GenerateGraphs: time series graphs for up, down, ratio, buffer, warningbuffer
func (ss *StatsSeries) GenerateGraphs(directory, prefix string, firstTimestamp time.Time, addSMA bool) error {
	// check we have some data
	if len(ss.Time) < 2 {
		return errors.New("not enough data points to generate graphs")
	}

	// firstTimestamp is the beginning of the graphs.
	// if it's not the timestamp of the first sample, we need to add it.
	if !firstTimestamp.Equal(ss.Time[0]) {
		// if the first timestamp isn't in the stats list, artificially add it and another point right before the first data point
		ss.Time = append([]time.Time{firstTimestamp, ss.Time[0].Add(-1 * time.Hour)}, ss.Time...)
		ss.Up = append([]float64{0, 0}, ss.Up...)
		ss.Down = append([]float64{0, 0}, ss.Down...)
		ss.Ratio = append([]float64{0, 0}, ss.Ratio...)
		ss.Buffer = append([]float64{0, 0}, ss.Buffer...)
		ss.WarningBuffer = append([]float64{0, 0}, ss.WarningBuffer...)
	}

	upSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: ss.Time,
		YValues: ss.Up,
	}
	downSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: ss.Time,
		YValues: ss.Down,
	}
	bufferSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: ss.Time,
		YValues: ss.Buffer,
	}
	warningBufferSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: ss.Time,
		YValues: ss.WarningBuffer,
	}
	ratioSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: ss.Time,
		YValues: ss.Ratio,
	}

	// TODO titles for stats graphs only, not for stats/time!!!!!!!

	// write individual graphs
	atLeastOneFailed := false
	if err := writeTimeSeriesChart(upSeries, "Upload (Gb)", filepath.Join(directory, prefix+uploadStatsFile), addSMA); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for upload"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(downSeries, "Download (Gb)", filepath.Join(directory, prefix+downloadStatsFile), addSMA); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for download"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(bufferSeries, "Buffer (Gb)", filepath.Join(directory, prefix+bufferStatsFile), addSMA); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for buffer"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(warningBufferSeries, "Warning Buffer (Gb)", filepath.Join(directory, prefix+warningBufferStatsFile), addSMA); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for warning buffer"), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(ratioSeries, "Ratio", filepath.Join(directory, prefix+ratioStatsFile), addSMA); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for ratio"), NORMAL)
		atLeastOneFailed = true
	}
	if atLeastOneFailed {
		return errors.New(errorGeneratingGraph)
	}
	return nil
}

// ------------------------

// SnatchStatsSeries is a struct that holds the SnatchStats data needed to generate time series graphs.
// It can then draw and save the graphs (for raw stats or stats/time unit)
type SnatchStatsSeries struct {
	Tracker string
	Time    []time.Time
	Number  []float64
	Size    []float64
}

// AddStats for all entries or a selection to get the correct timeseries
func (sss *SnatchStatsSeries) AddStats(entries ...SnatchStatsEntry) {
	// accumulate entries, converting to Gb directly
	for _, e := range entries {
		sss.Time = append(sss.Time, e.Timestamp)
		sss.Number = append(sss.Number, float64(e.Number))
		sss.Size = append(sss.Size, float64(e.Size)/(1024*1024*1024))
	}
}

// GenerateGraphs: time series graphs for size, number
func (sss *SnatchStatsSeries) GenerateGraphs(directory, prefix string, firstTimestamp time.Time, addSMA bool) error {
	// check we have some data
	if len(sss.Time) < 2 {
		return errors.New("not enough data points to generate graphs")
	}

	// firstTimestamp is the beginning of the graphs.
	// if it's not the timestamp of the first sample, we need to add it.
	if !firstTimestamp.Equal(sss.Time[0]) {
		// if the first timestamp isn't in the stats list, artificially add it and another point right before the first data point
		sss.Time = append([]time.Time{firstTimestamp, sss.Time[0].Add(-1 * time.Hour)}, sss.Time...)
		sss.Number = append([]float64{0, 0}, sss.Number...)
		sss.Size = append([]float64{0, 0}, sss.Size...)
	}

	numberSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: sss.Time,
		YValues: sss.Number,
	}
	sizeSeries := chart.TimeSeries{
		Style:   commonStyle,
		XValues: sss.Time,
		YValues: sss.Size,
	}

	// write individual graphs
	atLeastOneFailed := false
	if err := writeTimeSeriesChart(numberSeries, "Number of snatches/day", filepath.Join(directory, prefix+numberSnatchedPerDayFile), addSMA); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for number "), NORMAL)
		atLeastOneFailed = true
	}
	if err := writeTimeSeriesChart(sizeSeries, "Size snatched/day with varroa musica (Gb)", filepath.Join(directory, prefix+sizeSnatchedPerDayFile), addSMA); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraph+" for size snatched/day"), NORMAL)
		atLeastOneFailed = true
	}
	if atLeastOneFailed {
		return errors.New(errorGeneratingGraph)
	}
	return nil
}
