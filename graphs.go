package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/wcharczuk/go-chart"
	"github.com/wcharczuk/go-chart/drawing"
)

var (
	commonStyle = chart.Style{
		Show:        true,
		StrokeColor: chart.ColorBlue,
		FillColor:   chart.ColorBlue.WithAlpha(25),
	}
	commonStyleSVG = chart.Style{
		Show:        true,
		StrokeColor: drawing.ColorFromHex("f57f17"),
		FillColor:   drawing.ColorFromHex("f57f17").WithAlpha(80),
		FontColor:   chart.ColorWhite,
	}
	timeAxis = chart.XAxis{
		Style:          chart.StyleShow(),
		Name:           "Time",
		NameStyle:      chart.StyleShow(),
		ValueFormatter: chart.TimeValueFormatter,
	}
	timeAxisSVG = chart.XAxis{
		Style: chart.Style{
			Show:        true,
			FontColor:   chart.ColorWhite,
			StrokeColor: chart.ColorWhite,
		},
		Name: "Time",
		NameStyle: chart.Style{
			Show:      true,
			FontColor: chart.ColorWhite,
		},
		ValueFormatter: chart.TimeValueFormatter,
	}
)

func sliceByteToGigabyte(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = v / (1024 * 1024 * 1024)
	}
	return out
}

func writePieChart(values []chart.Value, title, filename string) error {
	// pie chart
	pie := chart.PieChart{
		Height: 1000,
		Width:  2000,
		Title:  title,
		TitleStyle: chart.Style{
			Show:      true,
			FontColor: chart.ColorBlack,
			FontSize:  chart.DefaultTitleFontSize,
		},
		Values: values,
	}
	// generate image
	// generate SVG
	bufferSVG := bytes.NewBuffer([]byte{})
	if err := pie.Render(chart.SVG, bufferSVG); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filename+svgExt, bufferSVG.Bytes(), 0644); err != nil {
		return err
	}
	// generate PNG
	bufferPNG := bytes.NewBuffer([]byte{})
	if err := pie.Render(chart.PNG, bufferPNG); err != nil {
		return err
	}
	return ioutil.WriteFile(filename+pngExt, bufferPNG.Bytes(), 0644)
}

func writeTimeSeriesChart(series chart.TimeSeries, axisLabel, filename string, addSMA bool) error {
	plottedSeries := []chart.Series{series}
	if addSMA {
		sma := &chart.SMASeries{
			Style: chart.Style{
				Show:            true,
				StrokeColor:     drawing.ColorFromHex("bc5100"),
				StrokeDashArray: []float64{5.0, 5.0},
			},
			InnerSeries: series,
		}
		plottedSeries = append(plottedSeries, sma)
	}
	graph := chart.Chart{
		Height: 1000,
		Width:  2000,
		XAxis:  timeAxis,
		YAxis: chart.YAxis{
			Style:     chart.StyleShow(),
			Name:      axisLabel,
			NameStyle: chart.StyleShow(),
		},
		Series: plottedSeries,
	}
	// generate PNG
	bufferPNG := bytes.NewBuffer([]byte{})
	if err := graph.Render(chart.PNG, bufferPNG); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filename+svgExt, bufferPNG.Bytes(), 0644); err != nil {
		return err
	}

	// changing styles for SVG
	graph.XAxis = timeAxisSVG
	graph.YAxis.Style = chart.Style{
		Show:        true,
		FontColor:   chart.ColorWhite,
		StrokeColor: chart.ColorWhite,
	}
	graph.YAxis.NameStyle = chart.Style{
		Show:      true,
		FontColor: chart.ColorWhite,
	}
	series.Style = commonStyleSVG
	graph.Series[0] = series
	graph.Background = chart.Style{
		StrokeWidth: 0,
		StrokeColor: drawing.ColorBlue.WithAlpha(0),
		Padding:     chart.Box{0, 0, 0, 0},
		FillColor:   drawing.ColorBlue.WithAlpha(0),
		FontColor:   chart.ColorWhite,
	}
	graph.Canvas = chart.Style{
		StrokeWidth: 0,
		StrokeColor: drawing.ColorBlue.WithAlpha(0),
		Padding:     chart.Box{0, 0, 0, 0},
		FillColor:   drawing.ColorBlue.WithAlpha(0),
		FontColor:   chart.ColorWhite,
	}
	// generate SVG
	bufferSVG := bytes.NewBuffer([]byte{})
	if err := graph.Render(chart.SVG, bufferSVG); err != nil {
		return err
	}
	return ioutil.WriteFile(filename+svgExt, bufferSVG.Bytes(), 0644)
}

func combineAllPNGs(combined string, graphs ...string) error {
	images := []image.Image{}
	// open and decode images
	for _, graph := range graphs {
		imgFile, err := os.Open(graph + pngExt)
		if err != nil {
			logThis.Error(errors.Wrap(err, errorImageNotFound), NORMAL)
			continue
		}
		img, _, err := image.Decode(imgFile)
		if err != nil {
			logThis.Error(errors.Wrap(err, errorImageNotFound), NORMAL)
			continue
		}
		images = append(images, img)
	}
	if len(images) == 0 {
		return errors.New(errorNoImageFound)
	}

	// ----------------
	// |  1    | 2    |
	// ----------------
	// |  3    | 4    |
	// ----------------
	// |  ...  | ...  |
	// ----------------
	// |  n    | n+1  |
	// ----------------
	// |  n+2  | n+3  |
	// ----------------

	maxX := 0
	maxY := 0
	tempMaxX := 0
	tempMaxY := 0
	// max size of combined graph:
	// max X = max (firstColumn.X + secondColumn.X)
	// max Y = sum (max(firstColumn.Y, secondColumn.Y))
	for i, img := range images {
		if i%2 == 0 {
			// first column
			tempMaxY = img.Bounds().Dy()
			tempMaxX = img.Bounds().Dx()
			if i == len(images)-1 {
				// if we're on the last row and this is the last image
				maxY += tempMaxY
				if tempMaxX > maxX {
					maxX = tempMaxX
				}
			}
		} else {
			// second column
			tempMaxX += img.Bounds().Dx()
			if tempMaxX > maxX {
				maxX = tempMaxX
			}
			if img.Bounds().Dy() > tempMaxY {
				maxY += img.Bounds().Dy()
			} else {
				maxY += tempMaxY
			}
		}
	}
	//rectangle for the big image
	r := image.Rectangle{image.Point{0, 0}, image.Point{maxX, maxY}}
	// new image
	rgba := image.NewRGBA(r)

	currentX := 0
	currentY := 0
	currentRowHeight := 0
	for i, img := range images {
		if i%2 == 0 {
			// first column
			currentX = 0
			sp := image.Point{currentX, currentY}
			draw.Draw(rgba, image.Rectangle{sp, sp.Add(img.Bounds().Size())}, img, image.Point{0, 0}, draw.Src)
			currentX = img.Bounds().Dx()
			currentRowHeight = img.Bounds().Dy()

		} else {
			// second column
			sp := image.Point{currentX, currentY}
			draw.Draw(rgba, image.Rectangle{sp, sp.Add(img.Bounds().Size())}, img, image.Point{0, 0}, draw.Src)
			if img.Bounds().Dy() > currentRowHeight {
				currentRowHeight = img.Bounds().Dy()
			}
			currentY += currentRowHeight
		}
	}
	// save new image
	out, err := os.Create(combined + pngExt)
	if err != nil {
		return err
	}
	return png.Encode(out, rgba)
}
