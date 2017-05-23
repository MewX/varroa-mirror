package main

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

const (
	ircServerPattern     = `^(.*):(\d*)$`
	gitRepositoryPattern = `^https://gitlab.com/(.*)/(.*).git$`
)

type ConfigGeneral struct {
	LogLevel                   int    `yaml:"log_level"`
	WatchDir                   string `yaml:"watch_directory"`
	DownloadDir                string `yaml:"download_directory"`
	AutomaticMetadataRetrieval bool   `yaml:"automatic_metadata_retrieval"`
}

func (cg *ConfigGeneral) Check() error {
	if cg.LogLevel < NORMAL || cg.LogLevel > VERBOSEST {
		return errors.New("Invalid log level")
	}
	if cg.DownloadDir != "" && !DirectoryExists(cg.DownloadDir) {
		return errors.New("Downloads directory does not exist")
	}
	if cg.WatchDir != "" && !DirectoryExists(cg.WatchDir) {
		return errors.New("Watch directory does not exist")
	}
	if cg.AutomaticMetadataRetrieval && cg.DownloadDir == "" {
		return errors.New("Downloads directory must be defined to allow metadata retrieval")
	}
	return nil
}

func (cg *ConfigGeneral) String() string {
	txt := "General configuration:\n"
	txt += "\tLog level: " + strconv.Itoa(cg.LogLevel) + "\n"
	txt += "\tWatch directory: " + cg.WatchDir + "\n"
	txt += "\tDownload directory: " + cg.DownloadDir + "\n"
	txt += "\tDownload metadata automatically: " + fmt.Sprintf("%v", cg.AutomaticMetadataRetrieval) + "\n"
	return txt
}

type ConfigTracker struct {
	Name     string
	User     string
	Password string
	URL      string
}

func (ct *ConfigTracker) Check() error {
	if ct.Name == "" {
		return errors.New("Missing tracker name")
	}
	if ct.User == "" {
		return errors.New("Missing tracker username")
	}
	if ct.Password == "" {
		return errors.New("Missing tracker password")
	}
	if ct.URL == "" {
		return errors.New("Missing tracker URL")
	}
	return nil
}

func (ct *ConfigTracker) String() string {
	txt := "Tracker configuration for " + ct.Name + "\n"
	txt += "\tUser: " + ct.User + "\n"
	txt += "\tPassword: " + ct.Password + "\n"
	txt += "\tURL: " + ct.URL + "\n"
	return txt
}

type ConfigAutosnatch struct {
	Tracker               string
	IRCServer             string `yaml:"irc_server"`
	IRCKey                string `yaml:"irc_key"`
	IRCSSL                bool   `yaml:"irc_ssl"`
	IRCSSLSkipVerify      bool   `yaml:"irc_ssl_skip_verify"`
	NickservPassword      string `yaml:"nickserv_password"`
	BotName               string `yaml:"bot_name"`
	Announcer             string
	AnnounceChannel       string   `yaml:"announce_channel"`
	BlacklistedUploaders  []string `yaml:"blacklisted_uploaders"`
	disabledAutosnatching bool
}

func (ca *ConfigAutosnatch) Check() error {
	if ca.Tracker == "" {
		return errors.New("Missing tracker name")
	}
	if ca.IRCServer == "" {
		return errors.New("Missing IRC server")
	} else {
		// check it's server:port
		r := regexp.MustCompile(ircServerPattern)
		hits := r.FindAllStringSubmatch(ca.IRCServer, -1)
		if len(hits) != 1 {
			return errors.New("IRC server must be in the form: server.hostname:port")
		}
	}
	if ca.IRCKey == "" {
		return errors.New("Missing IRC key")
	}
	if ca.NickservPassword == "" {
		return errors.New("Missing NickServ password")
	}
	if ca.BotName == "" {
		return errors.New("Missing bot registered nickname")
	}
	if ca.Announcer == "" {
		return errors.New("Missing announcer bot")
	}
	if ca.AnnounceChannel == "" {
		return errors.New("Missing announce channel")
	} else {
		if !strings.HasPrefix(ca.AnnounceChannel, "#") {
			return errors.New("Invalid announce channel")
		}
	}
	return nil
}

