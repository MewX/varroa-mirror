package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gregdel/pushover"
	"github.com/pkg/errors"
	daemon "github.com/sevlyar/go-daemon"
)

type boolFlag bool

func (b boolFlag) IsSet() bool {
	return bool(b)
}

// Environment keeps track of all the context varroa needs.
type Environment struct {
	config           *Config
	configPassphrase []byte
	daemon           *daemon.Context
	inDaemon         bool // <- == daemon.WasReborn()
	notification     *Notification
	history          *History
	serverHTTP       *http.Server
	serverHTTPS      *http.Server
	tracker          *GazelleTracker
	limiter          chan bool //  <- 1/tracker

	expectedOutput  bool
	websocketOutput bool
	sendBackToCLI   chan string
	sendToWebsocket chan string
}

// NewEnvironment prepares a new Environment.
func NewEnvironment() *Environment {
	e := &Environment{}
	e.daemon = &daemon.Context{
		PidFileName: pidFile,
		PidFilePerm: 0644,
		LogFileName: "log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       0002,
	}
	e.config = &Config{}
	e.notification = &Notification{}
	e.history = &History{}
	e.serverHTTP = &http.Server{}
	e.serverHTTPS = &http.Server{}
	e.tracker = &GazelleTracker{}
	// disable  autosnatching
	e.config.disabledAutosnatching = false
	// is only true if we're in the daemon
	e.inDaemon = false
	e.configPassphrase = make([]byte, 32)
	e.limiter = make(chan bool, allowedAPICallsByPeriod)
	// current command expects output
	e.expectedOutput = false
	if !daemon.WasReborn() {
		// here we're expecting output
		e.expectedOutput = true
	}
	// websocket is open and waiting for input
	e.websocketOutput = false
	e.sendBackToCLI = make(chan string, 10)
	e.sendToWebsocket = make(chan string, 10)
	return e
}

// Daemonize the process and return true if in child process.
func (e *Environment) Daemonize(args []string) error {
	e.inDaemon = false
	e.daemon.Args = os.Args
	child, err := e.daemon.Reborn()
	if err != nil {
		return err
	}
	if child != nil {
		logThis("Starting daemon...", NORMAL)
	} else {
		logThis("+ varroa musica daemon started", NORMAL)
		// now in the daemon
		daemon.AddCommand(boolFlag(false), syscall.SIGTERM, quitDaemon)
		e.inDaemon = true
	}
	return nil
}

func quitDaemon(sig os.Signal) error {
	logThis("+ terminating", VERBOSE)
	return daemon.ErrStop
}

// Wait for the daemon to stop.
func (e *Environment) WaitForDaemonStop() {
	if err := daemon.ServeSignals(); err != nil {
		logThisError(errors.Wrap(err, errorServingSignals), NORMAL)
	}
	logThis("+ varroa musica stopped", NORMAL)
}

// FindDaemon if it is running.
func (e *Environment) FindDaemon() (*os.Process, error) {
	// trying to talk to existing daemon
	return e.daemon.Search()
}

// StopDaemon if running
func (e *Environment) StopDaemon(daemonProcess *os.Process) {
	daemon.AddCommand(boolFlag(true), syscall.SIGTERM, quitDaemon)
	if err := daemon.SendCommands(daemonProcess); err != nil {
		logThisError(errors.Wrap(err, errorSendingSignal), NORMAL)
	}
	if err := e.daemon.Release(); err != nil {
		logThisError(errors.Wrap(err, errorReleasingDaemon), NORMAL)
	}
	if err := os.Remove(pidFile); err != nil {
		logThisError(errors.Wrap(err, errorRemovingPID), NORMAL)
	}
}

// SavePassphraseForDaemon save the encrypted configuration file passphrase to env if necessary.
// In the daemon, retrieve that passphrase.
func (e *Environment) SavePassphraseForDaemon() error {
	var passphrase string
	var err error
	if !daemon.WasReborn() {
		// if necessary, ask for passphrase and add to env
		passphrase, err = getPassphrase()
		if err != nil {
			return errors.Wrap(err, errorGettingPassphrase)
		}
		// saving to env for the daemon to pick up later
		if err := os.Setenv(envPassphrase, passphrase); err != nil {
			return errors.Wrap(err, errorSettingEnv)
		}
	} else {
		// getting passphrase from env if necessary
		passphrase = os.Getenv(envPassphrase)
	}
	if passphrase == "" {
		return errors.New(errorPassphraseNotFound)
	}
	copy(e.configPassphrase[:], passphrase)
	return nil
}

