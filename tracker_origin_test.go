package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrackerOriginJSON(t *testing.T) {
	fmt.Println("+ Testing TrackerOriginJSON...")
	check := assert.New(t)

	// setting up
	testDir := "test"
	env := &Environment{}
	c := &Config{}
	tr := &ConfigTracker{URL: "http://azerty.com"}
	c.Trackers = append(c.Trackers, tr)
	env.config = c
	info := TrackerTorrentInfo{id: 1234}
	destFile := filepath.Join(testDir, "test_origin.json")
	tracker := &GazelleTracker{URL: "http://azerty.com"}

	// saving origin JSON to file
	toj := &TrackerOriginJSON{}
	check.False(FileExists(destFile))
	err := toj.Save(destFile, tracker, info)
	check.Nil(err)
	check.True(FileExists(destFile))
	check.Equal(info.id, toj.ID)
	check.NotEqual(0, toj.TimeSnatched)
	check.NotEqual(0, toj.LastUpdatedMetadata)

	defer os.Remove(destFile)

	// reading file that was created and comparing with expected
	b, err := ioutil.ReadFile(destFile)
	check.Nil(err)
	var tojCheck TrackerOriginJSON
	err = json.Unmarshal(b, &tojCheck)
	check.Nil(err)
	check.Equal(toj.ID, tojCheck.ID)
	check.Equal(env.config.Trackers[0].URL, tojCheck.Tracker)
	check.True(tojCheck.IsAlive)
	check.Equal(toj.TimeSnatched, tojCheck.TimeSnatched)
	check.Equal(toj.LastUpdatedMetadata, tojCheck.LastUpdatedMetadata)
}