func (ca *ConfigAutosnatch) String() string {
	txt := "Autosnatch configuration for " + ca.Tracker + "\n"
	txt += "\tIRC server: " + ca.IRCServer + "\n"
	txt += "\tIRC KeyPassword: " + ca.IRCKey + "\n"
	txt += "\tUse SSL: " + fmt.Sprintf("%v", ca.IRCSSL) + "\n"
	txt += "\tSkip SSL verification: " + fmt.Sprintf("%v", ca.IRCSSLSkipVerify) + "\n"
	txt += "\tNickserv password: " + ca.NickservPassword + "\n"
	txt += "\tBot nickname: " + ca.BotName + "\n"
	txt += "\tAnnouncer: " + ca.Announcer + "\n"
	txt += "\tAnnounce channel: " + ca.AnnounceChannel + "\n"
	if len(ca.BlacklistedUploaders) != 0 {
		txt += "\tBlacklisted uploaders: " + strings.Join(ca.BlacklistedUploaders, ",") + "\n"
	}
	return txt
}

type ConfigStats struct {
	Tracker             string
	UpdatePeriodH       int `yaml:"update_period_hour"`
	MaxBufferDecreaseMB int `yaml:"max_buffer_decrease_by_period_mb"`
}

func (cs *ConfigStats) Check() error {
	if cs.Tracker == "" {
		return errors.New("Missing tracker name")
	}
	if cs.UpdatePeriodH == 0 {
		return errors.New("Missing stats update period (in hours)")
	}
	return nil
}

func (cs *ConfigStats) String() string {
	txt := "Stats configuration for " + cs.Tracker + "\n"
	txt += "\tUpdate period (hours): " + strconv.Itoa(cs.UpdatePeriodH) + "\n"
	txt += "\tMaximum buffer decrease (MB): " + strconv.Itoa(cs.MaxBufferDecreaseMB) + "\n"
	return txt
}

type ConfigWebServer struct {
	ServeMetadata  bool   `yaml:"serve_metadata"`
	ServeStats     bool   `yaml:"serve_stats"`
	User           string `yaml:"stats_user"`
	Password       string `yaml:"stats_password"`
	AllowDownloads bool   `yaml:"allow_downloads"`
	Token          string
	PortHTTP       int    `yaml:"http_port"`
	PortHTTPS      int    `yaml:"https_port"`
	Hostname       string `yaml:"https_hostname"`
}

func (cw *ConfigWebServer) Check() error {
	if !cw.ServeStats && !cw.AllowDownloads && !cw.ServeMetadata {
		return errors.New("Webserver configured, but not serving stats or allowing remote downloads")
	}
	if cw.AllowDownloads && cw.Token == "" {
		return errors.New("A user-defined token must be configured to allow remove downloads")
	}
	if cw.PortHTTP == 0 && cw.PortHTTPS == 0 {
		return errors.New("HTTP and/or HTTPS port(s) must be configured")
	}
	if cw.PortHTTPS == cw.PortHTTP {
		return errors.New("HTTP and/or HTTPS port(s) must be different")
	}
	// TODO NOT TRUE if the user provides the certificates...
	if cw.PortHTTPS != 0 && cw.Hostname == "" {
		return errors.New("HTTPS server requires a hostname")
	}
	if cw.Password != "" && cw.User == "" || cw.Password == "" && cw.User != "" {
		return errors.New("If password-protecting the stats webserver, both user & password must be provided")
	}
	return nil
}

func (cw *ConfigWebServer) String() string {
	txt := "Webserver configuration:\n"
	txt += "\tServe stats: " + fmt.Sprintf("%v", cw.ServeStats) + "\n"
	txt += "\tUser: " + cw.User + "\n"
	txt += "\tPassword: " + cw.Password + "\n"
	txt += "\tAllow downloads: " + fmt.Sprintf("%v", cw.AllowDownloads) + "\n"
	txt += "\tToken: " + cw.Token + "\n"
	txt += "\tHTTP port: " + strconv.Itoa(cw.PortHTTP) + "\n"
	txt += "\tHTTPS port: " + strconv.Itoa(cw.PortHTTPS) + "\n"
	txt += "\tHostname: " + cw.Hostname + "\n"
	return txt
}

