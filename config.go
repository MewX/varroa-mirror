package varroa

import (
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"gitlab.com/catastrophic/assistance/fs"
	"gitlab.com/catastrophic/assistance/logthis"
	"gitlab.com/catastrophic/assistance/strslice"
	yaml "gopkg.in/yaml.v2"
)

var config *Config
var onceConfig sync.Once

type Config struct {
	General                     *ConfigGeneral
	Trackers                    []*ConfigTracker
	Autosnatch                  []*ConfigAutosnatch
	Stats                       []*ConfigStats
	WebServer                   *ConfigWebServer
	Notifications               *ConfigNotifications
	GitlabPages                 *ConfigGitlabPages `yaml:"gitlab_pages"`
	Filters                     []*ConfigFilter
	Library                     *ConfigLibrary
	MPD                         *ConfigMPD
	Metadata                    *ConfigMetadata
	autosnatchConfigured        bool
	statsConfigured             bool
	webserverConfigured         bool
	webserverHTTP               bool
	webserverHTTPS              bool
	webserverMetadata           bool
	gitlabPagesConfigured       bool
	pushoverConfigured          bool
	ircNotifsConfigured         bool
	webhooksConfigured          bool
	DownloadFolderConfigured    bool
	LibraryConfigured           bool
	playlistDirectoryConfigured bool
	mpdConfigured               bool
	metadataConfigured          bool
	discogsTokenConfigured      bool
}

func NewConfig(path string) (*Config, error) {
	var newConfigErr error
	onceConfig.Do(func() {
		// TODO check path has yamlExt!
		newConf := &Config{}
		encryptedConfigurationFile := strings.TrimSuffix(path, yamlExt) + encryptedExt
		if fs.FileExists(encryptedConfigurationFile) && !fs.FileExists(path) {
			// if using encrypted config file, ask for the passphrase and retrieve it from the daemon side
			passphraseBytes, err := SavePassphraseForDaemon()
			if err != nil {
				newConfigErr = err
				return
			}
			configBytes, err := decrypt(encryptedConfigurationFile, passphraseBytes)
			if err != nil {
				newConfigErr = err
				return
			}
			if err := newConf.LoadFromBytes(configBytes); err != nil {
				newConfigErr = err
				return
			}
		} else {
			if err := newConf.Load(path); err != nil {
				newConfigErr = err
				return
			}
		}
		// set the global pointer once everything is OK.
		config = newConf
	})
	return config, newConfigErr
}

func (c *Config) String() string {
	txt := c.General.String() + "\n"
	for _, f := range c.Trackers {
		txt += f.String() + "\n"
	}
	for _, f := range c.Stats {
		txt += f.String() + "\n"
	}
	for _, f := range c.Autosnatch {
		txt += f.String() + "\n"
	}
	for _, f := range c.Filters {
		txt += f.String() + "\n"
	}
	if c.webserverConfigured {
		txt += c.WebServer.String() + "\n"
	}
	if c.pushoverConfigured {
		txt += c.Notifications.Pushover.String() + "\n"
	}
	if c.ircNotifsConfigured {
		txt += c.Notifications.Irc.String() + "\n"
	}
	if c.webhooksConfigured {
		txt += c.Notifications.WebHooks.String() + "\n"
	}
	if c.gitlabPagesConfigured {
		txt += c.GitlabPages.String() + "\n"
	}
	if c.mpdConfigured {
		txt += c.MPD.String() + "\n"
	}
	if c.LibraryConfigured {
		txt += c.Library.String() + "\n"
	}
	if c.metadataConfigured {
		txt += c.Metadata.String() + "\n"
	}
	return txt
}

func (c *Config) Load(file string) error {
	// loading the configuration file
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return errors.Wrap(err, errorReadingConfig)
	}
	return c.LoadFromBytes(b)
}

func (c *Config) LoadFromBytes(b []byte) error {
	err := yaml.Unmarshal(b, &c)
	if err != nil {
		return errors.Wrap(err, errorLoadingYAML)
	}
	return c.check()
}

