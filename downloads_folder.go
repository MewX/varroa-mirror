package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"strings"
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
	Metadata           map[string]TrackerTorrentInfo
	State              DownloadState
	Trackers           []string
	ID                 map[string]int
	GroupID            map[string]int
	HasTrackerMetadata bool
	HasOriginJSON      bool
	HasDescription     bool
	HasInfo            bool
	ReleaseInfo        map[string][]byte

	// TODO? LogFiles           []string // for check-log
}

func (d *DownloadFolder) String() string {
	return fmt.Sprintf("Index: %d, Folder: %s, State: %d", d.Index, d.Path, d.State)
}

func (d *DownloadFolder) Description() string {
	txt := d.String()
	if d.HasTrackerMetadata {
		txt += ", Has tracker metadata: "
		if d.HasOriginJSON {
			for _, t := range d.Trackers {
				txt += fmt.Sprintf("%s (ID #%d, GID #%d) ", t, d.ID[t], d.GroupID[t])
			}
		}
		if d.HasDescription {
			for _, t := range d.Trackers {
				txt += fmt.Sprintf("\n%s:\n%s", t, string(d.ReleaseInfo[t]))
			}
		} else if d.HasInfo {
			for _, t := range d.Trackers {
				artists := []string{}
				for k := range d.Metadata[t].artists {
					artists = append(artists, k)
				}
				txt += fmt.Sprintf("| %s metadata: Artist(s): %s / Label: %s ", t, strings.Join(artists, ", "), d.Metadata[t].label)
			}
		}
	} else {
		txt += ", does not have any tracker metadata."
	}
	return txt
}


func (d *DownloadFolder) init() {
	if d.ID == nil {
		d.ID = make(map[string]int)
	}
	if d.GroupID == nil {
		d.GroupID = make(map[string]int)
	}
	if d.ReleaseInfo == nil {
		d.ReleaseInfo = make(map[string][]byte)
	}
	if d.Metadata == nil {
		d.Metadata = make(map[string]TrackerTorrentInfo)
	}
}

func (d *DownloadFolder) Load() error {
	if d.Path == "" {
		return errors.New("Error, download folder path not set")
	}
	// init if necessary
	d.init()

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

					// TODO only update if file has changed!!! or if state == unsorted

					// getting release.md info
					releaseMD := filepath.Join(d.Root, d.Path, metadataDir, tracker+"_"+summaryFile)
					if FileExists(releaseMD) {
						bytes, err := ioutil.ReadFile(releaseMD)
						if err != nil {
							logThis.Error(err, NORMAL)
						} else {
							d.ReleaseInfo[tracker] = bytes
							d.HasDescription = true
						}
					}
					// getting release info from json
					infoJSON := filepath.Join(d.Root, d.Path, metadataDir, tracker+"_"+trackerMetadataFile)
					if FileExists(infoJSON) {
						info := TrackerTorrentInfo{}
						if err := info.Load(infoJSON); err != nil {
							logThis.Error(err, NORMAL)
						} else {
							d.Metadata[tracker] = info
							d.HasInfo = true
						}
					}
				}
			}
		}
	}

	// TODO external way to detect d.State? hidden file? ex: find if .rejected/.exported in root?
	// TODO scan for log files (using walk, for multi-disc releases)?

	return nil
}