type ConfigNotifications struct {
	Pushover *ConfigPushover
	WebHooks *WebHooksConfig
}

type ConfigPushover struct {
	User  string
	Token string
}

func (cp *ConfigPushover) Check() error {
	if cp.User == "" && cp.Token != "" {
		return errors.New("Pushover userID must be provided")
	}
	if cp.Token == "" && cp.User != "" {
		return errors.New("Pushover token must be provided")
	}
	return nil
}

func (cp *ConfigPushover) String() string {
	txt := "Pushover configuration:\n"
	txt += "\tUser: " + cp.User + "\n"
	txt += "\tToken: " + cp.Token + "\n"
	return txt
}

type WebHooksConfig struct {
	Address  string
	Token    string
	Trackers []string
}

func (whc *WebHooksConfig) Check() error {
	// TODO check address format!
	if whc.Address == "" {
		return errors.New("Webhook configuration must provide remote server address")
	}
	if whc.Token == "" {
		return errors.New("Webhook configuration must provide a token for the remote server")
	}
	if len(whc.Trackers) == 0 {
		return errors.New("Webhook configuration must provide the list of relevant trackers")
	}
	return nil
}

func (whc *WebHooksConfig) String() string {
	txt := "WebHook configuration:\n"
	txt += "\tAddress: " + whc.Address + "\n"
	txt += "\tToken: " + whc.Token + "\n"
	txt += "\tTrackers: " + strings.Join(whc.Trackers, ", ") + "\n"
	return txt
}

type ConfigGitlabPages struct {
	GitHTTPS string `yaml:"git_https"`
	User     string
	Password string
	URL      string
}

func (cg *ConfigGitlabPages) Check() error {
	if cg.User == "" {
		return errors.New("Gitlab username must be provided")
	}
	if cg.Password == "" {
		return errors.New("Gitlab password must be provided")
	}
	if cg.GitHTTPS == "" {
		return errors.New("Gitlab repository must be provided")
	} else {
		// check form
		r := regexp.MustCompile(gitRepositoryPattern)
		hits := r.FindAllStringSubmatch(cg.GitHTTPS, -1)
		if len(hits) != 1 {
			return errors.New("Gitlab Pages git repository must be in the form: https://gitlab.com/USER/REPO.git")
		}
		cg.URL = fmt.Sprintf("https://%s.gitlab.io/%s", hits[0][1], hits[0][2])
	}
	return nil
}

func (cg *ConfigGitlabPages) String() string {
	txt := "Gitlab Pages configuration:\n"
	txt += "\tGit repository: " + cg.GitHTTPS + "\n"
	txt += "\tUser: " + cg.User + "\n"
	txt += "\tPassword: " + cg.Password + "\n"
	txt += "\tURL: " + cg.URL + "\n"
	return txt
}

type ConfigFilter struct {
	Name                string   `yaml:"name"`
	Artist              []string `yaml:"artist"`
	ExcludedArtist      []string `yaml:"excluded_artist"`
	Year                []int    `yaml:"year"`
	RecordLabel         []string `yaml:"record_label"`
	TagsIncluded        []string `yaml:"included_tags"`
	TagsExcluded        []string `yaml:"excluded_tags"`
	ReleaseType         []string `yaml:"type"`
	ExcludedReleaseType []string `yaml:"excluded_type"`
	Edition             []string `yaml:"edition_contains"`
	Format              []string `yaml:"format"`
	Source              []string `yaml:"source"`
	Quality             []string `yaml:"quality"`
	HasCue              bool     `yaml:"has_cue"`
	HasLog              bool     `yaml:"has_log"`
	LogScore            int      `yaml:"log_score"`
	PerfectFlac         bool     `yaml:"perfect_flac"`
	AllowDuplicates     bool     `yaml:"allow_duplicates"`
	AllowScene          bool     `yaml:"allow_scene"`
	MinSizeMB           int      `yaml:"min_size_mb"`
	MaxSizeMB           int      `yaml:"max_size_mb"`
	WatchDir            string   `yaml:"watch_directory"`
	UniqueInGroup       bool     `yaml:"unique_in_group"`
	Tracker             []string `yaml:"tracker"`
}

