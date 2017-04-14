package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

/*
Template:

	## Release

	Edition: EDITION NAME
	Remaster: REMASTER
	Remaster Year: XXX
	Source: WEB/CD/...

	## Audio

	Format: XXX
	Quality: XXX

	## Tracklist

	DISC - NUMBER - Title (Time)

	Total time: XXX

	## Lineage

	Qobuz: xxx
	Bandcamp: xxx
	Discogs: xxx
	...

	## Tracker Description

	xxx

	## Source

	Automatically generated on TIMESTAMP.
	Torrent was still registered / unregistered on TRACKER URL.
	Direct link: TORRENTLINK
*/

const (
	userMetadataJSONFile = "user_metadata.json"

	mdTemplate = `# %s - %s (%d)

**Cover:** %s

**Tags:** %s

## Release

**Type:** %s

**Label:** %s

**Catalog Number:** %s

## Tracklist

%s


`
)

type ReleaseInfoTrack struct {
	Disc     string
	Number   string
	Title    string
	Duration string
	Size     string
}

func (rit *ReleaseInfoTrack) String() string {
	return fmt.Sprintf("%s [%s]", rit.Title, rit.Size)
}

type ReleaseInfoArtist struct {
	ID   int
	Name string
}

type ReleaseInfoLineage struct {
	Source            string
	LinkOrDescription string
}

type ReleaseInfo struct {
	Artists               []ReleaseInfoArtist
	Title                 string
	CoverPath             string
	Tags                  []string
	ReleaseType           string
	RecordLabel           string
	CatalogNumber         string
	Year                  int
	EditionName           string
	RemasterLabel         string
	RemasterCatalogNumber string
	RemasterYear          int
	Source                string
	Format                string
	Quality               string
	Tracks                []ReleaseInfoTrack
	TotalTime             string
	Lineage               []ReleaseInfoLineage
	Description           string
	TrackerURL            string
	LastUpdated           int64
	IsAlive               bool
}

// TODO new command varroa info ID/InfoHash/folder outputs this.
func (ri *ReleaseInfo) toMD() string {
	if ri.Title == "" {
		return "No metadata found"
	}

	artists := ""
	for i, a := range ri.Artists {
		artists += a.Name
		if i != len(ri.Artists)-1 {
			artists += ", "
		}
	}
	tracklist := ""
	for _, t := range ri.Tracks {
		tracklist += t.String() + "\n"
	}

	md := fmt.Sprintf(mdTemplate, artists, ri.Title, ri.Year, ri.CoverPath, strings.Join(ri.Tags, ", "),
		ri.ReleaseType, ri.RecordLabel, ri.CatalogNumber, tracklist)

	// TODO return template

	return md
}

func (ri *ReleaseInfo) toJSON(folder string) error {
	userJSON := filepath.Join(folder, userMetadataJSONFile)
	if FileExists(userJSON) {
		logThis("User metadata JSON already exists.", VERBOSE)
		return nil
	}

	// TODO save as blank JSON, with no values. call it user_metadata.json
	blank := ReleaseInfo{}
	blank.Artists = append(blank.Artists, ReleaseInfoArtist{})
	blank.Tracks = append(blank.Tracks, ReleaseInfoTrack{})
	blank.Lineage = append(blank.Lineage, ReleaseInfoLineage{})
	metadataJSON, err := json.MarshalIndent(blank, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(userJSON, metadataJSON, 0644)
}

func (ri *ReleaseInfo) fromJSON() error {
	// TODO load as JSON, to overwrite tracker value!

	return nil
}
