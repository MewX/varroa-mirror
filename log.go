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

type logMessage struct {
	level   int
	message string
}

func logThis(msg string, level int) {
	if env.config.logLevel >= level {
		if env.expectedOutput {
			// only is daemon is up...
			if env.inDaemon {
				log.Println(msg)
				env.sendBackToCLI <- msg
			} else {
				fmt.Println(msg)
			}
		} else {
			log.Println(msg)
		}
		// write to socket
		if env.websocketOutput {
			env.sendToWebsocket <- msg
		}
	}
}