func (c *Config) check() error {
	// general checks
	if c.General == nil {
		return errors.New("General configuration required")
	}
	if err := c.General.check(); err != nil {
		return errors.Wrap(err, "Error reading general configuration")
	}
	// setting log level
	logthis.SetLevel(c.General.LogLevel)
	logthis.SetStdOutput(true)

	// tracker checks
	if len(c.Trackers) == 0 {
		return errors.New("Missing tracker information")
	}
	for _, t := range c.Trackers {
		if err := t.check(); err != nil {
			return errors.Wrap(err, "Error reading tracker configuration")
		}
	}
	// autosnatch checks
	for _, t := range c.Autosnatch {
		if err := t.check(); err != nil {
			return errors.Wrap(err, "Error reading autosnatch configuration")
		}
	}
	// stats checks
	for _, t := range c.Stats {
		if err := t.check(); err != nil {
			return errors.Wrap(err, "Error reading stats configuration")
		}
	}
	// webserver checks
	if c.WebServer != nil {
		if err := c.WebServer.check(); err != nil {
			return errors.Wrap(err, "Error reading webserver configuration")
		}
	}
	// pushover checks
	if c.Notifications != nil && c.Notifications.Pushover != nil {
		if err := c.Notifications.Pushover.check(); err != nil {
			return errors.Wrap(err, "Error reading pushover configuration")
		}
	}
	// irc notifications checks
	if c.Notifications != nil && c.Notifications.Irc != nil {
		if err := c.Notifications.Irc.check(); err != nil {
			return errors.Wrap(err, "Error reading IRC notifications configuration")
		}
	}
	// webhook checks
	if c.Notifications != nil && c.Notifications.WebHooks != nil {
		if err := c.Notifications.WebHooks.check(); err != nil {
			return errors.Wrap(err, "Error reading webhooks configuration")
		}
	}
	// gitlab checks
	if c.GitlabPages != nil {
		if err := c.GitlabPages.check(); err != nil {
			return errors.Wrap(err, "Error reading Gitlab Pages configuration")
		}
	}
	// library checks
	if c.Library != nil {
		if err := c.Library.check(); err != nil {
			return errors.Wrap(err, "Error reading library configuration")
		}
	}
	// mpd checks
	if c.MPD != nil {
		if err := c.MPD.check(); err != nil {
			return errors.Wrap(err, "Error reading MPD configuration")
		}
	}
	// filter checks
	for _, t := range c.Filters {
		if err := t.check(); err != nil {
			return errors.Wrap(err, "Error reading filter configuration")
		}
	}
	// metadata checks
	if c.Metadata != nil {
		if err := c.Metadata.check(); err != nil {
			return errors.Wrap(err, "Error reading Metadata configuration")
		}
	}

	// setting a few shortcut flags
	c.autosnatchConfigured = len(c.Autosnatch) != 0
	c.statsConfigured = len(c.Stats) != 0
	c.webserverConfigured = c.WebServer != nil
	c.gitlabPagesConfigured = c.GitlabPages != nil
	c.pushoverConfigured = c.Notifications != nil && c.Notifications.Pushover != nil
	c.ircNotifsConfigured = c.Notifications != nil && c.Notifications.Irc != nil
	c.webhooksConfigured = c.Notifications != nil && c.Notifications.WebHooks != nil
	c.DownloadFolderConfigured = c.General.DownloadDir != ""
	c.webserverHTTP = c.webserverConfigured && c.WebServer.PortHTTP != 0
	c.webserverHTTPS = c.webserverConfigured && c.WebServer.PortHTTPS != 0
	c.LibraryConfigured = c.Library != nil
	c.playlistDirectoryConfigured = c.LibraryConfigured && c.Library.PlaylistDirectory != ""
	c.mpdConfigured = c.MPD != nil
	c.webserverMetadata = c.DownloadFolderConfigured && c.webserverConfigured && c.WebServer.ServeMetadata
	c.metadataConfigured = c.Metadata != nil
	c.discogsTokenConfigured = c.metadataConfigured && c.Metadata.DiscogsToken != ""

	// config-wide checks
	configuredTrackers := c.TrackerLabels()
	if c.autosnatchConfigured {
		if c.General.WatchDir == "" {
			return errors.New("Autosnatch enabled, existing watch directory must be provided")
		}
		if len(c.Filters) == 0 {
			return errors.New("Autosnatch enabled, but no filters are defined")
		}
		// check all autosnatch configs point to defined Trackers
		for _, a := range c.Autosnatch {
			if !strslice.Contains(configuredTrackers, a.Tracker) {
				return fmt.Errorf("autosnatch enabled, but tracker %s undefined", a.Tracker)
			}
		}
		// check all filter trackers are defined
		if len(c.Filters) != 0 {
			for _, f := range c.Filters {
				// checking applicable trackers in filters
				for _, t := range f.Tracker {
					if !strslice.Contains(configuredTrackers, t) {
						return fmt.Errorf("filter %s refers to undefined tracker %s", f.Name, t)
					}
				}
			}
		}
	}
	if c.statsConfigured {
		// check all stats point to defined Trackers
		for _, a := range c.Stats {
			if !strslice.Contains(configuredTrackers, a.Tracker) {
				return fmt.Errorf("stats enabled, but tracker %s undefined", a.Tracker)
			}
		}
	}
	if c.webhooksConfigured {
		// check all webhook trackers point to defined Trackers
		for _, a := range c.Notifications.WebHooks.Trackers {
			if !strslice.Contains(configuredTrackers, a) {
				return fmt.Errorf("stats enabled, but tracker %s undefined", a)
			}
		}
	}
	if c.webserverConfigured && c.WebServer.ServeStats && len(c.Stats) == 0 {
		return errors.New("Webserver configured to serve stats, but no stats configured")
	}
	if c.gitlabPagesConfigured && len(c.Stats) == 0 {
		return errors.New("GitLab Pages configured to serve stats, but no stats configured")
	}
	if len(c.Filters) != 0 && !c.autosnatchConfigured {
		return errors.New("Filters defined but no autosnatch configuration found")
	}
	if c.webhooksConfigured && c.WebServer.ServeMetadata && !c.DownloadFolderConfigured {
		return errors.New("Webserver configured to serve metadata, but download folder not configured")
	}
	if c.webhooksConfigured && c.webserverConfigured && c.WebServer.ServeMetadata && !c.General.AutomaticMetadataRetrieval {
		return errors.New("Webserver configured to serve metadata, but metadata automatic download not configured")
	}
	if c.LibraryConfigured && !c.DownloadFolderConfigured {
		return errors.New("Library is configured but not the default download directory")
	}
	if c.mpdConfigured && !c.DownloadFolderConfigured {
		return errors.New("To use the MPD server, a valid download directory must be provided")
	}
	if c.ircNotifsConfigured && !c.autosnatchConfigured {
		return errors.New("To use IRC notifications, a correctly configured IRC server must be defined in the autosnatch section")
	}
	if c.ircNotifsConfigured {
		// check the tracker has an associated IRC server defined
		var configuredIRCServers []string
		for _, a := range c.Autosnatch {
			configuredIRCServers = append(configuredIRCServers, a.Tracker)
		}
		if !strslice.Contains(configuredIRCServers, c.Notifications.Irc.Tracker) {
			return errors.New("IRC server for tracker " + c.Notifications.Irc.Tracker + " is not defined in the autosnatch section.")
		}
	}

	// TODO check no duplicates (2 Stats/autosnatch for same tracker, 2 trackers with same name)
	// TODO warning if autosnatch but no automatic disabling if buffer drops

	return nil
}

