package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"image"
	"image/draw"
	"image/png"

	"github.com/gregdel/pushover"
	"github.com/wcharczuk/go-chart"
)

func generateGraph(conf Config, stats *Stats) error {
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
		Name:    "Upload",
		XValues: timestamps,
		YValues: ups,
	}
	downSeries := chart.TimeSeries{
		Name:    "Download",
		XValues: timestamps,
		YValues: downs,
	}
	bufferSeries := chart.TimeSeries{
		Name:    "Buffer",
		XValues: timestamps,
		YValues: buffers,
	}
	warningBufferSeries := chart.TimeSeries{
		Name:    "Warning Buffer",
		XValues: timestamps,
		YValues: warningBuffers,
	}
	ratioSeries := chart.TimeSeries{
		Style: chart.Style{
			Show:        true,
			StrokeColor: chart.ColorBlue,
			FillColor:   chart.ColorBlue.WithAlpha(50),
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

	// TODO: generate separate graphs or several curves on same graph?

	graph1 := chart.Chart{
		XAxis: xAxis,
		YAxis: chart.YAxis{
			Style: chart.Style{
				Show:        true,
				StrokeColor: chart.ColorBlue,
				FillColor:   chart.ColorBlue.WithAlpha(50),
			},
			Name:      "Size (bytes)",
			NameStyle: chart.StyleShow(),
		},
		Series: []chart.Series{
			upSeries,
			//chart.LastValueAnnotation(upSeries),
			downSeries,
			//chart.LastValueAnnotation(downSeries),
			bufferSeries,
			//chart.LastValueAnnotation(bufferSeries),
			warningBufferSeries,
			//chart.LastValueAnnotation(warningBufferSeries),

		},
		Background: chart.Style{
			Padding: chart.Box{
				Top:  20,
				Left: 20,
			},
		},
	}
	//legend
	graph1.Elements = []chart.Renderable{
		chart.Legend(&graph1),
	}
	buffer1 := bytes.NewBuffer([]byte{})
	if err := graph1.Render(chart.PNG, buffer1); err != nil {
		return err
	}
	if err := ioutil.WriteFile("stats.png", buffer1.Bytes(), 0644); err != nil {
		return err
	}

	graph2 := chart.Chart{
		XAxis: xAxis,
		YAxis: chart.YAxis{
			Style: chart.Style{
				Show:        true,
				StrokeColor: chart.ColorBlue,
				FillColor:   chart.ColorBlue.WithAlpha(50),
			},
			Name:      "Ratio",
			NameStyle: chart.StyleShow(),
		},
		Series: []chart.Series{
			ratioSeries,
			//chart.LastValueAnnotation(ratioSeries),
		},
		Background: chart.Style{
			Padding: chart.Box{
				Top:  20,
				Left: 20,
			},
		},
	}
	//legend
	graph2.Elements = []chart.Renderable{
		chart.Legend(&graph2),
	}
	buffer2 := bytes.NewBuffer([]byte{})
	if err := graph2.Render(chart.PNG, buffer2); err != nil {
		return err
	}
	if err := ioutil.WriteFile("ratios.png", buffer2.Bytes(), 0644); err != nil {
		return err
	}

	// open and decode images
	imgFile1, err := os.Open("ratios.png")
	if err != nil {
		fmt.Println(err)
	}
	imgFile2, err := os.Open("stats.png")
	if err != nil {
		fmt.Println(err)
	}

	img1, _, err := image.Decode(imgFile1)
	if err != nil {
		fmt.Println(err)
	}
	img2, _, err := image.Decode(imgFile2)
	if err != nil {
		fmt.Println(err)
	}

	// create the rectangle, assuming they all have the same size
	// ------------
	// |  1  | 2  |
	// ------------

	sp2 := image.Point{X: img1.Bounds().Dx(), Y: 0}
	r2 := image.Rectangle{Min: sp2, Max: sp2.Add(img2.Bounds().Size())}

	//rectangle for the big image
	r := image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: r2.Max}

	// new image
	rgba := image.NewRGBA(r)
	// draw original images in new rectangle
	draw.Draw(rgba, img1.Bounds(), img1, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, r2, img2, image.Point{0, 0}, draw.Src)

	// save new image
	out, err := os.Create("grid.png")
	if err != nil {
		fmt.Println(err)
	}
	png.Encode(out, rgba)

	return nil
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
		// save to CSV?
		// get timestampString
		timestamp := time.Now().Unix()
		newStats := []string{fmt.Sprintf("%d", timestamp), strconv.FormatUint(stats.Up, 10), strconv.FormatUint(stats.Down, 10), strconv.FormatFloat(stats.Ratio, 'f', -1, 64), strconv.FormatUint(stats.Buffer, 10), strconv.FormatUint(stats.WarningBuffer, 10)}
		if err := addStatsToCSV(conf.statsFile, newStats); err != nil {
			log.Println(err.Error())
		}

		// TODO graph!
		if err := generateGraph(conf, stats); err != nil {
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
	previousStats := &Stats{}
	//tickChan := time.NewTicker(time.Hour).C
	tickChan := time.NewTicker(time.Minute * time.Duration(conf.statsUpdatePeriod)).C
	for {
		select {
		case <-tickChan:
			log.Println("Getting stats...")
			previousStats = getStats(conf, tracker, previousStats, notification, recipient)
		case <-done:
			return
		case <-stop:
			return
		}
	}
}
