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

func logThisError(err error, level int) {
	logThis(err.Error(), level)
}

func logThis(msg string, level int) {
	if env.config.General == nil {
		// configuration was not loaded, printing error message
		fmt.Println(msg)
		return
	}
	if env.config.General.LogLevel >= level {
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
