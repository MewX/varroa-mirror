package main

import (
	"fmt"
	"log"
)

const (
	NORMAL = iota
	VERBOSE
	VERBOSEST
	VERBOSESTEST
)

type LogThis struct {
	env *Environment
}

func (l *LogThis) Error(err error, level int) {
	l.Info(err.Error(), level)
}

func (l *LogThis) Info(msg string, level int) {
	if l.env.config.General == nil {
		// configuration was not loaded, printing error message
		fmt.Println(msg)
		return
	}
	if l.env.config.General.LogLevel >= level {
		if l.env.expectedOutput {
			// only is daemon is up...
			if l.env.inDaemon {
				log.Println(msg)
				l.env.sendBackToCLI <- msg
			} else {
				fmt.Println(msg)
			}
		} else {
			log.Println(msg)
		}
		// write to socket
		if l.env.websocketOutput {
			l.env.sendToWebsocket <- msg
		}
	}
}
