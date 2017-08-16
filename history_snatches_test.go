package varroa

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHistorySnatches(t *testing.T) {
	fmt.Println("\n --- Testing History snatches db. ---")
	check := assert.New(t)

	tempDBFile := "test/snatches.db"

	// setting up
	env1 := NewEnvironment()
	check.False(FileExists(tempDBFile))
	h1 := &History{Tracker: "label"}
	err := h1.SnatchHistory.Load(tempDBFile)
	check.Nil(err)
	env1.History["label"] = h1
	check.True(FileExists(tempDBFile))
	defer os.Remove(tempDBFile)

	// add one release to history
	err = env1.History["label"].AddSnatch(r1, "filter1")
	check.Nil(err)
	check.Equal(1, len(env1.History["label"].SnatchedReleases))
	check.Equal("filter1", env1.History["label"].SnatchedReleases[0].Filter)
	check.Equal("title", env1.History["label"].SnatchedReleases[0].Title)
	err = env1.History["label"].AddSnatch(r6, "filter6")
	check.Nil(err)
	check.Equal(2, len(env1.History["label"].SnatchedReleases))
	check.Equal("filter6", env1.History["label"].SnatchedReleases[1].Filter)
	check.Equal("title6", env1.History["label"].SnatchedReleases[1].Title)

	// testing searches
	// r1 & r2 are in the same groupID
	// t1 & r1Dupe are dupes
	check.True(env1.History["label"].HasReleaseFromGroup(r2))
	check.False(env1.History["label"].HasReleaseFromGroup(r3))
	check.True(env1.History["label"].HasReleaseFromGroup(r6Dupe))

	check.False(env1.History["label"].HasDupe(r2))
	check.True(env1.History["label"].HasDupe(r1Dupe))
	check.True(env1.History["label"].HasDupe(r6Dupe))

	// load from file, using 2nd History
	h2 := &History{Tracker: "label2"}
	err = h2.SnatchHistory.Load(tempDBFile)
	check.Nil(err)
	env1.History["label2"] = h2

	check.Equal(2, len(env1.History["label2"].SnatchedReleases))
	check.Equal("filter1", env1.History["label2"].SnatchedReleases[0].Filter)
	check.Equal("title", env1.History["label2"].SnatchedReleases[0].Title)
	check.Equal("filter6", env1.History["label2"].SnatchedReleases[1].Filter)
	check.Equal("title6", env1.History["label2"].SnatchedReleases[1].Title)

	// testing searches from loaded results
	check.True(env1.History["label2"].HasReleaseFromGroup(r2))
	check.False(env1.History["label2"].HasReleaseFromGroup(r3))
	check.True(env1.History["label2"].HasReleaseFromGroup(r6Dupe))

	check.False(env1.History["label2"].HasDupe(r2))
	check.True(env1.History["label2"].HasDupe(r1Dupe))
	check.True(env1.History["label2"].HasDupe(r6Dupe))
}