func (cf *ConfigFilter) Check() error {
	if cf.Name == "" {
		return errors.New("Missing filter name")
	}
	if (cf.HasCue || cf.HasLog || cf.LogScore != 0) && !StringInSlice("CD", cf.Source) {
		return errors.New("Has Log/Cue only relevant if CD is an acceptable source")
	}
	if cf.MaxSizeMB < 0 || cf.MinSizeMB < 0 {
		return errors.New("Minimun and maximum sizes must not be negative")
	}
	if cf.MaxSizeMB > 0 && cf.MinSizeMB >= cf.MaxSizeMB {
		return errors.New("Minimun release size must be lower than maximum release size")
	}
	if cf.WatchDir != "" && !DirectoryExists(cf.WatchDir) {
		return errors.New("Specific filter watch directory does not exist")
	}
	if CommonInStringSlices(cf.ExcludedArtist, cf.Artist) != nil {
		return errors.New("Artists cannot be both included and excluded")
	}
	if CommonInStringSlices(cf.TagsExcluded, cf.TagsIncluded) != nil {
		return errors.New("Tags cannot be both included and excluded")
	}
	if len(cf.ExcludedReleaseType) != 0 && len(cf.ReleaseType) != 0 {
		return errors.New("Release types cannot be both included and excluded")
	}
	if cf.UniqueInGroup && cf.AllowDuplicates {
		return errors.New("Filter can both allow duplicates and only allow one snatch/torrentgroup")
	}
	if cf.PerfectFlac {
		if cf.Format != nil || cf.Quality != nil || cf.Source != nil || cf.HasLog || cf.HasCue || cf.LogScore != 0 {
			return errors.New("The perfect_flag option replaces all options about quality, source, format, and cue/log/log score")
		}
		// setting the relevant options
		cf.Format = []string{"FLAC"}
		cf.Quality = []string{"Lossless", "24bit Lossless"}
		cf.HasCue = true
		cf.HasLog = true
		cf.LogScore = 100
		cf.Source = []string{"CD", "Vinyl", "DVD", "Soundboard", "WEB", "Cassette", "Blu-ray", "SACD", "DAT"}
	}
	if reflect.DeepEqual(*cf, ConfigFilter{Name: cf.Name}) {
		return errors.New("Empty filter would snatch everything, it probably is not what you want")
	}

	// TODO: check source/quality against hard-coded values?, MP3, 24bit Lossless, etc?
	// TODO: check impossible filters: ie format :FLAC + quality: 320

	return nil
}

