package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gregdel/pushover"
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

func (e *Environment) SetUp() error {
	// load configuration
	if err := e.Reload(); err != nil {
		return errors.New(errorLoadingConfig + err.Error())
	}
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

func (e *Environment) Reload() error {
	newConf := &Config{}

	// if using encrypted file
	encryptedConfigurationFile := strings.TrimSuffix(defaultConfigurationFile, yamlExt) + encryptedExt
	if FileExists(encryptedConfigurationFile) && !FileExists(defaultConfigurationFile) {
		// if this env variable is set, we're using the encrypted config file and already have the passphrase
		if !e.inDaemon && os.Getenv(envPassphrase) == "" {
			// getting passphrase from user
			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}
			copy(e.configPassphrase[:], passphrase)
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
	}
	e.config = newConf
	if e.disabledAutosnatching {
		e.disabledAutosnatching = false
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
		go webServer()
	}
	return nil
}

func (e *Environment) SavePassphraseForDaemon() error {
	encryptedConfigurationFile := strings.TrimSuffix(defaultConfigurationFile, yamlExt) + encryptedExt
	if !daemon.WasReborn() {
		// if necessary, ask for passphrase and add to env
		if !FileExists(defaultConfigurationFile) && FileExists(encryptedConfigurationFile) {
			stringPass, err := getPassphrase()
			if err != nil {
				return errors.New(errorGettingPassphrase + err.Error())
			}
			// testing
			copy(e.configPassphrase[:], stringPass)
			configBytes, err := decrypt(encryptedConfigurationFile, e.configPassphrase)
			if err != nil {
				return errors.New(errorLoadingConfig + err.Error())
			}
			newConf := &Config{}
			if err := newConf.loadFromBytes(configBytes); err != nil {
				return errors.New(errorLoadingConfig + err.Error())

			}
			// saving to env for the daemon to pick up later
			if err := os.Setenv(envPassphrase, stringPass); err != nil {
				return errors.New(errorSettingEnv + err.Error())

			}
		}
	} else {
		// getting passphrase from env if necessary
		if !FileExists(defaultConfigurationFile) && FileExists(encryptedConfigurationFile) {
			passphrase := os.Getenv(envPassphrase)
			copy(e.configPassphrase[:], passphrase)
		}
	}
	return nil
}
