package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	errorGettingStats        = "Error getting stats: "
	errorWritingCSV          = "Error writing stats to CSV file: "
	errorGeneratingGraphs    = "Error generating graphs: "
	errorNotEnoughDataPoints = "Not enough data points (yet) to generate graph"
	errorBufferDrop          = "Buffer drop too important, varroa will shutdown"
)

func addStatsToCSV(filename string, stats []string) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(stats); err != nil {
		return err
	}
	w.Flush()
	return nil
}

func manageStats(tracker GazelleTracker, previousStats *Stats) *Stats {
	stats, err := tracker.GetStats()
	if err != nil {
		logThis(errorGettingStats+err.Error(), NORMAL)
		return &Stats{}
	} else {
		logThis(stats.Progress(previousStats), NORMAL)
		// save to CSV
		timestamp := time.Now().Unix()
		newStats := []string{fmt.Sprintf("%d", timestamp), strconv.FormatUint(stats.Up, 10), strconv.FormatUint(stats.Down, 10), strconv.FormatFloat(stats.Ratio, 'f', -1, 64), strconv.FormatUint(stats.Buffer, 10), strconv.FormatUint(stats.WarningBuffer, 10)}
		if err := addStatsToCSV(statsFile, newStats); err != nil {
			logThis(errorWritingCSV+err.Error(), NORMAL)
		}
		// generate graphs
		if err := generateGraph(); err != nil {
			logThis(errorGeneratingGraphs+err.Error(), NORMAL)
		}
		// send notification
		if err := notification.Send("Current stats: " + stats.Progress(previousStats)); err != nil {
			logThis(errorNotification+err.Error(), VERBOSE)
		}
		// if something is wrong, send notification and stop
		if !stats.IsProgressAcceptable(previousStats, conf.maxBufferDecreaseByPeriodMB) {
			logThis(errorBufferDrop, NORMAL)
			// sending notification
			if err := notification.Send(errorBufferDrop); err != nil {
				logThis(errorNotification+err.Error(), VERBOSE)
			}
			// stopping things
			killDaemon()
		}
	}
	return stats
}

func monitorStats(tracker GazelleTracker) {
	// initial stats
	previousStats := &Stats{}
	previousStats = manageStats(tracker, previousStats)
	// periodic check
	period := time.NewTicker(time.Hour * time.Duration(conf.statsUpdatePeriod)).C
	for {
		select {
		case <-period:
			previousStats = manageStats(tracker, previousStats)
		case <-done:
			return
		case <-stop:
			return
		}
	}
}
