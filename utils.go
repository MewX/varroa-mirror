package varroa

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gitlab.com/catastrophic/assistance/fs"
	"gitlab.com/catastrophic/assistance/logthis"
	"gitlab.com/catastrophic/assistance/music"
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
				logthis.Error(err, logthis.VERBOSE)
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
				logthis.Error(err, logthis.VERBOSE)
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

// DirectoryContainsMusicAndMetadata returns true if it contains mp3 or flac files, and JSONs in a TrackerMetadata folder.
func DirectoryContainsMusicAndMetadata(directoryPath string) bool {
	if !music.ContainsMusic(directoryPath) {
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

// TimeTrack helps track the time taken by a function.
func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	if elapsed > time.Millisecond {
		logthis.Info(fmt.Sprintf("-- %s in %s", name, elapsed), logthis.VERBOSESTEST)
	}
}
