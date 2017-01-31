package main

import (
	"fmt"
	"log"

	"github.com/gregdel/pushover"
)

func monitorStats(conf Config, tracker GazelleTracker, notification *pushover.Pushover, recipient *pushover.Recipient) {

	// TODO loop: select with ticker & done/stop chans

	stats, err := tracker.GetStats()
	if err != nil {
		fmt.Println(err.Error())
	} else {

		// TODO: if something is wrong, send notif and stop

		// send notification
		message := pushover.NewMessageWithTitle("Current stats: "+stats.Stats(), "varroa musica")
		_, err := notification.SendMessage(message, recipient)
		if err != nil {
			log.Println(err.Error())
		}
	}

	fmt.Println("End of monitor")
}
