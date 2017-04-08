package main

import (
	"fmt"
	"log"
)

const (
	NORMAL = iota
	VERBOSE
	VERBOSEST
)

var (
	sendBackToCLI = make(chan string, 10)
	sendToWebsocket = make(chan string, 10)
)

type logMessage struct {
	level   int
	message string
}

func logThis(msg string, level int) {
	if conf.logLevel >= level {
		if expectedOutput {
			// only is daemon is up...
			if inDaemon {
				log.Println(msg)
				sendBackToCLI <- msg
			} else {
				fmt.Println(msg)
			}
		} else {
			log.Println(msg)
		}
		// write to socket
		if websocketOutput {
			sendToWebsocket <- msg
		}
	}
}
