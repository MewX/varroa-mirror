package varroa

import (
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/catastrophic/assistance/fs"
	"gitlab.com/catastrophic/assistance/m3u"
)

func getCurrentPlaylists(root string) (*m3u.Playlist, *m3u.Playlist, error) {
	c, e := NewConfig(DefaultConfigurationFile)
	if e != nil {
		return nil, nil, e
	}
	if !c.LibraryConfigured {
		return nil, nil, errors.New("library section of the configuration file not found")
	}

	var daily, monthly *m3u.Playlist
	var err error

	// generate name
	now := time.Now().Local()
	thisMonth := now.Format("2006-01")
	thisDay := now.Format("2006-01-02")

	dailyFilename := filepath.Join(root, thisDay+m3uExt)
	monthlyFilename := filepath.Join(root, thisMonth+m3uExt)

	// if it exists, parse and return
	if fs.FileExists(dailyFilename) {
		daily, err = m3u.New(dailyFilename)
		if err != nil {
			return nil, nil, err
		}
	} else {
		daily = &m3u.Playlist{Filename: dailyFilename}
	}
	// check if monthly playlist exists,
	if fs.FileExists(monthlyFilename) {
		monthly, err = m3u.New(monthlyFilename)
		if err != nil {
			return nil, nil, err
		}
	} else {
		monthly = &m3u.Playlist{Filename: monthlyFilename}
	}

	return daily, monthly, nil
}

func addReleaseToCurrentPlaylists(playlistDirectory, libraryDirectory, release string) error {
	// daily playlist
	dailyPlaylist, monthlyPlaylist, err := getCurrentPlaylists(playlistDirectory)
	if err != nil {
		return errors.Wrap(err, "error getting current playlists")
	}

	if err = dailyPlaylist.AddRelease(libraryDirectory, release); err != nil {
		return errors.Wrap(err, "error adding tracks to daily playlist")
	}
	if err = dailyPlaylist.Save(); err != nil {
		return errors.Wrap(err, "error saving daily playlist")
	}
	// monthly playlist
	if err = monthlyPlaylist.AddRelease(libraryDirectory, release); err != nil {
		return errors.Wrap(err, "error adding tracks to daily playlist")
	}
	if err = monthlyPlaylist.Save(); err != nil {
		return errors.Wrap(err, "error saving daily playlist")
	}
	return nil
}
