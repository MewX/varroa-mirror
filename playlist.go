package varroa

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/catastrophic/assistance/fs"
	"gitlab.com/catastrophic/assistance/strslice"
)

type Playlist struct {
	Filename string
	Contents []string
}

func CurrentPlaylists(root string) (*Playlist, *Playlist, error) {
	daily := &Playlist{}
	monthly := &Playlist{}
	// generate name
	now := time.Now().Local()
	thisMonth := now.Format("2006-01")
	thisDay := now.Format("2006-01-02")

	daily.Filename = filepath.Join(root, thisDay+m3uExt)
	monthly.Filename = filepath.Join(root, thisMonth+m3uExt)
	// if it exists, parse and return
	if fs.FileExists(daily.Filename) {
		if err := daily.Load(daily.Filename); err != nil {
			return nil, nil, err
		}
	}
	if fs.FileExists(monthly.Filename) {
		if err := monthly.Load(monthly.Filename); err != nil {
			return nil, nil, err
		}
	}
	return daily, monthly, nil
}

func AddReleaseToCurrentPlaylists(playlistDirectory, libraryDirectory, release string) error {
	// daily playlist
	dailyPlaylist, monthlyPlaylist, err := CurrentPlaylists(playlistDirectory)
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

func (p *Playlist) Load(filename string) error {
	if !fs.FileExists(filename) {
		return errors.New("playlist " + filename + " does not exist")
	}
	p.Filename = filename
	// open file and get strings
	content, err := ioutil.ReadFile(p.Filename)
	if err != nil {
		return err
	}
	p.Contents = strings.Split(string(content), "\n")
	return nil
}

func (p *Playlist) AddRelease(root, path string) error {
	if !DirectoryContainsMusicAndMetadata(filepath.Join(root, path)) {
		return fmt.Errorf(ErrorFindingMusicAndMetadata, path)
	}

	// walk path and list all music files, get relative to library directory
	e := filepath.Walk(filepath.Join(root, path), func(subPath string, fileInfo os.FileInfo, walkError error) error {
		if os.IsNotExist(walkError) {
			return nil
		}
		// load all music files
		if strslice.Contains([]string{flacExt, mp3Ext}, filepath.Ext(subPath)) {
			// MPD wants relative paths
			relativePath, err := filepath.Rel(root, subPath)
			if err != nil {
				return err
			}
			p.Contents = append(p.Contents, relativePath)
		}
		return nil
	})
	return e
}

func (p *Playlist) Save() error {
	// write if everything is good.
	return ioutil.WriteFile(p.Filename, []byte(strings.Join(p.Contents, "\n")), 0777)
}

func (p *Playlist) Contains(release string) bool {
	// assumes that release is the release folder, relative to the library directory, not a parent.
	for _, f := range p.Contents {
		if strings.HasPrefix(f, release) {
			return true
		}
	}
	return false
}

func (p *Playlist) Update(old, new string) {
	// assumes that release is the release folder, relative to the library directory, not a parent.
	for i, f := range p.Contents {
		if strings.HasPrefix(f, old) {
			p.Contents[i] = strings.Replace(f, old, new, -1)
		}
	}
}
