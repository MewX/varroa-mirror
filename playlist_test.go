package varroa

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPlaylist(t *testing.T) {
	fmt.Println("+ Testing Playlist...")
	check := assert.New(t)

	now := time.Now().Local()
	thisMonth := now.Format("2006-01")
	thisDay := now.Format("2006-01-02")

	fakeLibraryPath := "test/library"
	fakePlaylistPath := "test/playlists"
	fakeRelease := "Release"
	fakeReleaseMoved := "Polka/Artist (2000) Release"
	fakeFiles := []string{"01. Track1.flac", "02. Track2.mp3", "this.log"}

	// create test dir
	check.Nil(os.MkdirAll(filepath.Join(fakeLibraryPath, fakeRelease, MetadataDir), 0777))
	check.Nil(os.MkdirAll(fakePlaylistPath, 0777))
	// create dummy files
	check.Nil(ioutil.WriteFile(filepath.Join(fakeLibraryPath, fakeRelease, MetadataDir, OriginJSONFile), []byte("Nothing interesting."), 0777))
	for _, f := range fakeFiles {
		check.Nil(ioutil.WriteFile(filepath.Join(fakeLibraryPath, fakeRelease, f), []byte("Nothing interesting."), 0777))
	}
	// remove everything once the test is over
	defer os.RemoveAll(fakeLibraryPath)
	defer os.RemoveAll(fakePlaylistPath)

	// add release to playlists
	check.Nil(AddReleaseToCurrentPlaylists(fakePlaylistPath, fakeLibraryPath, fakeRelease))
	check.True(FileExists(filepath.Join(fakePlaylistPath, thisDay+m3uExt)))
	check.True(FileExists(filepath.Join(fakePlaylistPath, thisMonth+m3uExt)))

	// check contents
	p := Playlist{}
	check.Nil(p.Load(filepath.Join(fakePlaylistPath, thisDay+m3uExt)))
	check.Equal(filepath.Join(fakePlaylistPath, thisDay+m3uExt), p.Filename)
	check.Equal(2, len(p.Contents))
	check.Equal([]string{filepath.Join(fakeRelease, "01. Track1.flac"), filepath.Join(fakeRelease, "02. Track2.mp3")}, p.Contents)

	p2 := Playlist{}
	check.Nil(p2.Load(filepath.Join(fakePlaylistPath, thisMonth+m3uExt)))
	check.Equal(filepath.Join(fakePlaylistPath, thisMonth+m3uExt), p2.Filename)
	check.Equal(2, len(p2.Contents))
	check.Equal([]string{filepath.Join(fakeRelease, "01. Track1.flac"), filepath.Join(fakeRelease, "02. Track2.mp3")}, p2.Contents)

	// update
	p.Update(fakeRelease, fakeReleaseMoved)
	check.Nil(p.Save())

	p3 := Playlist{}
	check.Nil(p3.Load(filepath.Join(fakePlaylistPath, thisDay+m3uExt)))
	check.Equal(2, len(p3.Contents))
	check.Equal([]string{filepath.Join(fakeReleaseMoved, "01. Track1.flac"), filepath.Join(fakeReleaseMoved, "02. Track2.mp3")}, p.Contents)

	// contains
	check.True(p2.Contains(fakeRelease))
	check.False(p2.Contains(fakeReleaseMoved))
	check.False(p3.Contains(fakeRelease))
	check.True(p3.Contains(fakeReleaseMoved))
}