// GetPassphrase and keep in Environment
func (e *Environment) GetPassphrase() error {
	passphrase, err := getPassphrase()
	if err != nil {
		return err
	}
	copy(env.configPassphrase[:], passphrase)
	return nil
}

// LoadConfiguration whether the configuration file is encrypted or not.
func (e *Environment) LoadConfiguration() error {
	newConf := &Config{}
	encryptedConfigurationFile := strings.TrimSuffix(defaultConfigurationFile, yamlExt) + encryptedExt
	if FileExists(encryptedConfigurationFile) && !FileExists(defaultConfigurationFile) {
		// if using encrypted config file, ask for the passphrase and retrieve it from the daemon side
		if err := e.SavePassphraseForDaemon(); err != nil {
			return err
		}
		configBytes, err := decrypt(encryptedConfigurationFile, e.configPassphrase)
		if err != nil {
			return err
		}
		if err := newConf.LoadFromBytes(configBytes); err != nil {
			return err
		}
	} else {
		if err := newConf.Load(defaultConfigurationFile); err != nil {
			return err
		}
	}
	if len(e.config.Trackers) != 0 {
		// if trackers are configured, the configuration had been loaded previously
		logThis("Configuration reloaded.", NORMAL)
	} else {
		logThis("Configuration loaded.", VERBOSE)
	}
	e.config = newConf
	return nil
}

// SetUp the Environment
func (e *Environment) SetUp() error {
	// prepare directory for stats if necessary
	if !DirectoryExists(statsDir) {
		if err := os.MkdirAll(statsDir, 0777); err != nil {
			return errors.Wrap(err, errorCreatingStatsDir)
		}
	}
	// init notifications with pushover
	if e.config.notificationsConfigured {
		e.notification.client = pushover.New(e.config.Notifications.Pushover.Token)
		e.notification.recipient = pushover.NewRecipient(e.config.Notifications.Pushover.User)
	}
	// log in tracker
	e.tracker = &GazelleTracker{URL: e.config.Trackers[0].URL}
	if err := e.tracker.Login(e.config.Trackers[0].User, e.config.Trackers[0].Password); err != nil {
		return err
	}
	logThis("Logged in tracker.", NORMAL)
	// load history
	return e.history.LoadAll(statsFile, historyFile)
}

// Reload the configuration file, restart autosnatching, and try to restart the web server
func (e *Environment) Reload() error {
	if err := env.LoadConfiguration(); err != nil {
		return errors.Wrap(err, errorLoadingConfig)
	}
	if e.config.disabledAutosnatching {
		e.config.disabledAutosnatching = false
		logThis("Autosnatching enabled.", NORMAL)
	}
	// if server up
	thingsWentOK := true
	serverWasUp := false
	if e.serverHTTP.Addr != "" {
		serverWasUp = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.serverHTTP.Shutdown(ctx); err != nil {
			logThisError(errors.Wrap(err, errorShuttingDownServer), NORMAL)
			thingsWentOK = false
		}
	}
	if e.serverHTTPS.Addr != "" {
		serverWasUp = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.serverHTTPS.Shutdown(ctx); err != nil {
			logThisError(errors.Wrap(err, errorShuttingDownServer), NORMAL)
			thingsWentOK = false
		}
	}
	if serverWasUp && thingsWentOK {
		// launch server again
		go webServer(e.config, e.serverHTTP, e.serverHTTPS)
	}
	return nil
}

// Notify in a goroutine, or directly.
func (e *Environment) Notify(msg string) error {
	notity := func() error {
		if e.config.notificationsConfigured {
			if err := env.notification.Send(msg); err != nil {
				logThisError(errors.Wrap(err, errorNotification), VERBOSE)
				return err
			}
		}
		return nil
	}
	return e.RunOrGo(notity)
}

// RunOrGo depending on whether we're in the daemon or not.
func (e *Environment) RunOrGo(f func() error) error {
	if e.inDaemon {
		go f()
		return nil
	}
	return f()
}
