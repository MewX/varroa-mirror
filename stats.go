package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gregdel/pushover"
)


func getStats(tracker GazelleTracker, notification *pushover.Pushover, recipient *pushover.Recipient) {
	stats, err := tracker.GetStats()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		log.Println(stats.Stats())
		// TODO save to CSV?

		// TODO: if something is wrong, send notif and stop
		// send notification
		message := pushover.NewMessageWithTitle("Current stats: "+stats.Stats(), "varroa musica")
		_, err := notification.SendMessage(message, recipient)
		if err != nil {
			log.Println(err.Error())
		}
	}
}

func monitorStats(conf Config, tracker GazelleTracker, notification *pushover.Pushover, recipient *pushover.Recipient) {
	tickChan := time.NewTicker(time.Hour).C
	for {
		select {
		case <-tickChan:
			log.Println("Getting stats...")
			getStats(tracker, notification, recipient)
		case <-done:
			return
		case <-stop:
			return
		}
	}
}
