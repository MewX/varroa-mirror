package main

import (
	"reflect"
	"time"

	"github.com/pkg/errors"
)

func manageStats(e *Environment, h *History, tracker *GazelleTracker, maxDecrease int) error {
	stats, err := tracker.GetStats()
	if err != nil {
		return errors.Wrap(err, errorGettingStats)
	}
	// get previous stats
	previousStats := &TrackerStats{}
	if len(h.TrackerStats) != 0 {
		previousStats = h.TrackerStats[len(h.TrackerStats)-1]
	}
	// compare with new stats
	logThis.Info(stats.Progress(previousStats), NORMAL)
	// send notification
	e.Notify("stats: "+stats.Progress(previousStats), tracker.Name, "info")
	// if something is wrong, send notification and stop
	if !stats.IsProgressAcceptable(previousStats, maxDecrease) {
		logThis.Info(errorBufferDrop, NORMAL)
		// sending notification
		e.Notify(errorBufferDrop, tracker.Name, "error")
		// stopping things
		e.config.disabledAutosnatching = true
	}
	// save to CSV
	if err := h.TrackerStatsHistory.Add(stats); err != nil {
		return errors.Wrap(err, errorWritingCSV)
	}
	// generate graphs
	return h.GenerateGraphs(e)
}

func updateStats(e *Environment, label string) error {
	statsConfig, err := e.config.GetStats(label)
	if err != nil {
		return errors.Wrap(err, "Error loading stats config for "+label)
	}
	tracker, err := e.Tracker(label)
	if err != nil {
		return errors.Wrap(err, "Error getting tracker info for "+label)
	}
	history, ok := e.History[label]
	if !ok {
		return errors.Wrap(err, "Error getting History for "+label)
	}
	return manageStats(e, history, tracker, statsConfig.MaxBufferDecreaseMB)
}

func monitorAllStats(e *Environment) {
	if !e.config.statsConfigured {
		return
	}
	// track all different periods
	tickers := map[int][]string{}
	for label, t := range e.Trackers {
		if statsConfig, err := e.config.GetStats(t.Name); err == nil {
			// initial stats
			if err := updateStats(e, label); err != nil {
				logThis.Error(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
			}
			// get update period
			tickers[statsConfig.UpdatePeriodH] = append(tickers[statsConfig.UpdatePeriodH], label)
		}
	}
	// generate index.html
	if err := e.GenerateIndex(); err != nil {
		logThis.Error(errors.Wrap(err, "Error generating index.html"), NORMAL)
	}
	// deploy
	if err := e.DeployToGitlabPages(); err != nil {
		logThis.Error(errors.Wrap(err, "Error deploying to Gitlab Pages"), NORMAL)
	}

	// preparing
	tickerChans := []<-chan time.Time{}
	tickerPeriods := []int{}
	for p := range tickers {
		tickerChans = append(tickerChans, time.NewTicker(time.Hour*time.Duration(p)).C)
		tickerPeriods = append(tickerPeriods, p)
	}
	cases := make([]reflect.SelectCase, len(tickerChans))
	for i, ch := range tickerChans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	// wait for ticks
	for {
		triggered, _, ok := reflect.Select(cases)
		if !ok {
			// The triggered channel has been closed, so zero out the channel to disable the case
			cases[triggered].Chan = reflect.ValueOf(nil)
			continue
		}
		// TODO checks
		for _, trackerLabel := range tickers[tickerPeriods[triggered]] {
			if err := updateStats(e, trackerLabel); err != nil {
				logThis.Error(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
			}
		}
		// generate index.html
		if err := e.GenerateIndex(); err != nil {
			logThis.Error(errors.Wrap(err, "Error generating index.html"), NORMAL)
		}
		// deploy
		if err := e.DeployToGitlabPages(); err != nil {
			logThis.Error(errors.Wrap(err, "Error deploying to Gitlab Pages"), NORMAL)
		}
	}
}
