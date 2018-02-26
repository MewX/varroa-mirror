package varroa

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
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
	if cg.LogLevel < NORMAL || cg.LogLevel > VERBOSESTEST {
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

// String representation for ConfigGeneral.
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
	LocalAddress          string `yaml:"local_address"`
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
	}
	// check it's server:port
	r := regexp.MustCompile(ircServerPattern)
	hits := r.FindAllStringSubmatch(ca.IRCServer, -1)
	if len(hits) != 1 {
		return errors.New("IRC server must be in the form: server.hostname:port")
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
	}
	if !strings.HasPrefix(ca.AnnounceChannel, "#") {
		return errors.New("Invalid announce channel")
	}
	return nil
}

func (ca *ConfigAutosnatch) String() string {
	txt := "Autosnatch configuration for " + ca.Tracker + "\n"
	if ca.LocalAddress != "" {
		txt += "\tLocal address: " + ca.LocalAddress + "\n"
	}
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
	} else {
		txt += "\tNo blacklisted uploaders"
	}
	return txt
}

type ConfigLibrary struct {
	Directory      string `yaml:"directory"`
	UseHardLinks   bool   `yaml:"use_hard_links"`
	FolderTemplate string `yaml:"folder_template"`
}

func (cl *ConfigLibrary) Check() error {
	if cl.Directory == "" || !DirectoryExists(cl.Directory) {
		return errors.New("Library directory does not exist")
	}
	if strings.Contains(cl.FolderTemplate, "/") {
		return errors.New("Library folder template cannot contains subdirectories")
	}
	return nil
}

func (cl *ConfigLibrary) String() string {
	txt := "Library configuration:\n"
	txt += "\tDirectory: " + cl.Directory + "\n"
	txt += "\tUse hard links: " + fmt.Sprintf("%v", cl.UseHardLinks) + "\n"
	txt += "\tFolder name template: " + cl.FolderTemplate + "\n"
	return txt
}

type ConfigStats struct {
	Tracker             string
	UpdatePeriodH       int     `yaml:"update_period_hour"`
	MaxBufferDecreaseMB int     `yaml:"max_buffer_decrease_by_period_mb"`
	MinimumRatio        float64 `yaml:"min_ratio"`
	TargetRatio         float64 `yaml:"target_ratio"`
}

func (cs *ConfigStats) Check() error {
	if cs.Tracker == "" {
		return errors.New("Missing tracker name")
	}
	if cs.UpdatePeriodH == 0 {
		return errors.New("Missing stats update period (in hours)")
	}
	if cs.MinimumRatio == 0 {
		cs.MinimumRatio = warningRatio
	}
	if cs.MinimumRatio < warningRatio {
		return fmt.Errorf("Minimum ratio must be at least %.2f", warningRatio)
	}
	if cs.TargetRatio == 0 {
		cs.TargetRatio = defaultTargetRatio
	}
	if cs.TargetRatio < warningRatio {
		return fmt.Errorf("Target ratio must be higher than %.2f", warningRatio)
	}
	if cs.TargetRatio < cs.MinimumRatio {
		return fmt.Errorf("Target ratio must be higher than minimum ratio (%.2f)", cs.MinimumRatio)
	}
	return nil
}

func (cs *ConfigStats) String() string {
	txt := "Stats configuration for " + cs.Tracker + "\n"
	txt += "\tUpdate period (hours): " + strconv.Itoa(cs.UpdatePeriodH) + "\n"
	txt += "\tMaximum buffer decrease (MB): " + strconv.Itoa(cs.MaxBufferDecreaseMB) + "\n"
	txt += "\tMinimum ratio: " + strconv.FormatFloat(cs.MinimumRatio, 'f', 2, 64) + "\n"
	txt += "\tTarget ratio: " + strconv.FormatFloat(cs.TargetRatio, 'f', 2, 64) + "\n"
	return txt
}

type ConfigWebServer struct {
	ServeMetadata  bool   `yaml:"serve_metadata"`
	ServeStats     bool   `yaml:"serve_stats"`
	Theme          string `yaml:"theme"`
	User           string `yaml:"stats_user"`
	Password       string `yaml:"stats_password"`
	AllowDownloads bool   `yaml:"allow_downloads"`
	Token          string `yaml:"token"`
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
	if cw.Theme == "" {
		cw.Theme = "dark_orange"
	}
	if !StringInSlice(cw.Theme, knownThemeNames) {
		return errors.New("Unknown theme name")
	}
	return nil
}

