package main

import (
	"errors"
	"fmt"
	"path/filepath"
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
	Path     string
	Root     string
	Metadata ReleaseMetadata // => add InfoHash!!
	State    DownloadState
	LogFile  []string // for check-log
	Tracker  string
	ID       int
	GroupID  int
}

func (d *DownloadFolder) Load() error {
	if d.Path == "" {
		return errors.New("ERRRRR")
	}

	// TODO dertermine d.State
	fmt.Println(d.State)
	fmt.Println(d.State == stateUnknown)

	// TODO check if metadata is present
	fmt.Println(filepath.Join(d.Root, d.Path, metadataDir))
	if DirectoryExists(filepath.Join(d.Root, d.Path, metadataDir)) {
		fmt.Println("HAS METADATA")
	}

	// TODO find if .rejected/.exported in root

	// TODO scan for log files (using walk, for multi-disc releases)

	// TODO if state = unsorted, parse metadata to get Tracker + ID (+ GroupID?)

	return nil
}
