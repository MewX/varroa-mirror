package varroa

import (
	"fmt"
	"log"
	"sync"

	"github.com/sevlyar/go-daemon"
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

var logThis *LogThis
var onceLog sync.Once

func NewLogThis(e *Environment) *LogThis {
	onceLog.Do(func() {
		logThis = &LogThis{env: e}
	})
	return logThis
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
			if daemon.WasReborn() {
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