func (cw *ConfigWebServer) String() string {
	txt := "Webserver configuration:\n"
	txt += "\tServe stats: " + fmt.Sprintf("%v", cw.ServeStats) + "\n"
	txt += "\tServe metadata: " + fmt.Sprintf("%v", cw.ServeMetadata) + "\n"
	txt += "\tTheme: " + cw.Theme + "\n"
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
	User               string
	Token              string
	IncludeBufferGraph bool `yaml:"include_buffer_graph"`
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
	txt += "\tInclude Buffer Graph: " + fmt.Sprintf("%v", cp.IncludeBufferGraph) + "\n"
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
	Folder   string
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
		cg.Folder = hits[0][2]
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

type ConfigMPD struct {
	Server   string
	Password string
	Library  string
}

func (cm *ConfigMPD) String() string {
	txt := "MPD configuration:\n"
	txt += "\tMPD Library: " + cm.Library + "\n"
	txt += "\tMPD Server: " + cm.Server + "\n"
	txt += "\tMPD Server password: " + cm.Password + "\n"
	return txt
}

func (cm *ConfigMPD) Check() error {
	if cm.Server == "" {
		return errors.New("Server name must be provided")
	} else {
		// check it's server:port
		r := regexp.MustCompile(ircServerPattern)
		hits := r.FindAllStringSubmatch(cm.Server, -1)
		if len(hits) != 1 {
			return errors.New("MPD server must be in the form: server.hostname:port")
		}
	}
	if cm.Library == "" || !DirectoryExists(cm.Library) {
		return errors.New("A valid MPD Library path must be provided")
	}
	return nil
}

type ConfigFilter struct {
	Name                string   `yaml:"name"`
	Artist              []string `yaml:"artist"`
	ExcludedArtist      []string `yaml:"excluded_artist"`
	Year                []int    `yaml:"year"`
	EditionYear         []int    `yaml:"edition_year"`
	RecordLabel         []string `yaml:"record_label"`
	TagsIncluded        []string `yaml:"included_tags"`
	TagsExcluded        []string `yaml:"excluded_tags"`
	ReleaseType         []string `yaml:"type"`
	ExcludedReleaseType []string `yaml:"excluded_type"`
	Edition             []string `yaml:"edition"`
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
	Uploader            []string `yaml:"uploader"`
	RejectUnknown       bool     `yaml:"reject_unknown_releases"`
}

func (cf *ConfigFilter) Check() error {
	if cf.Name == "" {
		return errors.New("Missing filter name")
	}
	if (cf.HasCue || cf.HasLog || cf.LogScore != 0) && !StringInSlice(sourceCD, cf.Source) {
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
		return errors.New("The same artist cannot be both included and excluded")
	}
	if CommonInStringSlices(cf.TagsExcluded, cf.TagsIncluded) != nil {
		return errors.New("The same tag cannot be both included and excluded")
	}
	if len(cf.ExcludedReleaseType) != 0 && len(cf.ReleaseType) != 0 {
		return errors.New("Release types should be either included or excluded, not both")
	}
	if cf.UniqueInGroup && cf.AllowDuplicates {
		return errors.New("Filter can both allow duplicates and only allow one snatch/torrentgroup")
	}
	if cf.PerfectFlac {
		if cf.Format != nil || cf.Quality != nil || cf.Source != nil || cf.HasLog || cf.HasCue || cf.LogScore != 0 {
			return errors.New("The perfect_flag option replaces all options about quality, source, format, and cue/log/log score")
		}
		// setting the relevant options
		cf.Format = []string{formatFLAC}
		cf.Quality = []string{quality24bitLossless, qualityLossless}
		cf.HasCue = true
		cf.HasLog = true
		cf.LogScore = 100
		cf.Source = knownSources
	}
	if reflect.DeepEqual(*cf, ConfigFilter{Name: cf.Name}) {
		return errors.New("Empty filter would snatch everything, it probably is not what you want")
	}
	if len(cf.Year) != 0 && len(cf.EditionYear) != 0 {
		return errors.New("A filter can define year or edition_year, but not both")
	}

	// checking against known gazelle values
	if len(cf.ReleaseType) != 0 {
		for _, r := range cf.ReleaseType {
			if !StringInSlice(r, knownReleaseTypes) {
				return errors.New("unknown release type " + r + ", acceptable values: " + strings.Join(knownReleaseTypes, ", "))
			}
		}
	}
	if len(cf.ExcludedReleaseType) != 0 {
		for _, r := range cf.ExcludedReleaseType {
			if !StringInSlice(r, knownReleaseTypes) {
				return errors.New("unknown release type " + r + ", acceptable values: " + strings.Join(knownReleaseTypes, ", "))
			}
		}
	}
	if len(cf.Format) != 0 {
		for _, r := range cf.Format {
			if !StringInSlice(r, knownFormats) {
				return errors.New("unknown format " + r + ", acceptable values: " + strings.Join(knownFormats, ", "))
			}
		}
	}
	if len(cf.Source) != 0 {
		for _, r := range cf.Source {
			if !StringInSlice(r, knownSources) {
				return errors.New("unknown source " + r + ", acceptable values: " + strings.Join(knownSources, ", "))
			}
		}
	}
	if len(cf.Quality) != 0 {
		for _, r := range cf.Quality {
			if !StringInSlice(r, knownQualities) {
				return errors.New("unknown quality " + r + ", acceptable values: " + strings.Join(knownQualities, ", "))
			}
		}
	}

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
	description += "\tHas Cue: " + fmt.Sprintf("%v", cf.HasCue) + "\n"
	description += "\tHas Log: " + fmt.Sprintf("%v", cf.HasLog) + "\n"
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
	description += "\tUnique in Group: " + fmt.Sprintf("%v", cf.UniqueInGroup) + "\n"
	if len(cf.Tracker) != 0 {
		description += "\tTracker(s): " + strings.Join(cf.Tracker, ", ") + "\n"
	} else {
		description += "\tTracker(s): All\n"
	}
	if len(cf.Uploader) != 0 {
		description += "\tUploader(s): " + strings.Join(cf.Uploader, ", ") + "\n"
	}
	if len(cf.Edition) != 0 {
		description += "\tEdition contains: " + strings.Join(cf.Edition, ", ") + "\n"
	}
	if len(cf.EditionYear) != 0 {
		description += "\tEdition Year(s): " + strings.Join(IntSliceToStringSlice(cf.EditionYear), ", ") + "\n"
	}
	description += "\tReject unknown releases: " + fmt.Sprintf("%v", cf.RejectUnknown) + "\n"
	return description
}
