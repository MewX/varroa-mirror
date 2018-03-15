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

// MoveToNewPath moves an album directory to its new home in another genre.
func MoveToNewPath(current, new string, doNothing bool) (bool, error) {
	if new == "" {
		return false, errors.New("no new path for this release")
	}
	// comparer avec l'ancien
	if new != current {
		// if different, move folder
		if !doNothing {
			newPathParent := filepath.Dir(new)
			if _, err := os.Stat(newPathParent); os.IsNotExist(err) {
				// newPathParent does not exist, creating
				err = os.MkdirAll(newPathParent, 0777)
				if err != nil {
					return false, err
				}
			}
			// move
			if err := os.Rename(current, new); err != nil {
				return false, err
			}
			return true, nil
		} else {
			// would have moved, but must do nothing
			return false, nil
		}
	}
	return false, nil
}

func ReorganizeLibrary() error {
	defer TimeTrack(time.Now(), "Reorganize Library")

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
				newName = info.GeneratePath(template)
				break // stop once we have a name.
			}

			if newName == "" {
				return errors.New("could not generate path for " + fileInfo.Name())
			}

			hasMoved, err := MoveToNewPath(path, filepath.Join(c.Library.Directory, newName), false)
			if err != nil {
				return err
			}
			if hasMoved {
				movedAlbums += 1
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
	return DeleteEmptyFolders()
}

// DeleteEmptyFolders deletes empty folders that may appear after sorting albums.
func DeleteEmptyFolders() error {
	defer TimeTrack(time.Now(), "Deleting empty folders")

	c, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		return err
	}
	if !c.LibraryConfigured {
		return errors.New("library section of the configuration file not found")
	}

	deletedDirectories := 0
	deletedDirectoriesThisTime := 0
	atLeastOnce := false

	// loops until all levels of empty directories are deleted
	for !atLeastOnce || deletedDirectoriesThisTime != 0 {
		atLeastOnce = true
		deletedDirectoriesThisTime = 0
		walkErr := filepath.Walk(c.Library.Directory, func(path string, fileInfo os.FileInfo, walkError error) error {
			// when an album has just been removed, Walk goes through it a second
			// time with an "file does not exist" error
			if os.IsNotExist(walkError) {
				return nil
			}
			if fileInfo.IsDir() {
				isEmpty, err := DirectoryIsEmpty(path)
				if err != nil {
					return nil
				}
				if isEmpty {
					logThis.Info("Removing empty directory ", VERBOSEST)
					if err := os.Remove(path); err == nil {
						deletedDirectories++
						deletedDirectoriesThisTime++
					}
				}
			}
			return nil
		})
		if walkErr != nil {
			logThis.Error(walkErr, NORMAL)
		}
	}
	fmt.Printf("Removed %d empty folder(s).\n", deletedDirectories)
	return nil
}
