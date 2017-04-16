package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gregdel/pushover"
	daemon "github.com/sevlyar/go-daemon"
)

const (
	envPassphrase = "_VARROA_PASSPHRASE"

	errorPassphraseNotFound = "Error retrieving passphrase for daemon"
)

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
	env.config.disabledAutosnatching = false

	// is only true if we're in the daemon
	env.inDaemon = false
	env.configPassphrase = make([]byte, 32)

	env.limiter = make(chan bool, allowedAPICallsByPeriod)

	// current command expects output
	env.expectedOutput = false
	if !daemon.WasReborn() {
		// here we're expecting output
		env.expectedOutput = true
	}
	// websocket is open and waiting for input
	env.websocketOutput = false
	env.sendBackToCLI = make(chan string, 10)
	env.sendToWebsocket = make(chan string, 10)
	return env
}

// Daemonize the process and return true if in child process.
func (e *Environment) Daemonize(args []string) error {
	e.inDaemon = false
	e.daemon.Args = os.Args
	child, err := e.daemon.Reborn()
	if err != nil {
		logThis(errorGettingDaemonContext+err.Error(), NORMAL)
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
		logThis(errorServingSignals+err.Error(), NORMAL)
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
		logThis(errorSendingSignal+err.Error(), NORMAL)
	}
	if err := e.daemon.Release(); err != nil {
		logThis(errorReleasingDaemon+err.Error(), NORMAL)
	}
	if err := os.Remove(pidFile); err != nil {
		logThis(errorRemovingPID+err.Error(), NORMAL)
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
			return errors.New(errorGettingPassphrase + err.Error())
		}
		// saving to env for the daemon to pick up later
		if err := os.Setenv(envPassphrase, passphrase); err != nil {
			return errors.New(errorSettingEnv + err.Error())
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
		if err := newConf.loadFromBytes(configBytes); err != nil {
			logThis(errorLoadingConfig+err.Error(), NORMAL)
			return err
		}
	} else {
		if err := newConf.load(defaultConfigurationFile); err != nil {
			logThis(errorLoadingConfig+err.Error(), NORMAL)
			return err
		}
	}
	if e.config.user != "" {
		// if conf.user exists, the configuration had been loaded previously
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
			return errors.New(errorCreatingStatsDir + err.Error())
		}
	}
	// init notifications with pushover
	if e.config.pushoverConfigured() {
		e.notification.client = pushover.New(e.config.pushover.token)
		e.notification.recipient = pushover.NewRecipient(e.config.pushover.user)
	}
	// log in tracker
	e.tracker = &GazelleTracker{rootURL: e.config.url}
	if err := e.tracker.Login(e.config.user, e.config.password); err != nil {
		return err
	}
	logThis("Logged in tracker.", NORMAL)
	// load history
	return e.history.LoadAll(statsFile, historyFile)
}

// Reload the configuration file, restart autosnatching, and try to restart the web server
func (e *Environment) Reload() error {
	if err := env.LoadConfiguration(); err != nil {
		return errors.New(errorLoadingConfig + err.Error())
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
			logThis(errorShuttingDownServer+err.Error(), NORMAL)
			thingsWentOK = false
		}
	}
	if e.serverHTTPS.Addr != "" {
		serverWasUp = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.serverHTTPS.Shutdown(ctx); err != nil {
			logThis(errorShuttingDownServer+err.Error(), NORMAL)
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
		if e.config.pushoverConfigured() {
			if err := env.notification.Send(msg); err != nil {
				logThis(errorNotification+err.Error(), VERBOSE)
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
