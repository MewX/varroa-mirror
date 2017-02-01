package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gregdel/pushover"
)

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
	tickChan := time.NewTicker(time.Minute*time.Duration(conf.statsUpdatePeriod)).C
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
