package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/wcharczuk/go-chart"
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

	commonStyle = chart.Style{
		Show:        true,
		StrokeColor: chart.ColorBlue,
		FillColor:   chart.ColorBlue.WithAlpha(25),
	}
)

func sliceByteToGigabyte(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = v / (1024 * 1024 * 1024)
	}
	return out
}

func writePie(values map[string]float64, title, filename string) error {
	// map to []chart.Value
	pieSlices := []chart.Value{}
	for k, v := range values {
		pieSlices = append(pieSlices, chart.Value{Value: v, Label: fmt.Sprintf("%s (%d)", k, int(v))})
	}
	// pie chart
	pie := chart.PieChart{
		Title: title,
		TitleStyle: chart.Style{
			Show:      true,
			FontColor: chart.ColorBlack,
			FontSize:  chart.DefaultTitleFontSize,
		},
		Values: pieSlices,
	}
	// generate image
	buffer := bytes.NewBuffer([]byte{})
	if err := pie.Render(chart.PNG, buffer); err != nil {
		return err
	}
	return ioutil.WriteFile(filename, buffer.Bytes(), 0644)
}

func writeGraph(xAxis chart.XAxis, series chart.Series, axisLabel, filename string) error {
	graphUp := chart.Chart{
		XAxis: xAxis,
		YAxis: chart.YAxis{
			Style:     chart.StyleShow(),
			Name:      axisLabel,
			NameStyle: chart.StyleShow(),
		},
		Series: []chart.Series{
			series,
		},
	}
	buffer := bytes.NewBuffer([]byte{})
	if err := graphUp.Render(chart.PNG, buffer); err != nil {
		return err
	}
	return ioutil.WriteFile(filename, buffer.Bytes(), 0644)
}

func combineGraphs(up, down, buffer, ratio, combined string) error {
	// open and decode images
	imgFile1, err1 := os.Open(up)
	imgFile2, err2 := os.Open(down)
	imgFile3, err3 := os.Open(buffer)
	imgFile4, err4 := os.Open(ratio)
	if err := checkErrors(err1, err2, err3, err4); err != nil {
		return err
	}
	img1, _, err1 := image.Decode(imgFile1)
	img2, _, err2 := image.Decode(imgFile2)
	img3, _, err3 := image.Decode(imgFile3)
	img4, _, err4 := image.Decode(imgFile4)
	if err := checkErrors(err1, err2, err3, err4); err != nil {
		return err
	}

	// create the rectangle, assuming they all have the same size
	// ------------
	// |  1  | 2  |
	// ------------
	// |  3  | 4  |
	// ------------

	sp2 := image.Point{img1.Bounds().Dx(), 0}
	r2 := image.Rectangle{sp2, sp2.Add(img2.Bounds().Size())}

	sp3 := image.Point{0, img1.Bounds().Dy()}
	r3 := image.Rectangle{sp3, sp3.Add(img3.Bounds().Size())}

	sp4 := img1.Bounds().Size()
	r4 := image.Rectangle{sp4, sp4.Add(img4.Bounds().Size())}

	//rectangle for the big image
	r := image.Rectangle{image.Point{0, 0}, r4.Max}

	// new image
	rgba := image.NewRGBA(r)
	// draw original images in new rectangle
	draw.Draw(rgba, img1.Bounds(), img1, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, r2, img2, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, r3, img3, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, r4, img4, image.Point{0, 0}, draw.Src)

	// save new image
	out, err := os.Create(combined)
	if err != nil {
		fmt.Println(err)
	}
	return png.Encode(out, rgba)
}

func generateGraph() error {
	// prepare directory for pngs if necessary
	if !DirectoryExists(statsDir) {
		os.MkdirAll(statsDir, 0777)
	}
	// generate history stats graphs if necessary
	if err := history.GenerateGraphs(); err != nil {
		return err
	}
	// generate tracker stats graphs
	f, err := os.OpenFile(conf.statsFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	w := csv.NewReader(f)
	records, err := w.ReadAll()
	if err != nil {
		return err
	}

	//  create []time.Time{} from timestamps
	//  create []float64 from buffer
	timestamps := []time.Time{}
	ups := []float64{}
	downs := []float64{}
	buffers := []float64{}
	ratios := []float64{}
	for _, stats := range records {
		timestamp, err := strconv.ParseInt(stats[0], 10, 64)
		if err != nil {
			continue // bad line
		}
		timestamps = append(timestamps, time.Unix(timestamp, 0))

		up, err := strconv.ParseUint(stats[1], 10, 64)
		if err != nil {
			continue // bad line
		}
		ups = append(ups, float64(up))

		down, err := strconv.ParseUint(stats[2], 10, 64)
		if err != nil {
			continue // bad line
		}
		downs = append(downs, float64(down))

		buffer, err := strconv.ParseUint(stats[4], 10, 64)
		if err != nil {
			continue // bad line
		}
		buffers = append(buffers, float64(buffer))

		ratio, err := strconv.ParseFloat(stats[3], 64)
		if err != nil {
			continue // bad line
		}
		ratios = append(ratios, ratio)
	}
	if len(timestamps) < 2 {
		return errors.New(errorNotEnoughDataPoints)
	}

	upSeries := chart.TimeSeries{
		Style:   commonStyle,
		Name:    "Upload",
		XValues: timestamps,
		YValues: sliceByteToGigabyte(ups),
	}
	downSeries := chart.TimeSeries{
		Style:   commonStyle,
		Name:    "Download",
		XValues: timestamps,
		YValues: sliceByteToGigabyte(downs),
	}
	bufferSeries := chart.TimeSeries{
		Style:   commonStyle,
		Name:    "Buffer",
		XValues: timestamps,
		YValues: sliceByteToGigabyte(buffers),
	}
	ratioSeries := chart.TimeSeries{
		Style:   commonStyle,
		Name:    "Ratio",
		XValues: timestamps,
		YValues: ratios,
	}
	xAxis := chart.XAxis{
		Style: chart.Style{
			Show: true,
		},
		Name:           "Time",
		NameStyle:      chart.StyleShow(),
		ValueFormatter: chart.TimeValueFormatter,
	}

	// write individual graphs
	if err := writeGraph(xAxis, upSeries, "Upload (Gb)", uploadStatsFile); err != nil {
		return err
	}
	if err := writeGraph(xAxis, downSeries, "Download (Gb)", downloadStatsFile); err != nil {
		return err
	}
	if err := writeGraph(xAxis, bufferSeries, "Buffer (Gb)", bufferStatsFile); err != nil {
		return err
	}
	if err := writeGraph(xAxis, ratioSeries, "Ratio", ratioStatsFile); err != nil {
		return err
	}
	// combine graphs into overallStatsFile
	return combineGraphs(uploadStatsFile, downloadStatsFile, bufferStatsFile, ratioStatsFile, overallStatsFile)
}
