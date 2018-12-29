package varroa

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

type ReleaseDir struct {
	Path        string          `json:"-"`
	TrackerInfo TrackerMetadata `json:"-"`
	DiscogsInfo DiscogsRelease  `json:"-"`
	Tracks      []Track         `json:"tracks"`
}

func NewReleaseDir(path string) (*ReleaseDir, error) {
	if !DirectoryExists(path) {
		return nil, errors.New("path " + path + " does not exist")
	}

	return &ReleaseDir{Path: path}, nil
}

func (rd *ReleaseDir) Enhance() error {
	conf, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		logThis.Error(errors.Wrap(err, ErrorLoadingConfig), NORMAL)
		return err
	}

	// load tracker metadata
	if err := rd.getMetadata(); err != nil {
		return err
	}

	// get Discogs metadata
	if conf.discogsTokenConfigured {
		if rd.hasDiscogsMetadata() {
			// TODO  get date / disogsID
			// TODO ask if we want to refresh!
		}
		if Accept("Retrieve Discogs metadata") {
			// retrieve and save discogs metadata
			if err := rd.getDiscogsMetadata(); err != nil {
				return err
			}
		}
	}

	// analyze all tracks and save info to json
	if err := rd.analyzeTracks(); err != nil {
		return err
	}

	// compare & merge tags between Discogs & Tags
	if rd.DiscogsInfo.ID != 0 {
		if err := rd.mergeMetadata(); err != nil {
			return err
		}
	}

	// generate spectrals if they do not exist
	if err := rd.generateSpectrals(); err != nil {
		return err
	}

	// generate up-to-date playlist
	if err := rd.generatePlaylist(); err != nil {
		return err
	}

	return nil
}

func (rd *ReleaseDir) analyzeTracks() error {
	// list all tracks
	flacs := GetAllFLACs(rd.Path)
	// for each, create a ReleaseTrack
	for _, t := range flacs {
		var track Track
		if err := track.parse(t); err != nil {
			logThis.Error(err, NORMAL)
		}
		rd.Tracks = append(rd.Tracks, track)
	}

	// check all have same audio format
	sameEncoding := true
	for i, t := range rd.Tracks {
		if i != 0 {
			sameEncoding = sameEncoding && t.compareEncoding(rd.Tracks[0])
		}
	}
	if !sameEncoding {
		return errors.New("the files do not have the same bit depth and/or sample rate")
	}
	logThis.Info("Audio encoding seems consistent.", NORMAL)

	// TODO check all are from same album : might be tricky for multi-disc?

	// TODO when saving info to json, check if it already exists.
	// TODO if different, show diff before overwriting.

	// saving discogs json
	audioInfoJSON := filepath.Join(rd.Path, AdditionalMetadataDir, tracksMetadataFile)
	// create metadata dir if necessary
	if err := os.MkdirAll(filepath.Join(rd.Path, AdditionalMetadataDir), 0775); err != nil {
		return errors.Wrap(err, errorCreatingMetadataDir)
	}
	metadataJSON, err := json.MarshalIndent(rd.Tracks, "", "    ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(audioInfoJSON, metadataJSON, 0644); err != nil {
		return err
	}
	return nil
}

func (rd *ReleaseDir) getMetadata() error {
	fmt.Println("Reading local tracker metadata...")
	// TODO say how old it is and ask if refresh

	// load Metadata
	d := DownloadEntry{}
	d.FolderName = rd.Path
	if err := d.Load(filepath.Dir(rd.Path)); err != nil {
		return err
	}
	for _, t := range d.Tracker {
		info, err := d.getMetadata(filepath.Dir(rd.Path), t)
		if err != nil {
			logThis.Info("Could not find metadata for tracker "+t, NORMAL)
			continue
		}
		rd.TrackerInfo = info
		break // stop once we have something. if more than 1 tracker source, only the first is retrieved.
	}
	return nil
}

func (rd *ReleaseDir) hasDiscogsMetadata() bool {
	// TODO parse the file and display when it's been last updated
	return FileExists(filepath.Join(rd.Path, AdditionalMetadataDir, discogsMetadataFile))
}

func (rd *ReleaseDir) getDiscogsMetadata() error {
	conf, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		logThis.Error(errors.Wrap(err, ErrorLoadingConfig), NORMAL)
		return err
	}
	if !conf.discogsTokenConfigured {
		return errors.New("discogs token not provided in configuration")
	}

	// TODO check if not done before, if so say when it was and ask to refresh

	fmt.Println("Looking up release on Discogs")
	// lookup Discogs
	discogs, err := NewDiscogsRelease(conf.Metadata.DiscogsToken)
	if err != nil {
		return err
	}
	results, err := discogs.SearchFromTrackerMetadata(rd.TrackerInfo)
	if err != nil {
		return err
	}

	if results.Pagination.Items > 1 {
		// TODO choose one...
		logThis.Info("Found more than one result!", NORMAL)
	}
	// TODO else...
	// getting release metadata from discogs
	discogsMetadataID := results.Results[0].ID
	discogsMetadata, err := discogs.GetRelease(discogsMetadataID)
	if err != nil {
		return err
	}
	rd.DiscogsInfo = *discogsMetadata

	// saving discogs json
	discogsReleaseJSON := filepath.Join(rd.Path, AdditionalMetadataDir, discogsMetadataFile)
	// create metadata dir if necessary
	if mkErr := os.MkdirAll(filepath.Join(rd.Path, AdditionalMetadataDir), 0775); mkErr != nil {
		return errors.Wrap(mkErr, errorCreatingMetadataDir)
	}
	metadataJSON, err := json.MarshalIndent(discogsMetadata, "", "    ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(discogsReleaseJSON, metadataJSON, 0644); err != nil {
		return err
	}
	return nil
}

func (rd *ReleaseDir) generateSpectrals() error {
	// check sox is installed
	_, err := exec.LookPath("sox")
	if err != nil {
		return errors.New("'sox' is not available on this system, not able to generate spectrals")
	}
	// create metadata dir if necessary
	if mkErr := os.MkdirAll(filepath.Join(rd.Path, AdditionalMetadataDir, spectralsMetadataSubdir), 0775); mkErr != nil {
		return errors.Wrap(mkErr, errorCreatingMetadataDir)
	}
	// generate spectrals for each track
	for _, t := range rd.Tracks {
		if err = t.generateSpectrals(filepath.Join(rd.Path, AdditionalMetadataDir, spectralsMetadataSubdir)); err != nil {
			return err
		}
	}
	return nil
}

func (rd *ReleaseDir) generatePlaylist() error {
	// if playlists exist, remove them
	playlists := GetAllPlaylists(rd.Path)
	for _, pl := range playlists {
		if err := os.RemoveAll(pl); err != nil {
			return err
		}
	}

	// generate new playlist
	p := Playlist{Filename: filepath.Join(rd.Path, releasePlaylistFile)}
	if err := p.AddRelease(filepath.Dir(rd.Path), rd.Path); err != nil {
		return err
	}
	return p.Save()
}

func (rd *ReleaseDir) mergeMetadata() error {
	// compare then merge
	discogsTrackTags := rd.DiscogsInfo.TrackTags()
	for _, dt := range discogsTrackTags {
		for _, t := range rd.Tracks {
			if t.Tags.Number == dt.Number {
				if t.Tags.diff(dt) {
					logThis.Info("Tracks have the same metadata.", VERBOSEST)
				} else {
					if Accept("Try to merge tracker & discogs metadata") {
						if err := t.Tags.merge(dt); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}
