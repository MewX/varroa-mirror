package varroa

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
	daemon "github.com/sevlyar/go-daemon"
)

// ReorganizeLibrary using tracker metadata and the user-defined template
func ReorganizeLibrary(doNothing bool) error {
	defer TimeTrack(time.Now(), "Reorganize Library")

	if doNothing {
		logThis.Info("Simulating library reorganization...", NORMAL)
	}

	c, e := NewConfig(DefaultConfigurationFile)
	if e != nil {
		return e
	}
	if !c.LibraryConfigured {
		return errors.New("library section of the configuration file not found")
	}

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = "Reorganizing library"
	if !daemon.WasReborn() {
		s.Start()
	}

	movedAlbums := 0
	template := defaultFolderTemplate
	if c.Library.Template != "" {
		template = c.Library.Template
	}

	var playlists []Playlist
	if c.playlistDirectoryConfigured {
		// load all playlists
		e = filepath.Walk(c.Library.PlaylistDirectory, func(path string, fileInfo os.FileInfo, walkError error) error {
			if os.IsNotExist(walkError) {
				return nil
			}
			// load all found playlists
			if filepath.Ext(path) == m3uExt {
				p := Playlist{}
				if err := p.Load(path); err != nil {
					logThis.Error(err, VERBOSE)
				} else {
					playlists = append(playlists, p)
				}
			}
			return nil
		})
		if e != nil {
			logThis.Error(e, NORMAL)
		}
	}

	walkErr := filepath.Walk(c.Library.Directory, func(path string, fileInfo os.FileInfo, walkError error) error {
		// when an album has just been moved, Walk goes through it a second
		// time with an "file does not exist" error
		if os.IsNotExist(walkError) {
			return nil
		}

		if fileInfo.IsDir() && DirectoryContainsMusicAndMetadata(path) {
			var libraryEntry DownloadEntry
			libraryEntry.FolderName = fileInfo.Name()
			// read information from metadata
			if err := libraryEntry.Load(filepath.Dir(path)); err != nil {
				logThis.Error(errors.Wrap(err, "Error: could not load metadata for "+fileInfo.Name()), VERBOSEST)
				return err
			}
			var newName string
			for _, t := range libraryEntry.Tracker {
				info, err := libraryEntry.getMetadata(filepath.Dir(path), t)
				if err != nil {
					logThis.Info("Could not find metadata for tracker "+t, NORMAL)
					continue
				}
				newName = info.GeneratePath(template, filepath.Dir(path))
				break // stop once we have a name.
			}

			if newName == "" {
				return errors.New("could not generate path for " + fileInfo.Name())
			}

			hasMoved, err := MoveToNewPath(path, filepath.Join(c.Library.Directory, newName), doNothing)
			if err != nil {
				return err
			}
			if hasMoved {
				movedAlbums += 1
				logThis.Info("Moved "+path+" -> "+newName, VERBOSE)
				if !doNothing && c.playlistDirectoryConfigured {
					relativePath, err := filepath.Rel(c.Library.Directory, path)
					if err != nil {
						return err
					}
					// find all playlists mentionning the release that was moved, update the path
					for _, p := range playlists {
						if p.Contains(relativePath) {
							// update the playlist
							p.Update(relativePath, newName)
							// save the new playlist
							if err := p.Save(); err != nil {
								logThis.Error(err, VERBOSE)
							}
						}
					}
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		logThis.Error(walkErr, NORMAL)
	}
	if !daemon.WasReborn() {
		s.Stop()
	}
	logThis.Info(fmt.Sprintf("Moved %d release(s).", movedAlbums), NORMAL)
	return DeleteEmptyLibraryFolders()
}

// DeleteEmptyLibraryFolders deletes empty folders that may appear after sorting albums.
func DeleteEmptyLibraryFolders() error {
	c, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		return err
	}
	if !c.LibraryConfigured {
		return errors.New("library section of the configuration file not found")
	}
	return DeleteEmptyFolders(c.Library.Directory)
}
