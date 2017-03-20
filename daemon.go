package main

import (
	"flag"
	"os"
	"syscall"
	"time"

	"github.com/gregdel/pushover"
	daemon "github.com/sevlyar/go-daemon"
)

const (
	varroa = "varroa musica"

	// RED only allows 5 API calls every 10s
	allowedAPICallsByPeriod = 5
	apiCallsPeriodS         = 10

	errorKillingDaemon        = "Error killing running daemon"
	errorLoadingConfig        = "Error loading configuration: "
	errorServingSignals       = "Error serving signals: "
	errorSendingSignal        = "Error sending signal to the daemon: "
	errorGettingDaemonContext = "Error launching daemon: "
	errorCheckDaemonExited    = "Error checking daemon exited: "
	errorCreatingStatsDir     = "Error creating stats directory: "

)

var (
	signal = flag.String("s", "", `send orders to the daemon:
		reload — reload the configuration file
		stats  — generate graphs now
		quit   — graceful shutdown
		stop   — fast shutdown`)
	daemonContext = &daemon.Context{
		PidFileName: "pid",
		PidFilePerm: 0644,
		LogFileName: "log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       0002,
		Args:        []string{"[autosnatcher for your favorite tracker]"},
	}
	conf         = &Config{}
	notification = &Notification{}
	history      = &History{}

	// daemon control channels
	stop = make(chan struct{})
	done = make(chan struct{})

	// channel of allowedAPICallsByPeriod elements, which will rate-limit the requests
	limiter = make(chan bool, allowedAPICallsByPeriod)

	// disable  autosnatching
	disabledAutosnatching = false
)

func main() {
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, quitDaemon)
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, quitDaemon)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, loadConfiguration)
	daemon.AddCommand(daemon.StringFlag(signal, "stats"), syscall.SIGUSR1, generateStats)

	if len(daemon.ActiveFlags()) > 0 {
		d, err := daemonContext.Search()
		if err != nil {
			logThis(errorSendingSignal+err.Error(), NORMAL)
			return
		}
		daemon.SendCommands(d)
		return
	}
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
	if err := loadConfiguration(nil); err != nil {
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
		notification.client = pushover.New(conf.pushoverToken)
		notification.recipient = pushover.NewRecipient(conf.pushoverUser)
	}
	// log in tracker
	tracker := GazelleTracker{rootURL: conf.url}
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
	go checkSignals()
	go ircHandler(tracker)
	go monitorStats(tracker)
	go apiCallRateLimiter()

	if err := daemon.ServeSignals(); err != nil {
		logThis(errorServingSignals+err.Error(), NORMAL)
	}
	logThis("+ varroa musica stopped", NORMAL)
}

func checkSignals() {
	for {
		time.Sleep(time.Second)
		if _, ok := <-stop; ok {
			break
		}
	}
	done <- struct{}{}
}

func loadConfiguration(sig os.Signal) error {
	newConf := &Config{}
	if err := newConf.load("config.yaml"); err != nil {
		logThis(errorLoadingConfig+err.Error(), NORMAL)
		return err
	}
	conf = newConf
	logThis(" - Configuration reloaded.", NORMAL)
	disabledAutosnatching = false
	logThis(" - Autosnatching enabled.", NORMAL)
	return nil
}

func generateStats(sig os.Signal) error {
	if err := history.GenerateGraphs(); err != nil {
		logThis(errorGeneratingGraphs, NORMAL)
	}
	return nil
}

func quitDaemon(sig os.Signal) error {
	logThis("+ terminating", VERBOSE)
	stop <- struct{}{}
	if sig == syscall.SIGQUIT {
		<-done
	}
	return daemon.ErrStop
}

func killDaemon() {
	d, err := daemonContext.Search()
	if err != nil {
		logThis(errorSendingSignal, NORMAL)
	}
	if d != nil {
		if err := d.Signal(syscall.SIGTERM); err != nil {
			logThis(errorKillingDaemon+err.Error(), NORMAL)
			return
		}
		// Ascertain process has exited
		for {
			if err := d.Signal(syscall.Signal(0)); err != nil {
				if err.Error() == "os: process already finished" {
					break
				}
				logThis(errorCheckDaemonExited+err.Error(), NORMAL)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func apiCallRateLimiter() {
	// fill the rate limiter the first time
	for i := 0; i < allowedAPICallsByPeriod; i++ {
		limiter <- true
	}
	// every apiCallsPeriodS, refill the limiter channel
	for range time.Tick(time.Second * time.Duration(apiCallsPeriodS)) {
		for i := 0; i < allowedAPICallsByPeriod; i++ {
			select {
			case limiter <- true:
			default:
				// if channel is full, do nothing
				break
			}
		}
	}
}
