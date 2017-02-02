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
	buffers := []float64{}
	ratios := []float64{}
	for _, stats := range records {
		timestamp, err := strconv.ParseInt(stats[0], 10, 64)
		if err != nil {
			continue // bad line
		}
		timestamps = append(timestamps, time.Unix(timestamp, 0))

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

	bufferSeries := chart.TimeSeries{
		Name:    "Buffer",
		XValues: timestamps,
		YValues: buffers,
	}
	ratioSeries := chart.TimeSeries{
		YAxis:   chart.YAxisSecondary,
		Name:    "Ratio",
		XValues: timestamps,
		YValues: ratios,
	}

	// TODO: generate separate graphs or several curves on same graph?

	graph := chart.Chart{
		XAxis: chart.XAxis{
			Style: chart.Style{
				Show: true,
			},
			Name:           "Time",
			NameStyle: chart.StyleShow(),
			ValueFormatter: chart.TimeHourValueFormatter,
		},
		YAxis: chart.YAxis{
			Style: chart.Style{
				Show: true,
			},
			Name: "Size (bytes)",
			NameStyle: chart.StyleShow(),
		},
		YAxisSecondary: chart.YAxis{
			Style: chart.Style{
				Show: true, //enables / displays the secondary y-axis
			},
			Name: "Ratio",
			NameStyle: chart.StyleShow(),
		},
		Series: []chart.Series{
			bufferSeries,
			ratioSeries,
		},
		Background: chart.Style{
			Padding: chart.Box{
				Top:  20,
				Left: 20,
			},
		},
	}

	//legend
	graph.Elements = []chart.Renderable{
		chart.Legend(&graph),
	}

	buffer := bytes.NewBuffer([]byte{})
	if err := graph.Render(chart.PNG, buffer); err != nil {
		return err
	}

	return ioutil.WriteFile("test.png", buffer.Bytes(), 0644)
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
