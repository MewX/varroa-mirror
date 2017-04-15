package main

import (
	"net/http"

	daemon "github.com/sevlyar/go-daemon"
)

type Environment struct {
	config                *Config
	configPassphrase      []byte
	daemon                *daemon.Context
	inDaemon              bool // <- == daemon.WasReborn()
	notification          *Notification
	history               *History
	serverHTTP            *http.Server
	serverHTTPS           *http.Server
	tracker               *GazelleTracker
	limiter               chan bool //  <- 1/tracker
	disabledAutosnatching bool
	expectedOutput        bool
	websocketOutput       bool
	sendBackToCLI         chan string
	sendToWebsocket       chan string
}

func NewEnvironment() *Environment {
	env := &Environment{}

	env.daemon = &daemon.Context{
		PidFileName: pidFile,
		PidFilePerm: 0644,
		LogFileName: "log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       0002,
	}
	env.config = &Config{}
	env.notification = &Notification{}
	env.history = &History{}
	env.serverHTTP = &http.Server{}
	env.serverHTTPS = &http.Server{}
	env.tracker = &GazelleTracker{}

	// disable  autosnatching
	env.disabledAutosnatching = false

	// is only true if we're in the daemon
	env.inDaemon = false
	env.configPassphrase = make([]byte, 32)

	env.limiter = make(chan bool, allowedAPICallsByPeriod)

	// current command expects output
	env.expectedOutput = false
	// websocket is open and waiting for input
	env.websocketOutput = false
	env.sendBackToCLI = make(chan string, 10)
	env.sendToWebsocket = make(chan string, 10)
	return env
}
