package varroa

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
	daemon "github.com/sevlyar/go-daemon"
	"gitlab.com/catastrophic/assistance/fs"
	"gitlab.com/catastrophic/assistance/logthis"
	"gitlab.com/catastrophic/assistance/m3u"
)

// ReorganizeLibrary using tracker metadata and the user-defined template
func ReorganizeLibrary(doNothing, interactive bool) error {
	defer TimeTrack(time.Now(), "Reorganize Library")

	if doNothing {
		logthis.Info("Simulating library reorganization...", logthis.NORMAL)
	}

	c, e := NewConfig(DefaultConfigurationFile)
	if e != nil {
		return e
	}
	if !c.LibraryConfigured {
		return errors.New("library section of the configuration file not found")
	}

	// display spinner if not interactive
	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = "Reorganizing library"
	if !interactive && !daemon.WasReborn() {
		s.Start()
	}

	movedAlbums := 0
	template := defaultFolderTemplate
	if c.Library.Template != "" {
		template = c.Library.Template
	}

	var playlists []m3u.Playlist
	if c.playlistDirectoryConfigured {
		// load all playlists
		e = filepath.Walk(c.Library.PlaylistDirectory, func(path string, fileInfo os.FileInfo, walkError error) error {
			if os.IsNotExist(walkError) {
				return nil
			}
			// load all found playlists
			if filepath.Ext(path) == m3uExt {
				p, err := m3u.New(path)
				if err != nil {
					logthis.Error(err, logthis.VERBOSE)
				} else {
					playlists = append(playlists, *p)
				}
			}
			return nil
		})
		if e != nil {
			logthis.Error(e, logthis.NORMAL)
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
				logthis.Error(errors.Wrap(err, "Error: could not load metadata for "+fileInfo.Name()), logthis.VERBOSEST)
				return err
			}
			var newName string
			for _, t := range libraryEntry.Tracker {
				info, err := libraryEntry.getMetadata(filepath.Dir(path), t)
				if err != nil {
					logthis.Info("Could not find metadata for tracker "+t, logthis.NORMAL)
					continue
				}
				newName = info.GeneratePath(template, path)
				break // stop once we have a name.
			}

			if newName == "" {
				return errors.New("could not generate path for " + fileInfo.Name())
			}

			hasMoved, err := fs.MoveDir(path, filepath.Join(c.Library.Directory, newName), doNothing, interactive)
			if err != nil {
				return err
			}
			if hasMoved {
				movedAlbums++
				logthis.Info("Moved "+path+" -> "+newName, logthis.VERBOSE)
				if !doNothing && c.playlistDirectoryConfigured {
					relativePath, err := filepath.Rel(c.Library.Directory, path)
					if err != nil {
						return err
					}
					// find all playlists mentioning the release that was moved, update the path
					for _, p := range playlists {
						if p.Contains(relativePath) {
							// update the playlist
							p.Update(relativePath, newName)
							// save the new playlist
							if err := p.Save(); err != nil {
								logthis.Error(err, logthis.VERBOSE)
							}
						}
					}
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		logthis.Error(walkErr, logthis.NORMAL)
	}

	if !interactive && !daemon.WasReborn() {
		s.Stop()
	}
	logthis.Info(fmt.Sprintf("Moved %d release(s).", movedAlbums), logthis.NORMAL)
	return deleteEmptyLibraryFolders()
}

// DeleteEmptyLibraryFolders deletes empty folders that may appear after sorting albums.
func deleteEmptyLibraryFolders() error {
	c, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		return err
	}
	if !c.LibraryConfigured {
		return errors.New("library section of the configuration file not found")
	}
	// preserving .stfolder for syncthing compatibility
	return fs.DeleteEmptyDirs(c.Library.Directory, []string{filepath.Join(c.Library.Directory, ".stfolder")})
}
