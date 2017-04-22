package main

import (
	"time"

	"github.com/pkg/errors"
)

func manageStats(e *Environment, h *History, tracker *GazelleTracker, previousStats *TrackerStats, maxDecrease int) *TrackerStats {
	stats, err := tracker.GetStats()
	if err != nil {
		logThis.Error(errors.Wrap(err, errorGettingStats), NORMAL)
		return &TrackerStats{}
	}
	logThis.Info(stats.Progress(previousStats), NORMAL)
	// save to CSV
	if err := h.TrackerStatsHistory.Add(stats); err != nil {
		logThis.Error(errors.Wrap(err, errorWritingCSV), NORMAL)
	}
	// generate graphs
	if err := h.GenerateGraphs(e); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
	}
	// send notification
	e.Notify("Current stats: " + stats.Progress(previousStats))
	// if something is wrong, send notification and stop
	if !stats.IsProgressAcceptable(previousStats, maxDecrease) {
		logThis.Info(errorBufferDrop, NORMAL)
		// sending notification
		e.Notify(errorBufferDrop)
		// stopping things
		e.config.disabledAutosnatching = true
	}
	return stats
}

func monitorStats(e *Environment, h *History, tracker *GazelleTracker) {
	statsConfig, err := e.config.GetStats(tracker.Name)
	if err != nil {
		logThis.Error(errors.Wrap(err, "Error loading stats config for "+tracker.Name), NORMAL)
		return
	}

	// initial stats
	previousStats := &TrackerStats{}
	previousStats = manageStats(e, h, tracker, previousStats, statsConfig.MaxBufferDecreaseMB)
	// periodic check
	period := time.NewTicker(time.Hour * time.Duration(statsConfig.UpdatePeriodH)).C
	for {
		<-period
		previousStats = manageStats(e, h, tracker, previousStats, statsConfig.MaxBufferDecreaseMB)
	}
}
