package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gregdel/pushover"
	"github.com/wcharczuk/go-chart"
)

func sliceByteToGigabyte(in []float64) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = v / (1024 * 1024 * 1024)
	}
	return out
}

func generateGraph(conf Config) error {
	// prepare directory for pngs if necessary
	if !DirectoryExists("stats") {
		os.MkdirAll("stats", 0777)
	}

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
	warningBuffers := []float64{}
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

		warningBuffer, err := strconv.ParseUint(stats[5], 10, 64)
		if err != nil {
			continue // bad line
		}
		warningBuffers = append(warningBuffers, float64(warningBuffer))

		ratio, err := strconv.ParseFloat(stats[3], 64)
		if err != nil {
			continue // bad line
		}
		ratios = append(ratios, ratio)
	}

	upSeries := chart.TimeSeries{
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorBlue,
			FillColor:   chart.ColorBlue.WithAlpha(25),
		},
		Name:    "Upload",
		XValues: timestamps,
		YValues: sliceByteToGigabyte(ups),
	}
	downSeries := chart.TimeSeries{
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorBlue,
			FillColor:   chart.ColorBlue.WithAlpha(25),
		},
		Name:    "Download",
		XValues: timestamps,
		YValues: sliceByteToGigabyte(downs),
	}
	bufferSeries := chart.TimeSeries{
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorBlue,
			FillColor:   chart.ColorBlue.WithAlpha(25),
		},
		Name:    "Buffer",
		XValues: timestamps,
		YValues: sliceByteToGigabyte(buffers),
	}
	/*warningBufferSeries := chart.TimeSeries{
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorBlue,
			FillColor:   chart.ColorBlue.WithAlpha(25),
		},
		Name:    "Warning Buffer",
		XValues: timestamps,
		YValues: sliceByteToGigabyte(warningBuffers),
	}*/
	ratioSeries := chart.TimeSeries{
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorBlue,
			FillColor:   chart.ColorBlue.WithAlpha(25),
		},
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

	graphUp := chart.Chart{
		XAxis: xAxis,
		YAxis: chart.YAxis{
			Style:     chart.StyleShow(),
			Name:      "Upload (Gb)",
			NameStyle: chart.StyleShow(),
		},
		Series: []chart.Series{
			upSeries,
			chart.LastValueAnnotation(upSeries),
		},
	}
	bufferUp := bytes.NewBuffer([]byte{})
	if err := graphUp.Render(chart.PNG, bufferUp); err != nil {
		return err
	}
	if err := ioutil.WriteFile("stats/up.png", bufferUp.Bytes(), 0644); err != nil {
		return err
	}

	graphDown := chart.Chart{
		XAxis: xAxis,
		YAxis: chart.YAxis{
			Style:     chart.StyleShow(),
			Name:      "Download (Gb)",
			NameStyle: chart.StyleShow(),
		},
		Series: []chart.Series{
			downSeries,
			chart.LastValueAnnotation(downSeries),
		},
	}
	bufferDown := bytes.NewBuffer([]byte{})
	if err := graphDown.Render(chart.PNG, bufferDown); err != nil {
		return err
	}
	if err := ioutil.WriteFile("stats/down.png", bufferDown.Bytes(), 0644); err != nil {
		return err
	}

	graphBuffer := chart.Chart{
		XAxis: xAxis,
		YAxis: chart.YAxis{
			Style:     chart.StyleShow(),
			Name:      "Buffer (Gb)",
			NameStyle: chart.StyleShow(),
		},
		Series: []chart.Series{
			bufferSeries,
			chart.LastValueAnnotation(bufferSeries),
			//warningBufferSeries,
			//chart.LastValueAnnotation(warningBufferSeries),
		},
	}
	bufferBuffer := bytes.NewBuffer([]byte{})
	if err := graphBuffer.Render(chart.PNG, bufferBuffer); err != nil {
		return err
	}
	if err := ioutil.WriteFile("stats/buffer.png", bufferBuffer.Bytes(), 0644); err != nil {
		return err
	}

	graphRatio := chart.Chart{
		XAxis: xAxis,
		YAxis: chart.YAxis{
			Style:     chart.StyleShow(),
			Name:      "Ratio",
			NameStyle: chart.StyleShow(),
		},
		Series: []chart.Series{
			ratioSeries,
			chart.LastValueAnnotation(ratioSeries),
		},
	}
	bufferRatio := bytes.NewBuffer([]byte{})
	if err := graphRatio.Render(chart.PNG, bufferRatio); err != nil {
		return err
	}
	if err := ioutil.WriteFile("stats/ratio.png", bufferRatio.Bytes(), 0644); err != nil {
		return err
	}

	// open and decode images
	imgFile1, err1 := os.Open("stats/up.png")
	imgFile2, err2 := os.Open("stats/down.png")
	imgFile3, err3 := os.Open("stats/buffer.png")
	imgFile4, err4 := os.Open("stats/ratio.png")
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
	out, err := os.Create("stats/grid.png")
	if err != nil {
		fmt.Println(err)
	}
	return png.Encode(out, rgba)
}

func addStatsToCSV(filename string, stats []string) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write(stats); err != nil {
		return err
	}
	w.Flush()
	return nil
}

func getStats(conf Config, tracker GazelleTracker, previousStats *Stats, notification *pushover.Pushover, recipient *pushover.Recipient) *Stats {
	stats, err := tracker.GetStats()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		log.Println(stats.Progress(previousStats))
		// save to CSV
		timestamp := time.Now().Unix()
		newStats := []string{fmt.Sprintf("%d", timestamp), strconv.FormatUint(stats.Up, 10), strconv.FormatUint(stats.Down, 10), strconv.FormatFloat(stats.Ratio, 'f', -1, 64), strconv.FormatUint(stats.Buffer, 10), strconv.FormatUint(stats.WarningBuffer, 10)}
		if err := addStatsToCSV(conf.statsFile, newStats); err != nil {
			log.Println(err.Error())
		}
		// generate graphs
		if err := generateGraph(conf); err != nil {
			log.Println(err.Error())
		}
		// send notification
		message := pushover.NewMessageWithTitle("Current stats: "+stats.Progress(previousStats), "varroa musica")
		_, err := notification.SendMessage(message, recipient)
		if err != nil {
			log.Println(err.Error())
		}
		// if something is wrong, send notif and stop
		if !stats.IsProgressAcceptable(previousStats, conf) {
			log.Println("Drop in buffer too important, stopping autodl.")
			// sending notification
			message := pushover.NewMessageWithTitle("Drop in buffer too important, stopping autodl.", "varroa musica")
			_, err := notification.SendMessage(message, recipient)
			if err != nil {
				log.Println(err.Error())
			}
			// stopping things
			killDaemon()
		}
	}
	return stats
}

func monitorStats(conf Config, tracker GazelleTracker, notification *pushover.Pushover, recipient *pushover.Recipient) {
	// initial stats
	previousStats := &Stats{}
	previousStats = getStats(conf, tracker, previousStats, notification, recipient)
	// periodic check
	period := time.NewTicker(time.Hour * time.Duration(conf.statsUpdatePeriod)).C
	for {
		select {
		case <-period:
			log.Println("Getting stats...")
			previousStats = getStats(conf, tracker, previousStats, notification, recipient)
		case <-done:
			return
		case <-stop:
			return
		}
	}
}
