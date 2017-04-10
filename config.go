package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Filter struct {
	label             string
	year              []int
	source            []string
	format            []string
	releaseType       []string
	artist            []string
	quality           []string
	destinationFolder string
	logScore          int
	recordLabel       []string
	hasLog            bool
	hasCue            bool
	allowScene        bool
	allowDuplicate    bool

	size struct {
		min uint64
		max uint64
	}
	tags struct {
		included []string
		excluded []string
	}
}

func (f Filter) String() string {
	description := f.label + ":\n"
	if len(f.year) != 0 {
		description += "\tYear(s): " + strings.Join(IntSliceToStringSlice(f.year), ", ") + "\n"
	}
	if len(f.artist) != 0 {
		description += "\tArtist(s): " + strings.Join(f.artist, ", ") + "\n"
	}
	if len(f.recordLabel) != 0 {
		description += "\tRecord Label(s): " + strings.Join(f.recordLabel, ", ") + "\n"
	}
	if len(f.tags.included) != 0 {
		description += "\tRequired tags: " + strings.Join(f.tags.included, ", ") + "\n"
	}
	if len(f.tags.excluded) != 0 {
		description += "\tExcluded tags: " + strings.Join(f.tags.excluded, ", ") + "\n"
	}
	if len(f.source) != 0 {
		description += "\tSource(s): " + strings.Join(f.source, ", ") + "\n"
	}
	if len(f.format) != 0 {
		description += "\tFormat(s): " + strings.Join(f.format, ", ") + "\n"
	}
	if len(f.quality) != 0 {
		description += "\tQuality: " + strings.Join(f.quality, ", ") + "\n"
	}
	if len(f.releaseType) != 0 {
		description += "\tType(s): " + strings.Join(f.releaseType, ", ") + "\n"
	}
	if f.hasCue {
		description += "\tHas Cue: true\n"
	}
	if f.hasLog {
		description += "\tHas Log: true\n"
	}
	if f.logScore != 0 {
		description += "\tMinimum Log Score: " + strconv.Itoa(f.logScore) + "\n"
	}
	if f.allowScene {
		description += "\tAllow Scene releases: true\n"
	}
	if f.allowDuplicate {
		description += "\tAllow duplicates: true\n"
	}
	if f.size.min != 0 {
		description += "\tMinimum Size: " + strconv.FormatUint(f.size.min, 10) + "\n"
	}
	if f.size.max != 0 {
		description += "\tMaximum Size: " + strconv.FormatUint(f.size.max, 10) + "\n"
	}

	if f.destinationFolder != "" {
		description += "\tSpecial destination folder: " + f.destinationFolder + "\n"
	}
	return description
}

type Config struct {
	filters                     []Filter
	url                         string
	user                        string
	password                    string
	blacklistedUploaders        []string
	statsUpdatePeriod           int
	maxBufferDecreaseByPeriodMB int
	irc                         struct {
		server           string
		key              string
		SSL              bool
		SSLSkipVerify    bool
		nickServPassword string
		botName          string
		announcer        string
		announceChannel  string
	}
	pushover struct {
		token string
		user  string
	}
	defaultDestinationFolder string
	downloadFolder           string
	gitlab                   struct {
		user        string
		password    string
		pagesGitURL string
		pagesURL    string
	}
	webServer struct {
		serveStats     bool
		statsPassword  string
		allowDownloads bool
		token          string
		hostname       string
		portHTTP       int
		portHTTPS      int
	}
	logLevel int
}

func getStringValues(source map[string]interface{}, key string) []string {
	result := []string{}
	if value, ok := source[key]; ok {
		switch value.(type) {
		case string:
			result = append(result, value.(string))
		case []interface{}:
			for _, el := range value.([]interface{}) {
				result = append(result, el.(string))
			}
		}
	}
	return result
}

