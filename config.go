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
	// TODO
}

type Config struct {
	filters                     []Filter
	url                         string
	user                        string
	password                    string
	ircServer                   string
	ircKey                      string
	nickServPassword            string
	botName                     string
	announcer                   string
	announceChannel             string
	pushoverToken               string
	pushoverUser                string
	statsFile                   string
	statsUpdatePeriod           int
	maxBufferDecreaseByPeriodMB int
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
	c.nickServPassword = conf.GetString("tracker.nickserv_password")
	c.botName = conf.GetString("tracker.bot_name")
	c.announcer = conf.GetString("tracker.announcer")
	c.announceChannel = conf.GetString("tracker.announce_channel")
	c.statsFile = conf.GetString("tracker.stats_file")
	c.statsUpdatePeriod = conf.GetInt("tracker.stats_update_period_min")
	c.maxBufferDecreaseByPeriodMB = conf.GetInt("tracker.max_buffer_decrease_by_period_mb")
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
		if source, ok := tinfo["source"]; ok {
			switch source.(type) {
			case string:
				t.source = append(t.source, source.(string))
			case []interface{}:
				for _, el := range source.([]interface{}) {
					t.source = append(t.source, el.(string))
				}
			}
		}
		if format, ok := tinfo["format"]; ok {
			switch format.(type) {
			case string:
				t.format = append(t.format, format.(string))
			case []interface{}:
				for _, el := range format.([]interface{}) {
					t.format = append(t.format, el.(string))
				}
			}
		}
		if releaseType, ok := tinfo["type"]; ok {
			switch releaseType.(type) {
			case string:
				t.releaseType = append(t.releaseType, releaseType.(string))
			case []interface{}:
				for _, el := range releaseType.([]interface{}) {
					t.releaseType = append(t.releaseType, el.(string))
				}
			}
		}
		if artist, ok := tinfo["artist"]; ok {
			switch artist.(type) {
			case string:
				t.artist = append(t.artist, artist.(string))
			case []interface{}:
				for _, el := range artist.([]interface{}) {
					t.artist = append(t.artist, el.(string))
				}
			}
		}
		if destination, ok := tinfo["destination"]; ok {
			t.destinationFolder = destination.(string)
			if !DirectoryExists(t.destinationFolder) {
				return errors.New(t.destinationFolder + " does not exist")
			}
		}
		if maxSize, ok := tinfo["max_size_mb"]; ok {
			t.maxSize = uint64(maxSize.(int))
		}
		if included, ok := tinfo["included_tags"]; ok {
			switch included.(type) {
			case string:
				t.includedTags = append(t.includedTags, included.(string))
			case []interface{}:
				for _, el := range included.([]interface{}) {
					t.includedTags = append(t.includedTags, el.(string))
				}
			}
		}
		if excluded, ok := tinfo["excluded_tags"]; ok {
			switch excluded.(type) {
			case string:
				t.excludedTags = append(t.excludedTags, excluded.(string))
			case []interface{}:
				for _, el := range excluded.([]interface{}) {
					t.excludedTags = append(t.excludedTags, el.(string))
				}
			}
		}
		// TODO : quality

		c.filters = append(c.filters, t)
	}
	return
}
