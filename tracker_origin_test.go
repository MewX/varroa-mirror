package varroa

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

	c, err := NewConfig("test/test_complete.yaml")
	check.Nil(err)
	c.General.DownloadDir = testDir
	tr := &ConfigTracker{Name: "tracker1", URL: "http://azerty.com"}
	tr2 := &ConfigTracker{Name: "tracker2", URL: "http://qwerty.com"}
	c.Trackers = append(config.Trackers, tr, tr2)
	env.config = c
	tracker1 := &GazelleTracker{Name: "tracker1", URL: "http://azerty.com"}
	tracker2 := &GazelleTracker{Name: "tracker2", URL: "http://qwerty.com"}
	info1 := TrackerMetadata{ID: 1234, GroupID: 11, Tracker: tracker1.Name, TrackerURL: tracker1.URL, LastUpdated: 1, IsAlive: true}
	info2 := TrackerMetadata{ID: 1234, GroupID: 12, Tracker: tracker2.Name, TrackerURL: tracker2.URL}

	// make directory
	check.Nil(os.MkdirAll(filepath.Join(testDir, metadataDir), 0775))
	defer os.Remove(filepath.Join(testDir, metadataDir))
	expectedFilePath := filepath.Join(testDir, metadataDir, originJSONFile)
	defer os.Remove(expectedFilePath)

	// saving origin JSON to file
	check.False(FileExists(expectedFilePath))
	check.Nil(info1.saveOriginJSON())
	check.True(FileExists(expectedFilePath))
	check.Nil(info2.saveOriginJSON())

	// reading file that was created and comparing with expected
	b, err := ioutil.ReadFile(expectedFilePath)
	check.Nil(err)
	var tojCheck TrackerOriginJSON
	check.Nil(json.Unmarshal(b, &tojCheck))
	check.Equal(info1.ID, tojCheck.Origins[tracker1.Name].ID)
	check.Equal(info1.GroupID, tojCheck.Origins[tracker1.Name].GroupID)
	check.Equal(info1.TrackerURL, tojCheck.Origins[tracker1.Name].Tracker)
	check.True(tojCheck.Origins[tracker1.Name].IsAlive)
	check.Equal(info1.TimeSnatched, tojCheck.Origins[tracker1.Name].TimeSnatched)
	check.Equal(info1.LastUpdated, tojCheck.Origins[tracker1.Name].LastUpdatedMetadata)

	check.Equal(info2.ID, tojCheck.Origins[tracker2.Name].ID)
	check.Equal(info2.GroupID, tojCheck.Origins[tracker2.Name].GroupID)
	check.Equal(info2.TrackerURL, tojCheck.Origins[tracker2.Name].Tracker)
	check.False(tojCheck.Origins[tracker2.Name].IsAlive)
	check.Equal(info2.TimeSnatched, tojCheck.Origins[tracker2.Name].TimeSnatched)
	check.Equal(info2.LastUpdated, tojCheck.Origins[tracker2.Name].LastUpdatedMetadata)

	// update
	info1.LastUpdated = 2
	check.Nil(info1.saveOriginJSON())

	// read from file again
	b, err = ioutil.ReadFile(expectedFilePath)
	check.Nil(err)
	check.Nil(json.Unmarshal(b, &tojCheck))
	check.Equal(info1.LastUpdated, tojCheck.Origins[tracker1.Name].LastUpdatedMetadata)
}
