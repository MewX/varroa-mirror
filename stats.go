package main

import (
	"time"
)

const (
	errorGettingStats        = "Error getting stats: "
	errorWritingCSV          = "Error writing stats to CSV file: "
	errorGeneratingGraphs    = "Error generating graphs (may require more data): "
	errorNotEnoughDataPoints = "Not enough data points (yet) to generate graph"
	errorBufferDrop          = "Buffer drop too important, stopping autosnatching. Reload to start again."
)

func manageStats(tracker GazelleTracker, previousStats *TrackerStats) *TrackerStats {
	stats, err := tracker.GetStats()
	if err != nil {
		logThis(errorGettingStats+err.Error(), NORMAL)
		return &TrackerStats{}
	} else {
		logThis(stats.Progress(previousStats), NORMAL)
		// save to CSV
		if err := history.TrackerStatsHistory.Add(stats); err != nil {
			logThis(errorWritingCSV+err.Error(), NORMAL)
		}
		// generate graphs
		if err := history.GenerateGraphs(); err != nil {
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
			disabledAutosnatching = true
			// killDaemon()
		}
	}
	return stats
}

func monitorStats(tracker GazelleTracker) {
	// initial stats
	previousStats := &TrackerStats{}
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
