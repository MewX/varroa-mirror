package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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
	serverHTTP       *http.Server
	serverHTTPS      *http.Server
	Trackers         map[string]*GazelleTracker
	History          map[string]*History

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
	e.serverHTTP = &http.Server{}
	e.serverHTTPS = &http.Server{}
	// make maps
	e.Trackers = make(map[string]*GazelleTracker)
	e.History = make(map[string]*History)
	// is only true if we're in the daemon
	e.inDaemon = false
	e.configPassphrase = make([]byte, 32)
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
		logThis.Info("Starting daemon...", NORMAL)
	} else {
		logThis.Info("+ varroa musica daemon started", NORMAL)
		// now in the daemon
		daemon.AddCommand(boolFlag(false), syscall.SIGTERM, quitDaemon)
		e.inDaemon = true
	}
	return nil
}

func quitDaemon(sig os.Signal) error {
	logThis.Info("+ terminating", VERBOSE)
	return daemon.ErrStop
}

// Wait for the daemon to stop.
func (e *Environment) WaitForDaemonStop() {
	if err := daemon.ServeSignals(); err != nil {
		logThis.Error(errors.Wrap(err, errorServingSignals), NORMAL)
	}
	logThis.Info("+ varroa musica stopped", NORMAL)
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
		logThis.Error(errors.Wrap(err, errorSendingSignal), NORMAL)
	}
	if err := e.daemon.Release(); err != nil {
		logThis.Error(errors.Wrap(err, errorReleasingDaemon), NORMAL)
	}
	if err := os.Remove(pidFile); err != nil {
		logThis.Error(errors.Wrap(err, errorRemovingPID), NORMAL)
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
		logThis.Info("Configuration reloaded.", NORMAL)
	} else {
		logThis.Info("Configuration loaded.", VERBOSE)
	}
	e.config = newConf
	return nil
}

// SetUp the Environment
func (e *Environment) SetUp(autologin bool) error {
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

	// log in all trackers, assuming labels are unique (configuration was checked)
	for _, label := range e.config.TrackerLabels() {
		config, err := e.config.GetTracker(label)
		if err != nil {
			return errors.Wrap(err, "Error getting tracker information")
		}
		tracker := &GazelleTracker{Name: config.Name, URL: config.URL, User: config.User, Password: config.Password, limiter: make(chan bool, allowedAPICallsByPeriod)}
		if autologin {
			if err := tracker.Login(); err != nil {
				return errors.Wrap(err, "Error logging in tracker "+label)
			}
			logThis.Info(fmt.Sprintf("Logged in tracker %s.", label), NORMAL)
		}
		// launching rate limiter
		go tracker.apiCallRateLimiter()
		e.Trackers[label] = tracker

		// load history for this tracker
		h := &History{Tracker: label}
		// load relevant history
		if err := h.LoadAll(); err != nil {
			return errors.Wrap(err, "Error loading history for tracker "+label)
		}
		e.History[label] = h
	}
	return nil
}

// Reload the configuration file, restart autosnatching, and try to restart the web server
func (e *Environment) Reload() error {
	if err := e.LoadConfiguration(); err != nil {
		return errors.Wrap(err, errorLoadingConfig)
	}
	if e.config.disabledAutosnatching {
		e.config.disabledAutosnatching = false
		logThis.Info("Autosnatching enabled.", NORMAL)
	}
	// if server up
	thingsWentOK := true
	serverWasUp := false
	if e.serverHTTP.Addr != "" {
		serverWasUp = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.serverHTTP.Shutdown(ctx); err != nil {
			logThis.Error(errors.Wrap(err, errorShuttingDownServer), NORMAL)
			thingsWentOK = false
		}
	}
	if e.serverHTTPS.Addr != "" {
		serverWasUp = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.serverHTTPS.Shutdown(ctx); err != nil {
			logThis.Error(errors.Wrap(err, errorShuttingDownServer), NORMAL)
			thingsWentOK = false
		}
	}
	if serverWasUp && thingsWentOK {
		// launch server again
		go webServer(e, e.serverHTTP, e.serverHTTPS)
	}
	return nil
}

