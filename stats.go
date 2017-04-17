package main

import (
	"time"

	"github.com/pkg/errors"
)

func manageStats(config *Config, tracker *GazelleTracker, previousStats *TrackerStats) *TrackerStats {
	stats, err := tracker.GetStats()
	if err != nil {
		logThisError(errors.Wrap(err, errorGettingStats), NORMAL)
		return &TrackerStats{}
	}
	logThis(stats.Progress(previousStats), NORMAL)
	// save to CSV
	if err := env.history.TrackerStatsHistory.Add(stats); err != nil {
		logThisError(errors.Wrap(err, errorWritingCSV), NORMAL)
	}
	// generate graphs
	if err := env.history.GenerateGraphs(); err != nil {
		logThisError(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
	}
	// send notification
	env.Notify("Current stats: " + stats.Progress(previousStats))
	// if something is wrong, send notification and stop
	if !stats.IsProgressAcceptable(previousStats, config.Stats[0].MaxBufferDecreaseMB) {
		logThis(errorBufferDrop, NORMAL)
		// sending notification
		env.Notify(errorBufferDrop)
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
	period := time.NewTicker(time.Hour * time.Duration(config.Stats[0].UpdatePeriodH)).C
	for {
		<-period
		previousStats = manageStats(config, tracker, previousStats)
	}
}
