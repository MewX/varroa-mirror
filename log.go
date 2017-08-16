package varroa

import (
	"fmt"
	"log"
	"sync"
)

const (
	NORMAL = iota
	VERBOSE
	VERBOSEST
)

type LogThis struct {
	env *Environment
}

var logThis *LogThis
var once sync.Once

func NewLogThis(e *Environment) *LogThis {
	once.Do(func() {
		logThis = &LogThis{env: e}
	})
	return logThis
}

func (l *LogThis) Error(err error, level int) {
	l.Info(err.Error(), level)
}

func (l *LogThis) Info(msg string, level int) {
	if l.env.Config.General == nil {
		// configuration was not loaded, printing error message
		fmt.Println(msg)
		return
	}
	if l.env.Config.General.LogLevel >= level {
		if l.env.expectedOutput {
			// only is daemon is up...
			if l.env.InDaemon {
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
