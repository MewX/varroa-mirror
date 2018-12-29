package varroa

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gitlab.com/catastrophic/assistance/fs"
	"gitlab.com/catastrophic/assistance/strslice"
	"gitlab.com/catastrophic/assistance/ui"
)

// MatchAllInSlice checks if all strings in slice a are in slice b
func MatchAllInSlice(a []string, b []string) bool {
	for _, el := range a {
		if !MatchInSlice(el, b) {
			return false
		}
	}
	return true
}

// MatchInSlice checks if a string regexp-matches a slice of patterns, returns bool
func MatchInSlice(a string, b []string) bool {
	// if no slice, no match by default
	if len(b) == 0 {
		return false
	}

	// finding the nature of the contents in b
	var hasIncludes, hasExcludes bool
	for _, p := range b {
		if strings.HasPrefix(p, filterExcludeRegExpPrefix) {
			hasExcludes = true
		} else {
			hasIncludes = true
		}
	}

	// match if we only have excludes and no source string
	if a == "" {
		return !hasIncludes
	}
	var matchFound bool
	for _, pattern := range b {
		if strings.HasPrefix(pattern, filterRegExpPrefix) {
			pattern = strings.Replace(pattern, filterRegExpPrefix, "", 1)
			// try to match
			match, err := regexp.MatchString(pattern, a)
			if err != nil {
				logThis.Error(err, VERBOSE)
			}
			if match {
				if !hasExcludes {
					return true // if only includes, one match is enough
				}
				matchFound = true // found match, but wait to see if it should be excluded
			}
		} else if strings.HasPrefix(pattern, filterExcludeRegExpPrefix) {
			pattern = strings.Replace(pattern, filterExcludeRegExpPrefix, "", 1)
			// try to match
			match, err := regexp.MatchString(pattern, a)
			if err != nil {
				logThis.Error(err, VERBOSE)
			}
			if match {
				return false // a is excluded
			}
		} else if pattern == a {
			if !hasExcludes {
				return true // if only includes, one match is enough
			} else {
				matchFound = true // found match, but wait to see if it should be excluded
			}
		}
	}
	if hasExcludes && !hasIncludes {
		// if we're here, no excludes were triggered and that's the only thing that counts
		return true
	}
	return matchFound
}

// -----------------------------------------------------------------------------

// DirectoryContainsMusic returns true if it contains mp3 or flac files.
func DirectoryContainsMusic(directoryPath string) bool {
	if err := filepath.Walk(directoryPath, func(path string, f os.FileInfo, err error) error {
		if strslice.ContainsCaseInsensitive([]string{mp3Ext, flacExt}, filepath.Ext(path)) {
			// stop walking the directory as soon as a track is found
			return errors.New(foundMusic)
		}
		return nil
	}); err == nil || err.Error() != foundMusic {
		return false
	}
	return true
}

// DirectoryContainsMusicAndMetadata returns true if it contains mp3 or flac files, and JSONs in a TrackerMetadata folder.
func DirectoryContainsMusicAndMetadata(directoryPath string) bool {
	if !DirectoryContainsMusic(directoryPath) {
		return false
	}
	if !fs.DirExists(filepath.Join(directoryPath, MetadataDir)) {
		return false
	}
	if !fs.FileExists(filepath.Join(directoryPath, MetadataDir, OriginJSONFile)) {
		return false
	}
	return true
}

// GetFirstFLACFound returns the first FLAC file found in a directory
func GetFirstFLACFound(directoryPath string) string {
	var firstPath string
	err := filepath.Walk(directoryPath, func(path string, f os.FileInfo, err error) error {
		if strings.ToLower(filepath.Ext(path)) == flacExt {
			// stop walking the directory as soon as a track is found
			firstPath = path
			return errors.New(foundMusic)
		}
		return nil
	})
	if err != nil && err.Error() == foundMusic {
		return firstPath
	}
	return ""
}

// GetAllFLACs returns all FLAC files found in a directory
func GetAllFLACs(directoryPath string) []string {
	return getAllFilesWithExtenstion(directoryPath, flacExt)
}

// GetAllPlaylists returns all m3u files found in a directory
func GetAllPlaylists(directoryPath string) []string {
	return getAllFilesWithExtenstion(directoryPath, m3uExt)
}

// getAllFilesWithExtenstion returns all files found in a directory with a specific extension
func getAllFilesWithExtenstion(directoryPath, extension string) []string {
	var paths []string
	err := filepath.Walk(directoryPath, func(path string, f os.FileInfo, err error) error {
		if strings.ToLower(filepath.Ext(path)) == extension {
			// stop walking the directory as soon as a track is found
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		logThis.Error(err, VERBOSE)
	}
	return paths
}

// MoveToNewPath moves a directory to its new home.
func MoveToNewPath(current, new string, doNothing, interactive bool) (bool, error) {
	if new == "" {
		return false, errors.New("no new path for this folder")
	}
	// comparer avec l'ancien
	if new != current {
		// if different, move folder
		if !doNothing {
			// if interactive and not in simulation mode, must be accepted else we just move on.
			if interactive {
				if !ui.Accept("Move:\n  " + current + "\n->\n  " + new + "\n") {
					return false, nil
				}
			}
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
			// would have moved, but must do nothing, so here we pretend.
			return true, nil
		}
	}
	return false, nil
}

// TimeTrack helps track the time taken by a function.
func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	if elapsed > time.Millisecond {
		logThis.Info(fmt.Sprintf("-- %s in %s", name, elapsed), VERBOSESTEST)
	}
}
