package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/russross/blackfriday"
)

const (
	stateUnknown  DownloadState = iota // no metadata
	stateUnsorted                      // has metadata but is unsorted
	stateExported                      // has metadata and has been exported to library
	stateRejected                      // has metadata and is not to be exported to library
)

type DownloadState int

//-----------------------

type DownloadFolder struct {
	Index              uint64
	Path               string
	Root               string
	Metadata           ReleaseMetadata // => add InfoHash!!
	State              DownloadState
	LogFiles           []string // for check-log
	Trackers           []string
	ID                 map[string]int
	GroupID            map[string]int
	HasTrackerMetadata bool
	HasOriginJSON      bool
	HasDescription     bool
	ReleaseInfo        map[string][]byte
}

func (d *DownloadFolder) String() string {
	txt := fmt.Sprintf("Index: %d, Folder: %s, State: %d", d.Index, d.Path, d.State)
	if d.HasTrackerMetadata {
		txt += ", Has tracker metadata: "
	}
	if d.HasOriginJSON {
		for _, t := range d.Trackers {
			txt += fmt.Sprintf("%s (ID #%d, GID #%d) ", t, d.ID[t], d.GroupID[t])
		}
	}
	return txt
}

func (d *DownloadFolder) Load() error {
	if d.Path == "" {
		return errors.New("Error, download folder path not set")
	}
	// init if necessary
	if d.ID == nil {
		d.ID = make(map[string]int)
	}
	if d.GroupID == nil {
		d.GroupID = make(map[string]int)
	}
	if d.ReleaseInfo == nil {
		d.ReleaseInfo = make(map[string][]byte)
	}

	// TODO dertermine d.State?

	// check if tracker metadata is present
	if DirectoryExists(filepath.Join(d.Root, d.Path, metadataDir)) {
		d.HasTrackerMetadata = true

		// find origin.json
		if FileExists(filepath.Join(d.Root, d.Path, metadataDir, originJSONFile)) {
			d.HasOriginJSON = true

			origin := TrackerOriginJSON{Path: filepath.Join(d.Root, d.Path, metadataDir, originJSONFile)}
			if err := origin.load(); err != nil {
				logThis.Error(err, NORMAL)
			} else {
				for tracker, o := range origin.Origins {
					if !StringInSlice(tracker, d.Trackers) {
						d.Trackers = append(d.Trackers, tracker)
					}
					// updating
					d.ID[tracker] = o.ID
					d.GroupID[tracker] = o.GroupID
					// getting release.md info
					// TODO only update if file has changed!!!
					if FileExists(filepath.Join(d.Root, d.Path, metadataDir, tracker+"_"+summaryFile)) {
						bytes, err := ioutil.ReadFile(filepath.Join(d.Root, d.Path, metadataDir, tracker+"_"+summaryFile))
						if err != nil {
							logThis.Error(err, NORMAL)
						} else {
							d.ReleaseInfo[tracker] = blackfriday.MarkdownCommon(bytes)
							d.HasDescription = true
						}
					}
				}
			}
		}
	}

	// TODO find if .rejected/.exported in root

	// TODO scan for log files (using walk, for multi-disc releases)

	// TODO if state = unsorted, parse metadata to get Tracker + ID (+ GroupID?)

	return nil
}
