package main

import (
	"github.com/spf13/viper"
)

type Filter struct {
	label       string
	year        []int
	source      []string
	format      []string
	releaseType []string
	artist      []string
	quality     []string
	// TODO
}

type Config struct {
	filters          []Filter
	url              string
	user             string
	password         string
	ircServer        string
	ircKey           string
	nickServPassword string
	botName          string
	announcer        string
	announceChannel  string
	pushoverToken    string
	pushoverUser     string
}

func (c *Config) load(path string) (err error) {

	conf := viper.New()
	conf.SetConfigType("yaml")
	conf.SetConfigFile(path)

	err = conf.ReadInConfig()
	if err != nil {
		return
	}

	c.url = conf.GetString("tracker.url")
	c.user = conf.GetString("tracker.user")
	c.password = conf.GetString("tracker.password")
	c.ircServer = conf.GetString("tracker.irc_server")
	c.ircKey = conf.GetString("tracker.irc_key")
	c.nickServPassword = conf.GetString("tracker.nickserv_password")
	c.botName = conf.GetString("tracker.bot_name")
	c.announcer = conf.GetString("tracker.announcer")
	c.announceChannel = conf.GetString("tracker.announce_channel")
	c.pushoverToken = conf.GetString("pushover.token")
	c.pushoverUser = conf.GetString("pushover.user")

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

		// TODO : tags: include/exclude ; maxsize ; destination folder for filters; quality

		c.filters = append(c.filters, t)
	}
	return
}
