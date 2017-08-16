package varroa

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

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
	Config           *Config
	ConfigPassphrase []byte
	daemon           *daemon.Context
	InDaemon         bool // <- == daemon.WasReborn()
	notification     *Notification
	serverHTTP       *http.Server
	serverHTTPS      *http.Server
	serverData       *ServerData
	Trackers         map[string]*GazelleTracker
	History          map[string]*History
	Downloads        *Downloads

	graphsLastUpdated string
	expectedOutput    bool
	websocketOutput   bool
	sendBackToCLI     chan string
	sendToWebsocket   chan string
	mutex             sync.RWMutex
	git               *Git
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
	e.Config = &Config{}
	e.notification = &Notification{}
	e.serverHTTP = &http.Server{}
	e.serverHTTPS = &http.Server{}
	e.serverData = &ServerData{}
	// make maps
	e.Trackers = make(map[string]*GazelleTracker)
	e.History = make(map[string]*History)
	// is only true if we're in the daemon
	e.InDaemon = false
	e.ConfigPassphrase = make([]byte, 32)
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
	// default graphs update time
	e.graphsLastUpdated = "unknown"
	return e
}

// Daemonize the process and return true if in child process.
func (e *Environment) Daemonize(args []string) error {
	e.InDaemon = false
	e.daemon.Args = os.Args
	child, err := e.daemon.Reborn()
	if err != nil {
		return err
	}
	if child != nil {
		logThis.Info("Starting daemon...", NORMAL)
	} else {
		logThis.Info("+ varroa musica daemon started ("+Version+")", NORMAL)
		// now in the daemon
		daemon.AddCommand(boolFlag(false), syscall.SIGTERM, quitDaemon)
		e.InDaemon = true
	}
	return nil
}

func quitDaemon(sig os.Signal) error {
	logThis.Info("+ terminating", VERBOSE)
	return daemon.ErrStop
}

// WaitForDaemonStop and clean exit
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
	copy(e.ConfigPassphrase[:], passphrase)
	return nil
}

// GetPassphrase and keep in Environment
func (e *Environment) GetPassphrase() error {
	passphrase, err := getPassphrase()
	if err != nil {
		return err
	}
	copy(e.ConfigPassphrase[:], passphrase)
	return nil
}

// LoadConfiguration whether the configuration file is encrypted or not.
func (e *Environment) LoadConfiguration() error {
	newConf := &Config{}
	encryptedConfigurationFile := strings.TrimSuffix(DefaultConfigurationFile, yamlExt) + encryptedExt
	if FileExists(encryptedConfigurationFile) && !FileExists(DefaultConfigurationFile) {
		// if using encrypted config file, ask for the passphrase and retrieve it from the daemon side
		if err := e.SavePassphraseForDaemon(); err != nil {
			return err
		}
		configBytes, err := decrypt(encryptedConfigurationFile, e.ConfigPassphrase)
		if err != nil {
			return err
		}
		if err := newConf.LoadFromBytes(configBytes); err != nil {
			return err
		}
	} else {
		if err := newConf.Load(DefaultConfigurationFile); err != nil {
			return err
		}
	}
	e.Config = newConf
	// init downloads configuration
	if e.Config.DownloadFolderConfigured {
		e.Downloads = &Downloads{Root: e.Config.General.DownloadDir}
	}
	// init notifications with pushover
	if e.Config.pushoverConfigured {
		e.notification.client = pushover.New(e.Config.Notifications.Pushover.Token)
		e.notification.recipient = pushover.NewRecipient(e.Config.Notifications.Pushover.User)
	}
	if e.Config.statsConfigured {
		theme := knownThemes[darkOrange]
		if e.Config.webserverConfigured {
			theme = knownThemes[e.Config.WebServer.Theme]
		}
		e.serverData.theme = theme
		e.serverData.index = HTMLIndex{Title: strings.ToUpper(FullName), Version: Version, CSS: theme.CSS(), Script: indexJS}
	}
	// git
	if e.Config.gitlabPagesConfigured {
		e.git = NewGit(StatsDir, e.Config.GitlabPages.User, e.Config.GitlabPages.User+"+varroa@musica")
	}
	return nil
}

// SetUp the Environment
func (e *Environment) SetUp(autologin bool) error {
	// prepare directory for stats if necessary
	if !DirectoryExists(StatsDir) {
		if err := os.MkdirAll(StatsDir, 0777); err != nil {
			return errors.Wrap(err, errorCreatingStatsDir)
		}
	}
	// log in all trackers, assuming labels are unique (configuration was checked)
	for _, label := range e.Config.TrackerLabels() {
		config, err := e.Config.GetTracker(label)
		if err != nil {
			return errors.Wrap(err, "Error getting tracker information")
		}
		tracker := &GazelleTracker{Name: config.Name, URL: config.URL, User: config.User, Password: config.Password, limiter: make(chan bool, allowedAPICallsByPeriod)}
		if autologin {
			if err = tracker.Login(); err != nil {
				return errors.Wrap(err, "Error logging in tracker "+label)
			}
			logThis.Info(fmt.Sprintf("Logged in tracker %s.", label), NORMAL)
		}
		// launching rate limiter
		go tracker.apiCallRateLimiter()
		e.Trackers[label] = tracker

		statsConfig, err := e.Config.GetStats(label)
		if err != nil {
			return errors.Wrap(err, "Error loading stats config for "+label)
		}
		// load history for this tracker
		h := &History{Tracker: label}
		// load relevant history
		if err := h.LoadAll(statsConfig); err != nil {
			return errors.Wrap(err, "Error loading history for tracker "+label)
		}
		e.History[label] = h
	}
	return nil
}

