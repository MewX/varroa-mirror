package main

import (
	"errors"
	"fmt"
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
	includedTags      []string
	excludedTags      []string
	destinationFolder string
	minSize           uint64
	maxSize           uint64
	logScore          int
	recordLabel       []string
	hasLog            bool
	hasCue            bool
	allowScene        bool
	allowDuplicate    bool
}

type Config struct {
	filters                     []Filter
	url                         string
	user                        string
	password                    string
	ircServer                   string
	ircKey                      string
	ircSSL                      bool
	ircSSLSkipVerify            bool
	nickServPassword            string
	botName                     string
	announcer                   string
	announceChannel             string
	pushoverToken               string
	pushoverUser                string
	statsUpdatePeriod           int
	maxBufferDecreaseByPeriodMB int
	defaultDestinationFolder    string
	blacklistedUploaders        []string
	logLevel                    int
	gitlabUser                  string
	gitlabPassword              string
	gitlabPagesGitURL           string
	gitlabPagesURL              string
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
	c.ircServer = conf.GetString("tracker.irc_server")
	c.ircKey = conf.GetString("tracker.irc_key")
	c.ircSSL = conf.GetBool("tracker.irc_ssl")
	c.ircSSLSkipVerify = conf.GetBool("tracker.irc_ssl_skip_verify")
	c.nickServPassword = conf.GetString("tracker.nickserv_password")
	c.botName = conf.GetString("tracker.bot_name")
	c.announcer = conf.GetString("tracker.announcer")
	c.announceChannel = conf.GetString("tracker.announce_channel")
	c.statsUpdatePeriod = conf.GetInt("tracker.stats_update_period_hour")
	if c.statsUpdatePeriod < 1 {
		return errors.New("Period must be at least 1 hour")
	}
	c.maxBufferDecreaseByPeriodMB = conf.GetInt("tracker.max_buffer_decrease_by_period_mb")
	if c.statsUpdatePeriod < 1 {
		return errors.New("Max buffer decrease must be at least 1MB.")
	}
	c.defaultDestinationFolder = conf.GetString("tracker.default_destination_folder")
	if c.defaultDestinationFolder == "" || !DirectoryExists(c.defaultDestinationFolder) {
		return errors.New("Default destination folder does not exist")
	}
	c.blacklistedUploaders = conf.GetStringSlice("tracker.blacklisted_uploaders")
	c.logLevel = conf.GetInt("tracker.log_level")
	// pushover configuration
	c.pushoverToken = conf.GetString("pushover.token")
	c.pushoverUser = conf.GetString("pushover.user")
	// gitlab pages configuration
	c.gitlabPagesGitURL = conf.GetString("gitlab.git")
	c.gitlabUser = conf.GetString("gitlab.user")
	c.gitlabPassword = conf.GetString("gitlab.password")
	if c.gitlabPagesConfigured() {
		repoNameParts := strings.Split(c.gitlabPagesGitURL, "/")
		c.gitlabPagesURL = fmt.Sprintf("https://%s.gitlab.io/%s", c.gitlabUser, strings.Replace(repoNameParts[len(repoNameParts)-1], ".git", "", -1))
	}
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
		t.includedTags = getStringValues(tinfo, "included_tags")
		t.excludedTags = getStringValues(tinfo, "excluded_tags")
		t.recordLabel = getStringValues(tinfo, "record_label")
		if destination, ok := tinfo["destination"]; ok {
			t.destinationFolder = destination.(string)
			if !DirectoryExists(t.destinationFolder) {
				return errors.New(t.destinationFolder + " does not exist")
			}
		}
		if maxSize, ok := tinfo["max_size_mb"]; ok {
			t.maxSize = uint64(maxSize.(int))
		}
		if minSize, ok := tinfo["min_size_mb"]; ok {
			t.minSize = uint64(minSize.(int))
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
	if c.pushoverUser != "" && c.pushoverToken != "" {
		return true
	}
	return false
}

func (c *Config) gitlabPagesConfigured() bool {
	if c.gitlabPagesGitURL != "" && c.gitlabUser != "" && c.gitlabPassword != "" {
		return true
	}
	return false
}