func (c *Config) load(path string) error {
	conf := viper.New()
	conf.SetConfigType("yaml")
	conf.SetConfigFile(path)

	if err := conf.ReadInConfig(); err != nil {
		return err
	}

	// tracker configuration
	c.url = conf.GetString("tracker.url")
	c.user = conf.GetString("tracker.user")
	c.password = conf.GetString("tracker.password")
	c.blacklistedUploaders = conf.GetStringSlice("tracker.blacklisted_uploaders")
	c.statsUpdatePeriod = conf.GetInt("tracker.stats_update_period_hour")
	if c.statsUpdatePeriod < 1 {
		return errors.New("Period must be at least 1 hour")
	}
	c.maxBufferDecreaseByPeriodMB = conf.GetInt("tracker.max_buffer_decrease_by_period_mb")
	if c.statsUpdatePeriod < 1 {
		return errors.New("Max buffer decrease must be at least 1MB.")
	}
	// IRC configuration
	c.irc.server = conf.GetString("tracker.irc_server")
	c.irc.key = conf.GetString("tracker.irc_key")
	c.irc.SSL = conf.GetBool("tracker.irc_ssl")
	c.irc.SSLSkipVerify = conf.GetBool("tracker.irc_ssl_skip_verify")
	c.irc.nickServPassword = conf.GetString("tracker.nickserv_password")
	c.irc.botName = conf.GetString("tracker.bot_name")
	c.irc.announcer = conf.GetString("tracker.announcer")
	c.irc.announceChannel = conf.GetString("tracker.announce_channel")
	// folder configuration
	c.defaultDestinationFolder = conf.GetString("tracker.default_destination_folder")
	if c.defaultDestinationFolder == "" || !DirectoryExists(c.defaultDestinationFolder) {
		return errors.New("Default destination folder does not exist")
	}
	c.downloadFolder = conf.GetString("tracker.download_folder")
	if c.downloadFolder != "" && !DirectoryExists(c.downloadFolder) {
		return errors.New("Download folder does not exist")
	}
	// logging configuration
	c.logLevel = conf.GetInt("tracker.log_level")
	// pushover configuration
	c.pushover.token = conf.GetString("pushover.token")
	c.pushover.user = conf.GetString("pushover.user")
	// gitlab pages configuration
	c.gitlab.pagesGitURL = conf.GetString("gitlab.git")
	c.gitlab.user = conf.GetString("gitlab.user")
	c.gitlab.password = conf.GetString("gitlab.password")
	if c.gitlabPagesConfigured() {
		repoNameParts := strings.Split(c.gitlab.pagesGitURL, "/")
		c.gitlab.pagesURL = fmt.Sprintf("https://%s.gitlab.io/%s", c.gitlab.user, strings.Replace(repoNameParts[len(repoNameParts)-1], ".git", "", -1))
	}
	// web server configuration
	c.webServer.allowDownloads = conf.GetBool("webserver.allow_downloads")
	c.webServer.serveStats = conf.GetBool("webserver.serve_stats")
	c.webServer.statsPassword = conf.GetString("webserver.stats_password")
	c.webServer.token = conf.GetString("webserver.token")
	c.webServer.hostname = conf.GetString("webserver.hostname")
	c.webServer.portHTTP = conf.GetInt("webserver.http_port")
	c.webServer.portHTTPS = conf.GetInt("webserver.https_port")
	// filter configuration
	for filter, info := range conf.GetStringMap("filters") {
		t := Filter{label: filter}
		tinfo := info.(map[string]interface{})

		if year, ok := tinfo["year"]; ok {
			switch year.(type) {
			case int:
				t.year = append(t.year, year.(int))
			case []interface{}:
				for _, el := range year.([]interface{}) {
					t.year = append(t.year, el.(int))
				}
			}
		}
		t.source = getStringValues(tinfo, "source")
		t.format = getStringValues(tinfo, "format")
		t.releaseType = getStringValues(tinfo, "type")
		t.artist = getStringValues(tinfo, "artist")
		t.tags.included = getStringValues(tinfo, "included_tags")
		t.tags.excluded = getStringValues(tinfo, "excluded_tags")
		t.recordLabel = getStringValues(tinfo, "record_label")
		if destination, ok := tinfo["destination"]; ok {
			t.destinationFolder = destination.(string)
			if !DirectoryExists(t.destinationFolder) {
				return errors.New(t.destinationFolder + " does not exist")
			}
		}
		if maxSize, ok := tinfo["max_size_mb"]; ok {
			t.size.max = uint64(maxSize.(int))
		}
		if minSize, ok := tinfo["min_size_mb"]; ok {
			t.size.min = uint64(minSize.(int))
		}
		if logScore, ok := tinfo["log_score"]; ok {
			t.logScore = logScore.(int)
		}
		if hasLog, ok := tinfo["has_log"]; ok {
			t.hasLog = hasLog.(bool)
		}
		if hasCue, ok := tinfo["has_cue"]; ok {
			t.hasCue = hasCue.(bool)
		}
		if allowScene, ok := tinfo["allow_scene"]; ok {
			t.allowScene = allowScene.(bool)
		}
		t.allowDuplicate = true // by default, accept duplicates
		if allowDuplicate, ok := tinfo["allow_duplicate"]; ok {
			t.allowDuplicate = allowDuplicate.(bool)
		}
		// special option which forces filter settings
		if perfectFlac, ok := tinfo["perfect_flac"]; ok {
			if perfectFlac.(bool) {
				// set all options that make a perfect flac
				// ie: 16bit/24bit FLAC 100%/log/cue/CD, or any Vinyl,DVD,Soundboard,WEB,Cassette,Blu-ray,SACD,DAT
				t.format = []string{"FLAC"}
				t.quality = []string{"Lossless", "24bit Lossless"}
				t.hasLog = true
				t.hasCue = true
				t.logScore = 100
				t.source = []string{"CD", "Vinyl", "DVD", "Soundboard", "WEB", "Cassette", "Blu-ray", "SACD", "DAT"}
			}
		}
		c.filters = append(c.filters, t)
	}
	return nil
}

func (c *Config) pushoverConfigured() bool {
	if c.pushover.user != "" && c.pushover.token != "" {
		return true
	}
	return false
}

func (c *Config) gitlabPagesConfigured() bool {
	if c.gitlab.pagesGitURL != "" && c.gitlab.user != "" && c.gitlab.password != "" {
		return true
	}
	return false
}

func (c *Config) downloadFolderConfigured() bool {
	if c.downloadFolder != "" && DirectoryExists(c.downloadFolder) {
		return true
	}
	return false
}

func (c *Config) webserverConfigured() bool {
	return c.serveHTTP() || c.serveHTTPS()
}

func (c *Config) serveHTTP() bool {
	// valid http port, and at least one feature (serving stats and allowing downloads) is enabled, and we have a token
	if c.webServer.portHTTP > 1024 && (c.webServer.serveStats || c.webServer.allowDownloads) && c.webServer.token != "" {
		return true
	}
	return false
}

func (c *Config) serveHTTPS() bool {
	// valid https port, and at least one feature (serving stats and allowing downloads) is enabled, and we have a token and hostname
	if c.webServer.portHTTPS > 1024 && (c.webServer.serveStats || c.webServer.allowDownloads) && c.webServer.token != "" && c.webServer.hostname != "" {
		return true
	}
	return false
}
