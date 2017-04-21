package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	fmt.Println("+ Testing Config...")
	check := assert.New(t)

	c := &Config{}
	err := c.Load("test/test_complete.yaml")
	check.Nil(err)

	// general
	check.Equal("test", c.General.WatchDir)
	check.Equal("../varroa/test", c.General.DownloadDir)
	check.Equal(2, c.General.LogLevel)
	check.True(c.General.AutomaticMetadataRetrieval)

	// trackers
	fmt.Println("Checking trackers")
	check.Equal(2, len(c.Trackers))
	tr := c.Trackers[0]
	check.Equal("blue", tr.Name)
	check.Equal("username", tr.User)
	check.Equal("secretpassword", tr.Password)
	check.Equal("https://blue.ch", tr.URL)
	tr = c.Trackers[1]
	check.Equal("purple", tr.Name)
	check.Equal("username", tr.User)
	check.Equal("secretpassword", tr.Password)
	check.Equal("https://purple.cd", tr.URL)
	// autosnatch
	fmt.Println("Checking autosnatch")
	check.Equal(2, len(c.Autosnatch))
	a := c.Autosnatch[0]
	check.Equal("blue", a.Tracker)
	check.Equal("irc.server.net:6697", a.IRCServer)
	check.Equal("kkeeyy", a.IRCKey)
	check.True(a.IRCSSL)
	check.False(a.IRCSSLSkipVerify)
	check.Equal("something", a.NickservPassword)
	check.Equal("mybot", a.BotName)
	check.Equal("Bee", a.Announcer)
	check.Equal("#blue-announce", a.AnnounceChannel)
	check.Equal([]string{"AwfulUser"}, a.BlacklistedUploaders)
	a = c.Autosnatch[1]
	check.Equal("purple", a.Tracker)
	check.Equal("irc.server.cd:6697", a.IRCServer)
	check.Equal("kkeeyy!", a.IRCKey)
	check.True(a.IRCSSL)
	check.True(a.IRCSSLSkipVerify)
	check.Equal("somethingŁ", a.NickservPassword)
	check.Equal("bobot", a.BotName)
	check.Equal("bolivar", a.Announcer)
	check.Equal("#announce", a.AnnounceChannel)
	check.Nil(a.BlacklistedUploaders)
	// stats
	fmt.Println("Checking stats")
	check.Equal(2, len(c.Stats))
	s := c.Stats[0]
	check.Equal("blue", s.Tracker)
	check.Equal(1, s.UpdatePeriodH)
	check.Equal(500, s.MaxBufferDecreaseMB)
	s = c.Stats[1]
	check.Equal("purple", s.Tracker)
	check.Equal(12, s.UpdatePeriodH)
	check.Equal(2500, s.MaxBufferDecreaseMB)
	// webserver
	fmt.Println("Checking webserver")
	check.True(c.WebServer.ServeStats)
	check.True(c.WebServer.AllowDownloads)
	check.Equal("httppassword", c.WebServer.Password)
	check.Equal("httpuser", c.WebServer.User)
	check.Equal("thisisatoken", c.WebServer.Token)
	check.Equal("server.that.is.mine.com", c.WebServer.Hostname)
	check.Equal(1234, c.WebServer.PortHTTP)
	check.Equal(1235, c.WebServer.PortHTTPS)
	// notifications
	fmt.Println("Checking notifications")
	check.Equal("tokenpushovertoken", c.Notifications.Pushover.Token)
	check.Equal("userpushoveruser", c.Notifications.Pushover.User)
	// gitlab
	fmt.Println("Checking gitlab pages")
	check.Equal("https://gitlab.com/something/repo.git", c.GitlabPages.GitHTTPS)
	check.Equal("gitlabuser", c.GitlabPages.User)
	check.Equal("anotherpassword", c.GitlabPages.Password)
	check.Equal("https://something.gitlab.io/repo", c.GitlabPages.URL)
	// filters
	fmt.Println("Checking filters")
	check.Equal(2, len(c.Filters))
	fmt.Println("Checking filter 'perfect'")
	f := c.Filters[0]
	check.Equal("perfect", f.Name)
	check.Nil(f.Year)
	check.Equal([]string{"CD", "Vinyl", "DVD", "Soundboard", "WEB", "Cassette", "Blu-ray", "SACD", "DAT"}, f.Source)
	check.Equal([]string{"FLAC"}, f.Format)
	check.Equal([]string{"Lossless", "24bit Lossless"}, f.Quality)
	check.True(f.HasCue)
	check.True(f.HasLog)
	check.Equal(100, f.LogScore)
	check.False(f.AllowScene)
	check.False(f.AllowDuplicates)
	check.Nil(f.ReleaseType)
	check.Equal("", f.WatchDir)
	check.Equal(0, f.MinSizeMB)
	check.Equal(0, f.MaxSizeMB)
	check.Nil(f.TagsIncluded)
	check.Nil(f.TagsExcluded)
	check.Nil(f.Artist)
	check.Nil(f.RecordLabel)
	check.True(f.PerfectFlac)
	check.True(f.UniqueInGroup)
	fmt.Println("Checking filter 'test'")
	f = c.Filters[1]
	check.Equal("test", f.Name)
	check.Equal([]int{2016, 2017}, f.Year)
	check.Equal([]string{"CD", "WEB"}, f.Source)
	check.Equal([]string{"FLAC", "MP3"}, f.Format)
	check.Equal([]string{"Lossless", "24bit Lossless", "320", "V0 (VBR)"}, f.Quality)
	check.True(f.HasCue)
	check.True(f.HasLog)
	check.Equal(80, f.LogScore)
	check.True(f.AllowScene)
	check.True(f.AllowDuplicates)
	check.Equal([]string{"Album", "EP"}, f.ReleaseType)
	check.Equal("watch/f1", f.WatchDir)
	check.Equal(10, f.MinSizeMB)
	check.Equal(500, f.MaxSizeMB)
	check.Equal([]string{"hip.hop", "pop"}, f.TagsIncluded)
	check.Equal([]string{"metal"}, f.TagsExcluded)
	check.Equal([]string{"The Beatles"}, f.Artist)
	check.Equal([]string{"Warp"}, f.RecordLabel)
	check.False(f.PerfectFlac)
	check.False(f.UniqueInGroup)

	check.True(c.autosnatchConfigured)
	check.True(c.statsConfigured)
	check.True(c.webserverConfigured)
	check.True(c.gitlabPagesConfigured)
	check.True(c.notificationsConfigured)
	check.True(c.downloadFolderConfigured)
	check.True(c.webserverHTTP)
	check.True(c.webserverHTTPS)

	// quick testing of files that only use a few features
	c = &Config{}
	err = c.Load("test/test_nostats.yaml")
	check.Nil(err)
	check.True(c.autosnatchConfigured)
	check.False(c.statsConfigured)
	check.True(c.webserverConfigured)
	check.False(c.gitlabPagesConfigured)
	check.False(c.notificationsConfigured)
	check.True(c.downloadFolderConfigured)
	check.False(c.webserverHTTP)
	check.True(c.webserverHTTPS)

	c = &Config{}
	err = c.Load("test/test_nostatsnoweb.yaml")
	check.Nil(err)
	check.True(c.autosnatchConfigured)
	check.False(c.statsConfigured)
	check.False(c.webserverConfigured)
	check.False(c.gitlabPagesConfigured)
	check.False(c.notificationsConfigured)
	check.False(c.downloadFolderConfigured)
	check.False(c.webserverHTTP)
	check.False(c.webserverHTTPS)

	c = &Config{}
	err = c.Load("test/test_statsnoautosnatch.yaml")
	check.Nil(err)
	check.False(c.autosnatchConfigured)
	check.True(c.statsConfigured)
	check.True(c.webserverConfigured)
	check.True(c.gitlabPagesConfigured)
	check.True(c.notificationsConfigured)
	check.True(c.downloadFolderConfigured)
	check.True(c.webserverHTTP)
	check.True(c.webserverHTTPS)
}