// Notify in a goroutine, or directly.
func (e *Environment) Notify(msg, tracker, msgType string) error {
	notify := func() error {
		link := ""
		if e.Config.gitlabPagesConfigured {
			link = e.Config.GitlabPages.URL
		} else if e.Config.webserverConfigured && e.Config.WebServer.ServeStats && e.Config.WebServer.PortHTTPS != 0 {
			link = "https://" + e.Config.WebServer.Hostname + ":" + strconv.Itoa(e.Config.WebServer.PortHTTPS)
		}
		atLeastOneError := false
		if e.Config.pushoverConfigured {
			if err := e.notification.Send(tracker+": "+msg, e.Config.gitlabPagesConfigured, link); err != nil {
				logThis.Error(errors.Wrap(err, errorNotification), VERBOSE)
				atLeastOneError = true
			}
		}
		if e.Config.webhooksConfigured && StringInSlice(tracker, e.Config.Notifications.WebHooks.Trackers) {
			// create json, POST it
			whJSON := &WebHookJSON{Site: tracker, Message: msg, Link: link, Type: msgType}
			if err := whJSON.Send(e.Config.Notifications.WebHooks.Address, e.Config.Notifications.WebHooks.Token); err != nil {
				logThis.Error(errors.Wrap(err, errorWebhook), VERBOSE)
				atLeastOneError = true
			}
		}
		if atLeastOneError {
			return errors.New(errorNotifications)
		}
		return nil
	}
	return e.RunOrGo(notify)
}

// RunOrGo depending on whether we're in the daemon or not.
func (e *Environment) RunOrGo(f func() error) error {
	if e.InDaemon {
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
		config, err := e.Config.GetTracker(label)
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
	if !e.Config.statsConfigured {
		return nil
	}
	return e.serverData.SaveIndex(e, filepath.Join(StatsDir, htmlIndexFile))
}

// DeployToGitlabPages with git wrapper
func (e *Environment) DeployToGitlabPages() error {
	if !e.Config.gitlabPagesConfigured {
		return nil
	}
	if e.git == nil {
		return errors.New("Error setting up git")
	}
	// make sure we're going back to cwd
	defer e.git.getBack()

	// init repository if necessary
	if !e.git.Exists() {
		if err := e.git.Init(); err != nil {
			return errors.Wrap(err, errorGitInit)
		}
		// create .gitlab-ci.yml
		if err := ioutil.WriteFile(filepath.Join(StatsDir, gitlabCIYamlFile), []byte(gitlabCI), 0666); err != nil {
			return err
		}
	}
	// add main files
	if err := e.git.Add(filepath.Base(gitlabCIYamlFile), filepath.Base(htmlIndexFile), "*"+csvExt); err != nil {
		return errors.Wrap(err, errorGitAdd)
	}
	// add the graphs, if it fails,
	if err := e.git.Add("*" + svgExt); err != nil {
		logThis.Error(errors.Wrap(err, errorGitAdd+", not all graphs are generated yet."), NORMAL)
	}
	// commit
	if err := e.git.Commit("varroa musica stats update."); err != nil {
		return errors.Wrap(err, errorGitCommit)
	}
	// push
	if !e.git.HasRemote("origin") {
		if err := e.git.AddRemote("origin", e.Config.GitlabPages.GitHTTPS); err != nil {
			return errors.Wrap(err, errorGitAddRemote)
		}
	}
	if err := e.git.Push("origin", e.Config.GitlabPages.GitHTTPS, e.Config.GitlabPages.User, e.Config.GitlabPages.Password); err != nil {
		return errors.Wrap(err, errorGitPush)
	}
	logThis.Info("Pushed new stats to "+e.Config.GitlabPages.URL, NORMAL)
	return nil
}

func  (e *Environment) GoGoRoutines() {
	//  tracker-dependent goroutines
	for _, t := range e.Trackers {
		if e.Config.autosnatchConfigured {
			go ircHandler(e, t)
		}
	}
	// general goroutines
	if e.Config.statsConfigured {
		go monitorAllStats(e)
	}
	if e.Config.webserverConfigured {
		go webServer(e, e.serverHTTP, e.serverHTTPS)
	}
	// background goroutines
	go awaitOrders(e)
	go automatedTasks(e)
}