func (cf *ConfigFilter) String() string {
	description := "Filter configuration for " + cf.Name + ":\n"
	if len(cf.Year) != 0 {
		description += "\tYear(s): " + strings.Join(IntSliceToStringSlice(cf.Year), ", ") + "\n"
	}
	if len(cf.Artist) != 0 {
		description += "\tArtist(s): " + strings.Join(cf.Artist, ", ") + "\n"
	}
	if len(cf.RecordLabel) != 0 {
		description += "\tRecord Label(s): " + strings.Join(cf.RecordLabel, ", ") + "\n"
	}
	if len(cf.TagsIncluded) != 0 {
		description += "\tRequired tags: " + strings.Join(cf.TagsIncluded, ", ") + "\n"
	}
	if len(cf.TagsExcluded) != 0 {
		description += "\tExcluded tags: " + strings.Join(cf.TagsExcluded, ", ") + "\n"
	}
	if len(cf.Source) != 0 {
		description += "\tSource(s): " + strings.Join(cf.Source, ", ") + "\n"
	}
	if len(cf.Format) != 0 {
		description += "\tFormat(s): " + strings.Join(cf.Format, ", ") + "\n"
	}
	if len(cf.Quality) != 0 {
		description += "\tQuality: " + strings.Join(cf.Quality, ", ") + "\n"
	}
	if len(cf.ReleaseType) != 0 {
		description += "\tType(s): " + strings.Join(cf.ReleaseType, ", ") + "\n"
	}
	if len(cf.ExcludedReleaseType) != 0 {
		description += "\tExcluded Type(s): " + strings.Join(cf.ExcludedReleaseType, ", ") + "\n"
	}
	if cf.HasCue {
		description += "\tHas Cue: true\n"
	}
	if cf.HasLog {
		description += "\tHas Log: true\n"
	}
	if cf.LogScore != 0 {
		description += "\tMinimum Log Score: " + strconv.Itoa(cf.LogScore) + "\n"
	}
	if cf.AllowScene {
		description += "\tAllow Scene releases: true\n"
	}
	if cf.AllowDuplicates {
		description += "\tAllow duplicates: true\n"
	}
	if cf.MinSizeMB != 0 {
		description += "\tMinimum Size: " + strconv.Itoa(cf.MinSizeMB) + "\n"
	}
	if cf.MaxSizeMB != 0 {
		description += "\tMaximum Size: " + strconv.Itoa(cf.MaxSizeMB) + "\n"
	}
	if cf.WatchDir != "" {
		description += "\tSpecial destination folder: " + cf.WatchDir + "\n"
	}
	return description
}

type Config struct {
	General                  *ConfigGeneral
	Trackers                 []*ConfigTracker
	Autosnatch               []*ConfigAutosnatch
	Stats                    []*ConfigStats
	WebServer                *ConfigWebServer
	Notifications            *ConfigNotifications
	GitlabPages              *ConfigGitlabPages `yaml:"gitlab_pages"`
	Filters                  []*ConfigFilter
	autosnatchConfigured     bool
	statsConfigured          bool
	webserverConfigured      bool
	webserverHTTP            bool
	webserverHTTPS           bool
	gitlabPagesConfigured    bool
	pushoverConfigured       bool
	webhooksConfigured       bool
	downloadFolderConfigured bool
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
	if c.gitlabPagesConfigured {
		txt += c.GitlabPages.String() + "\n"
	}
	if c.webhooksConfigured {
		txt += c.Notifications.WebHooks.String() + "\n"
	}
	return txt
}

func (c *Config) Load(file string) error {
	// loading the configuration file
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return errors.Wrap(err, "Error reading configuration file "+file)
	}
	return c.LoadFromBytes(b)
}

func (c *Config) LoadFromBytes(b []byte) error {
	err := yaml.Unmarshal(b, &c)
	if err != nil {
		return errors.Wrap(err, "Error loading configuration")
	}
	return c.Check()
}

