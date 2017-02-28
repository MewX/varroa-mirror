package main

import (
	"errors"

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
	maxSize           uint64
	logScore          int
	recordLabel       []string
	hasLog            bool
	hasCue            bool
	allowScene        bool
}

type Config struct {
	filters                     []Filter
	url                         string
	user                        string
	password                    string
	ircServer                   string
	ircKey                      string
	ircSSL                      bool
	nickServPassword            string
	botName                     string
	announcer                   string
	announceChannel             string
	pushoverToken               string
	pushoverUser                string
	statsFile                   string
	statsUpdatePeriod           int
	maxBufferDecreaseByPeriodMB int
	defaultDestinationFolder    string
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

func (c *Config) load(path string) (err error) {
	conf := viper.New()
	conf.SetConfigType("yaml")
	conf.SetConfigFile(path)

	err = conf.ReadInConfig()
	if err != nil {
		return
	}

	// tracker configuration
	c.url = conf.GetString("tracker.url")
	c.user = conf.GetString("tracker.user")
	c.password = conf.GetString("tracker.password")
	c.ircServer = conf.GetString("tracker.irc_server")
	c.ircKey = conf.GetString("tracker.irc_key")
	c.ircSSL = conf.GetBool("tracker.irc_ssl")
	c.nickServPassword = conf.GetString("tracker.nickserv_password")
	c.botName = conf.GetString("tracker.bot_name")
	c.announcer = conf.GetString("tracker.announcer")
	c.announceChannel = conf.GetString("tracker.announce_channel")
	c.statsFile = conf.GetString("tracker.stats_file")
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
	// pushover configuration
	c.pushoverToken = conf.GetString("pushover.token")
	c.pushoverUser = conf.GetString("pushover.user")
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
		c.filters = append(c.filters, t)
	}
	return
}

func (c *Config) pushoverConfigured() bool {
	if conf.pushoverUser != "" && conf.pushoverToken != "" {
		return true
	}
	return false
}
