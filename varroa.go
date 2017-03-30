package main

import (
	"net"
	"net/http"
	"os"
	"syscall"

	"github.com/gregdel/pushover"
	daemon "github.com/sevlyar/go-daemon"
)

const (
	varroa = "varroa musica"

	errorLoadingConfig        = "Error loading configuration: "
	errorServingSignals       = "Error serving signals: "
	errorFindingDaemon        = "Error finding daemon: "
	errorSendingSignal        = "Error sending signal to the daemon: "
	errorGettingDaemonContext = "Error launching daemon: "
	errorCreatingStatsDir     = "Error creating stats directory: "
	errorShuttingDownServer   = "Error shutting down web server: "
	errorArguments            = "Error parsing command line arguments: "
)

var (
	daemonContext = &daemon.Context{
		PidFileName: "pid",
		PidFilePerm: 0644,
		LogFileName: "log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       0002,
	}
	conf         = &Config{}
	notification = &Notification{}
	history      = &History{}
	server       = &http.Server{}
	tracker      = &GazelleTracker{}

	// disable  autosnatching
	disabledAutosnatching = false
)

type boolFlag bool

func (b boolFlag) IsSet() bool {
	return bool(b)
}

func main() {
	// parsing CLI
	cli := &varroaArguments{}
	if err := cli.parseCLI(os.Args[1:]); err != nil {
		logThis(errorArguments+err.Error(), NORMAL)
		return
	}
	if cli.builtin {
		return
	}

	// launching daemon
	if cli.start {
		daemonContext.Args = os.Args
		d, err := daemonContext.Reborn()
		if err != nil {
			logThis(errorGettingDaemonContext+err.Error(), NORMAL)
			return
		}
		if d != nil {
			return
		}
		defer daemonContext.Release()

		logThis("+ varroa musica started", NORMAL)
		// load configuration
		if err := loadConfiguration(); err != nil {
			logThis(err.Error(), NORMAL)
			return
		}
		// prepare directory for stats if necessary
		if !DirectoryExists(statsDir) {
			if err := os.MkdirAll(statsDir, 0777); err != nil {
				logThis(errorCreatingStatsDir+err.Error(), NORMAL)
				return
			}
		}
		// init notifications with pushover
		if conf.pushoverConfigured() {
			notification.client = pushover.New(conf.pushover.token)
			notification.recipient = pushover.NewRecipient(conf.pushover.user)
		}
		// log in tracker
		tracker = &GazelleTracker{rootURL: conf.url}
		if err := tracker.Login(conf.user, conf.password); err != nil {
			logThis(err.Error(), NORMAL)
			return
		}
		logThis(" - Logged in tracker.", NORMAL)
		// load history
		if err := history.LoadAll(statsFile, historyFile); err != nil {
			logThis(err.Error(), NORMAL)
		}

		// launch goroutines
		go ircHandler()
		go monitorStats()
		go apiCallRateLimiter()
		go webServer()
		go awaitOrders()

		if err := daemon.ServeSignals(); err != nil {
			logThis(errorServingSignals+err.Error(), NORMAL)
		}
		logThis("+ varroa musica stopped", NORMAL)
		return
	}

	// see if we want to send commands to the daemon through the unix socket
	sendCommand := false
	command := ""
	if cli.stats {
		sendCommand = true
		command = "stats"
	}
	if cli.reload {
		sendCommand = true
		command = "reload"
	}
	if cli.stop {
		// to cleanly close the unix socket
		sendCommand = true
		command = "stop"
	}
	if cli.refreshMetadata {
		sendCommand = true
		command = "refresh-metadata " + IntSliceToString(cli.torrentIDs)
	}
	if sendCommand {
		conn, err := net.Dial("unix", varroaSocket)
		if err != nil {
			logThis("Error dialing to unix socket: "+err.Error(), NORMAL)
			return
		}
		// sending command
		if _, err = conn.Write([]byte(command)); err != nil {
			logThis("Error writing to unix socket: "+err.Error(), NORMAL)
		}
		conn.Close()
	}

	// at last, sending signals for shutdown
	if cli.stop {
		daemon.AddCommand(boolFlag(cli.stop), syscall.SIGTERM, quitDaemon)
		// trying to talk to existing daemon
		daemonContext.Args = os.Args
		d, err := daemonContext.Search()
		if err != nil {
			logThis(errorFindingDaemon+err.Error(), NORMAL)
			return
		}
		if err := daemon.SendCommands(d); err != nil {
			logThis(errorSendingSignal+err.Error(), NORMAL)
		}
		return
	}
	return
}

func quitDaemon(sig os.Signal) error {
	logThis("+ terminating", VERBOSE)
	return daemon.ErrStop
}
