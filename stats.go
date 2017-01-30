package main

import "fmt"

func monitorStats(conf Config, tracker GazelleTracker) {

	// TODO loop: select with ticker & done/stop chans

	if err := tracker.GetStats(); err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println("End of monitor")
}
