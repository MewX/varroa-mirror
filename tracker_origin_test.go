package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTrackerOriginJSON(t *testing.T) {
	fmt.Println("+ Testing TrackerOriginJSON...")
	check := assert.New(t)

	// setting up
	testDir := "test"
	env := &Environment{}
	c := &Config{}
	tr := &ConfigTracker{Name: "tracker1", URL: "http://azerty.com"}
	tr2 := &ConfigTracker{Name: "tracker2", URL: "http://qwerty.com"}
	c.Trackers = append(c.Trackers, tr, tr2)
	env.config = c
	info1 := TrackerTorrentInfo{id: 1234, groupID: 11}
	info2 := TrackerTorrentInfo{id: 123456, groupID: 12}
	destFile := filepath.Join(testDir, "test_origin.json")
	tracker1 := &GazelleTracker{Name: "tracker1", URL: "http://azerty.com"}
	tracker2 := &GazelleTracker{Name: "tracker2", URL: "http://qwerty.com"}

	// saving origin JSON to file
	toj := &TrackerOriginJSON{}
	check.False(FileExists(destFile))
	err := toj.Save(destFile, tracker1, info1)
	check.Nil(err)
	check.True(FileExists(destFile))
	check.Equal(info1.id, toj.Origins[tracker1.Name].ID)
	check.Equal(info1.groupID, toj.Origins[tracker1.Name].GroupID)
	check.NotEqual(0, toj.Origins[tracker1.Name].TimeSnatched)
	check.NotEqual(0, toj.Origins[tracker1.Name].LastUpdatedMetadata)
	err = toj.Save(destFile, tracker2, info2)
	check.Nil(err)
	check.True(FileExists(destFile))
	check.Equal(info2.id, toj.Origins[tracker2.Name].ID)
	check.Equal(info2.groupID, toj.Origins[tracker2.Name].GroupID)
	check.NotEqual(0, toj.Origins[tracker2.Name].TimeSnatched)
	check.NotEqual(0, toj.Origins[tracker2.Name].LastUpdatedMetadata)

	defer os.Remove(destFile)

	// reading file that was created and comparing with expected
	b, err := ioutil.ReadFile(destFile)
	check.Nil(err)
	var tojCheck TrackerOriginJSON
	err = json.Unmarshal(b, &tojCheck)
	check.Nil(err)
	check.Equal(toj.Origins[tracker1.Name].ID, tojCheck.Origins[tracker1.Name].ID)
	check.Equal(toj.Origins[tracker1.Name].GroupID, tojCheck.Origins[tracker1.Name].GroupID)
	check.Equal(env.config.Trackers[0].URL, tojCheck.Origins[tracker1.Name].Tracker)
	check.True(tojCheck.Origins[tracker1.Name].IsAlive)
	check.Equal(toj.Origins[tracker1.Name].TimeSnatched, tojCheck.Origins[tracker1.Name].TimeSnatched)
	check.Equal(toj.Origins[tracker1.Name].LastUpdatedMetadata, tojCheck.Origins[tracker1.Name].LastUpdatedMetadata)

	check.Equal(toj.Origins[tracker2.Name].ID, tojCheck.Origins[tracker2.Name].ID)
	check.Equal(toj.Origins[tracker2.Name].GroupID, tojCheck.Origins[tracker2.Name].GroupID)
	check.Equal(env.config.Trackers[1].URL, tojCheck.Origins[tracker2.Name].Tracker)
	check.True(tojCheck.Origins[tracker2.Name].IsAlive)
	check.Equal(toj.Origins[tracker2.Name].TimeSnatched, tojCheck.Origins[tracker2.Name].TimeSnatched)
	check.Equal(toj.Origins[tracker2.Name].LastUpdatedMetadata, tojCheck.Origins[tracker2.Name].LastUpdatedMetadata)

	// update
	time.Sleep(time.Second * 1)
	lastUpdated := toj.Origins[tracker1.Name].LastUpdatedMetadata
	err = toj.Save(destFile, tracker1, info1)
	check.Nil(err)
	check.NotEqual(lastUpdated, toj.Origins[tracker1.Name].LastUpdatedMetadata)
	check.True(lastUpdated < toj.Origins[tracker1.Name].LastUpdatedMetadata)

}