func (c *Config) Check() error {
	// general checks
	if c.General == nil {
		return errors.New("General configuration required")
	}
	if err := c.General.Check(); err != nil {
		return errors.Wrap(err, "Error reading general configuration")
	}
	// tracker checks
	if len(c.Trackers) == 0 {
		return errors.New("Missing tracker information")
	}
	for _, t := range c.Trackers {
		if err := t.Check(); err != nil {
			return errors.Wrap(err, "Error reading tracker configuration")
		}
	}
	// autosnatch checks
	for _, t := range c.Autosnatch {
		if err := t.Check(); err != nil {
			return errors.Wrap(err, "Error reading autosnatch configuration")
		}
	}
	// stats checks
	for _, t := range c.Stats {
		if err := t.Check(); err != nil {
			return errors.Wrap(err, "Error reading stats configuration")
		}
	}
	// webserver checks
	if c.WebServer != nil {
		if err := c.WebServer.Check(); err != nil {
			return errors.Wrap(err, "Error reading webserver configuration")
		}
	}
	// pushover checks
	if c.Notifications != nil && c.Notifications.Pushover != nil {
		if err := c.Notifications.Pushover.Check(); err != nil {
			return errors.Wrap(err, "Error reading pushover configuration")
		}
	}
	// webhook checks
	if c.Notifications != nil && c.Notifications.WebHooks != nil {
		if err := c.Notifications.WebHooks.Check(); err != nil {
			return errors.Wrap(err, "Error reading webhooks configuration")
		}
	}
	// gitlab checks
	if c.GitlabPages != nil {
		if err := c.GitlabPages.Check(); err != nil {
			return errors.Wrap(err, "Error reading Gitlab Pages configuration")
		}
	}
	// filter checks
	for _, t := range c.Filters {
		if err := t.Check(); err != nil {
			return errors.Wrap(err, "Error reading filter configuration")
		}
	}

	// setting a few shortcut flags
	c.autosnatchConfigured = len(c.Autosnatch) != 0
	c.statsConfigured = len(c.Stats) != 0
	c.webserverConfigured = c.WebServer != nil
	c.gitlabPagesConfigured = c.GitlabPages != nil
	c.pushoverConfigured = c.Notifications != nil && c.Notifications.Pushover != nil
	c.webhooksConfigured = c.Notifications != nil && c.Notifications.WebHooks != nil
	c.downloadFolderConfigured = c.General.DownloadDir != ""
	c.webserverHTTP = c.webserverConfigured && c.WebServer.PortHTTP != 0
	c.webserverHTTPS = c.webserverConfigured && c.WebServer.PortHTTPS != 0

	// config-wide checks
	configuredTrackers := c.TrackerLabels()
	if len(c.Autosnatch) != 0 {
		if c.General.WatchDir == "" {
			return errors.New("Autosnatch enabled, existing watch directory must be provided")
		}
		if len(c.Filters) == 0 {
			return errors.New("Autosnatch enabled, but no filters are defined")
		}
		// check all autosnatch configs point to defined Trackers
		for _, a := range c.Autosnatch {
			if !StringInSlice(a.Tracker, configuredTrackers) {
				return errors.New(fmt.Sprintf("Autosnatch enabled, but tracker %s undefined", a.Tracker))
			}
		}
		// check all filter trackers are defined
		if len(c.Filters) != 0 {
			for _, f := range c.Filters {
				for _, t := range f.Tracker {
					if !StringInSlice(t, configuredTrackers) {
						return errors.New(fmt.Sprintf("Filter %s refers to undefined tracker %s", f.Name, t))
					}
				}
			}
		}
	}
	if len(c.Stats) != 0 {
		// check all stats point to defined Trackers
		for _, a := range c.Stats {
			if !StringInSlice(a.Tracker, configuredTrackers) {
				return errors.New(fmt.Sprintf("Stats enabled, but tracker %s undefined", a.Tracker))
			}
		}
	}
	if c.webhooksConfigured {
		// check all webhook trackers point to defined Trackers
		for _, a := range c.Notifications.WebHooks.Trackers {
			if !StringInSlice(a, configuredTrackers) {
				return errors.New(fmt.Sprintf("Stats enabled, but tracker %s undefined", a))
			}
		}
	}
	if c.webserverConfigured && c.WebServer.ServeStats && len(c.Stats) == 0 {
		return errors.New("Webserver configured to serve stats, but no stats configured")
	}
	if c.gitlabPagesConfigured && len(c.Stats) == 0 {
		return errors.New("GitLab Pages configured to serve stats, but no stats configured")
	}
	if len(c.Filters) != 0 && len(c.Autosnatch) == 0 {
		return errors.New("Filters defined but no autosnatch configuration found")
	}
	if c.webhooksConfigured && c.WebServer.ServeMetadata && !c.downloadFolderConfigured {
		return errors.New("Webserver configured to serve metadata, but download folder not configured")
	}
	if c.webhooksConfigured && c.WebServer.ServeMetadata && !c.General.AutomaticMetadataRetrieval {
		return errors.New("Webserver configured to serve metadata, but metadata automatic download not configured")
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
	labels := []string{}
	for _, t := range c.Trackers {
		labels = append(labels, t.Name)
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
