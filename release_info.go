package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

const (
	// TODO: add track durations + total duration
	// TODO: lineage: Discogs: XXX; Qobuz: XXX; etc...
	// TODO: add last.fm/discogs/etc info too?
	mdTemplate = `# %s - %s (%d)

**Cover:**

![cover](%s)

**Tags:** %s

## Release

**Type:** %s

**Label:** %s

**Catalog Number:** %s

**Source:** %s
%s
## Audio

**Format:** %s

**Quality:** %s

## Tracklist

%s

## Lineage

%s

## Origin

Automatically generated on %s.

Torrent is %s on %s.

Direct link: %s

`
	remasterTemplate = `
**Remaster Label**: %s

**Remaster Catalog Number:** %s

**Remaster Year:** %d

**Edition name:** %s

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
	return fmt.Sprintf("+ %s [%s]", rit.Title, rit.Size)
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
	Tracker               string
	TrackerURL            string
	LastUpdated           int64
	IsAlive               bool
}

func (ri *ReleaseInfo) fromGazelleInfo(tracker *GazelleTracker, info GazelleTorrent) error {
	ri.Title = html.UnescapeString(info.Response.Group.Name)
	allArtists := info.Response.Group.MusicInfo.Artists
	allArtists = append(allArtists, info.Response.Group.MusicInfo.Composers...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.Conductor...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.Dj...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.Producer...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.RemixedBy...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.With...)
	for _, a := range allArtists {
		ri.Artists = append(ri.Artists, ReleaseInfoArtist{ID: a.ID, Name: html.UnescapeString(a.Name)})
	}
	ri.CoverPath = info.Response.Group.WikiImage
	ri.Tags = info.Response.Group.Tags
	ri.ReleaseType = getGazelleReleaseType(info.Response.Group.ReleaseType)
	ri.Format = info.Response.Torrent.Format
	ri.Source = info.Response.Torrent.Media
	// TODO add if cue/log/logscore + scene
	ri.Quality = info.Response.Torrent.Encoding
	ri.Year = info.Response.Group.Year
	ri.RemasterYear = info.Response.Torrent.RemasterYear
	ri.RemasterLabel = info.Response.Torrent.RemasterRecordLabel
	ri.RemasterCatalogNumber = info.Response.Torrent.RemasterCatalogueNumber
	ri.RecordLabel = info.Response.Group.RecordLabel
	ri.CatalogNumber = info.Response.Group.CatalogueNumber
	ri.EditionName = info.Response.Torrent.RemasterTitle

	// TODO find other info, parse for discogs/musicbrainz/itunes links in both descriptions
	if info.Response.Torrent.Description != "" {
		ri.Lineage = append(ri.Lineage, ReleaseInfoLineage{Source: "TorrentDescription", LinkOrDescription: html.UnescapeString(info.Response.Torrent.Description)})
	}
	// parsing track list
	r := regexp.MustCompile(trackPattern)
	files := strings.Split(info.Response.Torrent.FileList, "|||")
	for _, f := range files {
		track := ReleaseInfoTrack{}
		hits := r.FindAllStringSubmatch(f, -1)
		if len(hits) != 0 {
			// TODO instead of path, actually find the title
			track.Title = html.UnescapeString(hits[0][1])
			size, _ := strconv.ParseUint(hits[0][2], 10, 64)
			track.Size = humanize.IBytes(size)
			ri.Tracks = append(ri.Tracks, track)
			// TODO Duration  + Disc + number
		} else {
			logThis.Info("Could not parse filelist.", NORMAL)
		}

	}
	// TODO TotalTime
	ri.Tracker = tracker.URL
	ri.TrackerURL = tracker.URL + "/torrents.php?torrentid=" + strconv.Itoa(info.Response.Torrent.ID)
	// TODO de-wikify
	ri.Description = html.UnescapeString(info.Response.Group.WikiBody)
	return nil
}

func (ri *ReleaseInfo) toMD() string {
	if ri.Title == "" {
		return "No metadata found"
	}
	// artists
	// TODO separate main from guests
	artists := ""
	for i, a := range ri.Artists {
		artists += a.Name
		if i != len(ri.Artists)-1 {
			artists += ", "
		}
	}
	// tracklist
	tracklist := ""
	for _, t := range ri.Tracks {
		tracklist += t.String() + "\n"
	}
	// compile remaster info
	remaster := ""
	if ri.EditionName != "" || ri.RemasterYear != 0 {
		remaster = fmt.Sprintf(remasterTemplate, ri.RemasterLabel, ri.RemasterCatalogNumber, ri.RemasterYear, ri.EditionName)
	}
	// lineage
	lineage := ""
	for _, l := range ri.Lineage {
		lineage += fmt.Sprintf("**%s**: %s\n", l.Source, l.LinkOrDescription)
	}
	// alive
	isAlive := "still registered"
	if !ri.IsAlive {
		isAlive = "unregistered"
	}
	// general output
	md := fmt.Sprintf(mdTemplate, artists, ri.Title, ri.Year, ri.CoverPath, strings.Join(ri.Tags, ", "),
		ri.ReleaseType, ri.RecordLabel, ri.CatalogNumber, ri.Source, remaster, ri.Format, ri.Quality, tracklist,
		lineage, time.Now().Format("2006-01-02 15:04"), isAlive, ri.Tracker, ri.TrackerURL)
	return md
}

func (ri *ReleaseInfo) writeUserJSON(folder string) error {
	userJSON := filepath.Join(folder, userMetadataJSONFile)
	if FileExists(userJSON) {
		logThis.Info("User metadata JSON already exists.", VERBOSE)
		return nil
	}
	// save as blank JSON, with no values, for the user to force metadata values if needed.
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

func (ri *ReleaseInfo) loadUserJSON(folder string) error {
	userJSON := filepath.Join(folder, userMetadataJSONFile)
	if !FileExists(userJSON) {
		logThis.Info("User metadata JSON does not exist.", VERBOSE)
		return nil
	}
	// loading user metadata file
	userJSONBytes, err := ioutil.ReadFile(userJSON)
	if err != nil {
		return errors.New("Could not read user JSON.")
	}
	var userInfo *ReleaseInfo
	if unmarshalErr := json.Unmarshal(userJSONBytes, &userInfo); unmarshalErr != nil {
		logThis.Info("Error parsing torrent info JSON", NORMAL)
		return nil
	}
	//  overwrite tracker values if non-zero value found
	s := reflect.ValueOf(ri).Elem()
	s2 := reflect.ValueOf(userInfo).Elem()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		f2 := s2.Field(i)
		if f.Type().String() == "string" && f2.String() != "" {
			f.Set(reflect.Value(f2))
		}
		if (f.Type().String() == "int" || f.Type().String() == "int64") && f2.Int() != 0 {
			f.Set(reflect.Value(f2))
		}
	}
	// more complicated types
	if len(userInfo.Tags) != 0 {
		ri.Tags = userInfo.Tags
	}
	// if artists are defined which are not blank
	if len(userInfo.Artists) != 0 {
		if userInfo.Artists[0].Name != "" {
			ri.Artists = userInfo.Artists
		}
	}
	if len(userInfo.Tracks) != 0 {
		if userInfo.Tracks[0].Title != "" {
			ri.Tracks = userInfo.Tracks
		}
	}
	if len(userInfo.Lineage) != 0 {
		if userInfo.Lineage[0].Source != "" {
			ri.Lineage = userInfo.Lineage
		}
	}
	// TODO: what to do with isAlive...
	return nil
}