// Notify in a goroutine, or directly.
func (e *Environment) Notify(msg string) error {
	notity := func() error {
		if e.config.notificationsConfigured {
			link := ""
			if e.config.gitlabPagesConfigured {
				link = e.config.GitlabPages.URL
			}
			if err := e.notification.Send(msg, e.config.gitlabPagesConfigured, link); err != nil {
				logThis.Error(errors.Wrap(err, errorNotification), VERBOSE)
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

func (e *Environment) Tracker(label string) (*GazelleTracker, error) {
	// find in already loaded trackers
	tracker, ok := e.Trackers[label]
	if !ok {
		// not found:
		config, err := e.config.GetTracker(label)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting tracker information")
		}
		tracker = &GazelleTracker{Name: config.Name, URL: config.URL, User: config.User, Password: config.Password}
		// saving
		e.Trackers[label] = tracker
	}
	if tracker.client == nil {
		if err := tracker.Login(); err != nil {
			return tracker, errors.Wrap(err, "Error logging in tracker "+label)
		}
		logThis.Info(fmt.Sprintf("Logged in tracker %s.", label), NORMAL)
	}
	return tracker, nil
}

func (e *Environment) GenerateIndex() error {
	if !e.config.statsConfigured {
		return nil
	}

	indexData := &HTMLIndex{Title: strings.ToUpper(varroa), Time: time.Now().Format("2006-01-02 15:04:05"), Version: version}
	for label, h := range e.History {
		indexData.CSV = append(indexData.CSV, HTMLLink{Name: label + ".csv", URL: filepath.Base(h.getPath(statsFile + csvExt))})

		statsNames := []struct {
			Name  string
			Label string
		}{
			{Name: "Buffer", Label: label + "_" + bufferStatsFile},
			{Name: "Upload", Label: label + "_" + uploadStatsFile},
			{Name: "Download", Label: label + "_" + downloadStatsFile},
			{Name: "Ratio", Label: label + "_" + ratioStatsFile},
			{Name: "Buffer/day", Label: label + "_" + bufferPerDayStatsFile},
			{Name: "Upload/day", Label: label + "_" + uploadPerDayStatsFile},
			{Name: "Download/day", Label: label + "_" + downloadPerDayStatsFile},
			{Name: "Ratio/day", Label: label + "_" + ratioPerDayStatsFile},
			{Name: "Snatches/day", Label: label + "_" + numberSnatchedPerDayFile},
			{Name: "Size Snatched/day", Label: label + "_" + sizeSnatchedPerDayFile},
		}
		// add graphs + links
		graphLinks := []HTMLLink{}
		graphs := []HTMLLink{}
		for _, s := range statsNames {
			graphLinks = append(graphLinks, HTMLLink{Name: s.Name, URL: "#" + s.Label})
			graphs = append(graphs, HTMLLink{Title: label + ": " + s.Name, Name: s.Label, URL: s.Label + svgExt})
		}
		stats := HTMLStats{Name: label, Stats: h.TrackerStats[len(h.TrackerStats)-1].String(), Graphs: graphs, GraphLinks: graphLinks}
		indexData.Stats = append(indexData.Stats, stats)
	}
	return indexData.ToHTML(filepath.Join(statsDir, htmlIndexFile))
}

// DeployToGitlabPages with git wrapper
func (e *Environment) DeployToGitlabPages() error {
	if !e.config.gitlabPagesConfigured {
		return nil
	}
	git := NewGit(statsDir, e.config.GitlabPages.User, e.config.GitlabPages.User+"+varroa@musica")
	if git == nil {
		return errors.New("Error setting up git")
	}
	// make sure we're going back to cwd
	defer git.getBack()

	// init repository if necessary
	if !git.Exists() {
		if err := git.Init(); err != nil {
			return errors.Wrap(err, errorGitInit)
		}
		// create .gitlab-ci.yml
		if err := ioutil.WriteFile(filepath.Join(statsDir, gitlabCIYamlFile), []byte(gitlabCI), 0666); err != nil {
			return err
		}
	}
	// add overall stats and other files
	if err := git.Add("*"+svgExt, "*"+csvExt, filepath.Base(gitlabCIYamlFile), filepath.Base(htmlIndexFile)); err != nil {
		return errors.Wrap(err, errorGitAdd)
	}
	// commit
	if err := git.Commit("varroa musica stats update."); err != nil {
		return errors.Wrap(err, errorGitCommit)
	}
	// push
	if !git.HasRemote("origin") {
		if err := git.AddRemote("origin", e.config.GitlabPages.GitHTTPS); err != nil {
			return errors.Wrap(err, errorGitAddRemote)
		}
	}
	if err := git.Push("origin", e.config.GitlabPages.GitHTTPS, e.config.GitlabPages.User, e.config.GitlabPages.Password); err != nil {
		return errors.Wrap(err, errorGitPush)
	}
	logThis.Info("Pushed new stats to "+e.config.GitlabPages.URL, NORMAL)
	return nil
}