func (c *Config) Encrypt(file string, passphrase []byte) error {
	return encryptAndSave(file, passphrase)
}

func (c *Config) DecryptTo(file string, passphrase []byte) error {
	encryptedConfigurationFile := strings.TrimSuffix(file, yamlExt) + encryptedExt
	return decryptAndSave(encryptedConfigurationFile, passphrase)
}

func (c *Config) TrackerLabels() []string {
	labels := make([]string, len(c.Trackers))
	for i, t := range c.Trackers {
		labels[i] = t.Name
	}
	return labels
}

func (c *Config) GetTracker(label string) (*ConfigTracker, error) {
	for _, t := range c.Trackers {
		if t.Name == label {
			return t, nil
		}
	}
	return nil, errors.New("Could not find configuration for tracker " + label)
}

func (c *Config) GetStats(label string) (*ConfigStats, error) {
	for _, t := range c.Stats {
		if t.Tracker == label {
			return t, nil
		}
	}
	return nil, errors.New("Could not find Stats configuration for tracker " + label)
}

func (c *Config) GetAutosnatch(label string) (*ConfigAutosnatch, error) {
	for _, t := range c.Autosnatch {
		if t.Tracker == label {
			return t, nil
		}
	}
	return nil, errors.New("Could not find Autosnatch configuration for tracker " + label)
}
