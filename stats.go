package main

import (
	"time"
)

const (
	errorGettingStats          = "Error getting stats: "
	errorWritingCSV            = "Error writing stats to CSV file: "
	errorGeneratingGraphs      = "Error generating graphs (may require more data): "
	errorGeneratingDailyGraphs = "Error generating daily graphs (at least 24h worth of data required): "
	errorNotEnoughDataPoints   = "Not enough data points (yet) to generate graph"
	errorBufferDrop            = "Buffer drop too important, stopping autosnatching. Reload to start again."
)

func manageStats(config *Config, tracker *GazelleTracker, previousStats *TrackerStats) *TrackerStats {
	stats, err := tracker.GetStats()
	if err != nil {
		logThis(errorGettingStats+err.Error(), NORMAL)
		return &TrackerStats{}
	}
	logThis(stats.Progress(previousStats), NORMAL)
	// save to CSV
	if err := env.history.TrackerStatsHistory.Add(stats); err != nil {
		logThis(errorWritingCSV+err.Error(), NORMAL)
	}
	// generate graphs
	if err := env.history.GenerateGraphs(); err != nil {
		logThis(errorGeneratingGraphs+err.Error(), NORMAL)
	}
	// send notification
	if err := env.notification.Send("Current stats: " + stats.Progress(previousStats)); err != nil {
		logThis(errorNotification+err.Error(), VERBOSE)
	}
	// if something is wrong, send notification and stop
	if !stats.IsProgressAcceptable(previousStats, config.maxBufferDecreaseByPeriodMB) {
		logThis(errorBufferDrop, NORMAL)
		// sending notification
		if err := env.notification.Send(errorBufferDrop); err != nil {
			logThis(errorNotification+err.Error(), VERBOSE)
		}
		// stopping things
		config.disabledAutosnatching = true
	}
	return stats
}

func monitorStats(config *Config, tracker *GazelleTracker) {
	// initial stats
	previousStats := &TrackerStats{}
	previousStats = manageStats(config, tracker, previousStats)
	// periodic check
	period := time.NewTicker(time.Hour * time.Duration(config.statsUpdatePeriod)).C
	for {
		<-period
		previousStats = manageStats(config, tracker, previousStats)
	}
}
