package varroa

import (
	"path/filepath"
	"reflect"
	"time"

	"github.com/asdine/storm"
	"github.com/pkg/errors"
)

func updateStats(e *Environment, tracker string, stats *StatsDB) error {
	// collect new stats for this tracker
	statsConfig, err := e.config.GetStats(tracker)
	if err != nil {
		return errors.Wrap(err, "Error loading stats config for "+tracker)
	}
	gazelleTracker, err := e.Tracker(tracker)
	if err != nil {
		return errors.Wrap(err, "Error getting tracker info for "+tracker)
	}
	newStats, err := gazelleTracker.GetStats()
	if err != nil {
		return errors.Wrap(err, errorGettingStats)
	}
	// save to database
	if err := stats.Save(newStats); err != nil {
		return errors.Wrap(err, "Error saving stats to database")
	}

	// get previous stats
	var previousStats *StatsEntry
	knownPreviousStats, err := stats.GetLastCollected(tracker, 1)
	if err != nil {
		if err != storm.ErrNotFound {
			previousStats = &StatsEntry{Collected: true}
		} else {
			return errors.Wrap(err, "Error retreiving previous stats for tracker "+tracker)
		}
	} else {
		previousStats = &knownPreviousStats[0]
	}

	// compare with new stats
	logThis.Info(newStats.Progress(previousStats), NORMAL)
	// send notification
	if err := Notify("stats: "+newStats.Progress(previousStats), tracker, "info"); err != nil {
		logThis.Error(err, NORMAL)
	}

	// if something is wrong, send notification and stop
	if !newStats.IsProgressAcceptable(previousStats, statsConfig.MaxBufferDecreaseMB, statsConfig.MinimumRatio) {
		if newStats.Ratio <= statsConfig.MinimumRatio {
			// unacceptable because of low ratio
			logThis.Info(tracker+": "+errorBelowWarningRatio, NORMAL)
			// sending notification
			if err := Notify(tracker+": "+errorBelowWarningRatio, tracker, "error"); err != nil {
				logThis.Error(err, NORMAL)
			}
		} else {
			// unacceptable because of ratio drop
			logThis.Info(tracker+": "+errorBufferDrop, NORMAL)
			// sending notification
			if err := Notify(tracker+": "+errorBufferDrop, tracker, "error"); err != nil {
				logThis.Error(err, NORMAL)
			}
		}
		// stopping things
		autosnatchConfig, err := e.config.GetAutosnatch(tracker)
		if err != nil {
			logThis.Error(errors.Wrap(err, "Cannot find autosnatch configuration for tracker "+tracker), NORMAL)
		} else {
			e.mutex.Lock()
			autosnatchConfig.disabledAutosnatching = true
			e.mutex.Unlock()
		}
	}

	// generate graphs
	return stats.GenerateAllGraphsForTracker(tracker)
}

func monitorAllStats(e *Environment) {
	if !e.config.statsConfigured {
		return
	}
	// access to statsDB
	stats, err := NewStatsDB(filepath.Join(StatsDir, DefaultHistoryDB))
	if err != nil {
		logThis.Error(errors.Wrap(err, "Error, could not access the stats database"), NORMAL)
		return
	}

	// track all different periods
	tickers := map[int][]string{}
	for label, t := range e.Trackers {
		if statsConfig, err := e.config.GetStats(t.Name); err == nil {
			// initial stats
			if err := updateStats(e, label, stats); err != nil {
				logThis.Error(errors.Wrap(err, ErrorGeneratingGraphs), NORMAL)
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
		logThis.Error(errors.Wrap(err, errorDeploying), NORMAL)
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
			if err := updateStats(e, trackerLabel, stats); err != nil {
				logThis.Error(errors.Wrap(err, ErrorGeneratingGraphs), NORMAL)
			}
		}
		// generate index.html
		if err := e.GenerateIndex(); err != nil {
			logThis.Error(errors.Wrap(err, "Error generating index.html"), NORMAL)
		}
		// deploy
		if err := e.DeployToGitlabPages(); err != nil {
			logThis.Error(errors.Wrap(err, errorDeploying), NORMAL)
		}
	}
}
