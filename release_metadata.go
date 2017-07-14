package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/subosito/norma"
)

const trackPattern = `(.*){{{(\d*)}}}`

type ReleaseMetadata struct {
	Root    string
	Artists []TrackerArtistInfo
	Info    TrackerTorrentInfo
	Group   TrackerTorrentGroupInfo
	Origin  TrackerOriginJSON
	Summary ReleaseInfo
}

func (rm *ReleaseMetadata) Synthetize(tracker *GazelleTracker) error {
	// fill rm.Summary
	var info GazelleTorrent
	if unmarshalErr := json.Unmarshal(rm.Info.fullJSON, &info.Response); unmarshalErr != nil {
		logThis.Info("Error parsing torrent info JSON", NORMAL)
		return nil
	}
	if err := rm.Summary.fromGazelleInfo(tracker, info); err != nil {
		return err
	}
	// origin
	origin, ok := rm.Origin.Origins[tracker.Name]
	if ok {
		rm.Summary.LastUpdated = origin.LastUpdatedMetadata
		rm.Summary.IsAlive = origin.IsAlive
	} else {
		return errors.New(errorInfoNoMatchForOrigin)
	}
	// load user info
	return rm.Summary.loadUserJSON(rm.Root)
}

func (rm *ReleaseMetadata) GenerateSummary(tracker *GazelleTracker) error {
	if err := rm.Synthetize(tracker); err != nil {
		return err
	}
	md := rm.Summary.toMD()
	return ioutil.WriteFile(filepath.Join(rm.Root, tracker.Name+"_"+summaryFile), []byte(md), 0644)
}

// SaveFromTracker all of the associated metadata.
func (rm *ReleaseMetadata) SaveFromTracker(tracker *GazelleTracker, info *TrackerTorrentInfo, destination string) error {
	if destination == "" {
		// download folder not set
		return nil
	}

	rm.Root = filepath.Join(destination, html.UnescapeString(info.folder), metadataDir)
	rm.Info = *info

	// create metadata dir if necessary
	if err := os.MkdirAll(filepath.Join(rm.Root), 0775); err != nil {
		return errors.Wrap(err, errorCreatingMetadataDir)
	}
	// creating or updating origin.json
	if err := rm.Origin.Save(filepath.Join(rm.Root, originJSONFile), tracker, rm.Info); err != nil {
		return errors.Wrap(err, errorWithOriginJSON)
	}

	// NOTE: errors are not returned (for now) in case the following things can be retrieved

	// write tracker metadata to target folder
	if err := ioutil.WriteFile(filepath.Join(rm.Root, tracker.Name+"_"+trackerMetadataFile), rm.Info.fullJSON, 0666); err != nil {
		logThis.Error(errors.Wrap(err, errorWritingJSONMetadata), NORMAL)
	} else {
		logThis.Info(infoMetadataSaved+rm.Info.folder, VERBOSE)
	}
	// get torrent group info
	torrentGroupInfo, err := tracker.GetTorrentGroupInfo(rm.Info.groupID)
	if err != nil {
		logThis.Info(fmt.Sprintf(errorRetrievingTorrentGroupInfo, rm.Info.groupID), NORMAL)
	} else {
		rm.Group = *torrentGroupInfo
		// write tracker artist metadata to target folder
		if err := ioutil.WriteFile(filepath.Join(rm.Root, tracker.Name+"_"+trackerTGroupMetadataFile), rm.Group.fullJSON, 0666); err != nil {
			logThis.Error(errors.Wrap(err, errorWritingJSONMetadata), NORMAL)
		} else {
			logThis.Info(fmt.Sprintf(infoTorrentGroupMetadataSaved, rm.Group.name, rm.Info.folder), VERBOSE)
		}
	}
	// get artist info
	for _, id := range info.ArtistIDs() {
		artistInfo, err := tracker.GetArtistInfo(id)
		if err != nil {
			logThis.Info(fmt.Sprintf(errorRetrievingArtistInfo, id), NORMAL)
			continue
		}
		rm.Artists = append(rm.Artists, *artistInfo)
		// write tracker artist metadata to target folder
		// making sure the artistInfo.name+jsonExt is a valid filename
		if err := ioutil.WriteFile(filepath.Join(rm.Root, tracker.Name+"_"+norma.Sanitize(artistInfo.name)+jsonExt), artistInfo.fullJSON, 0666); err != nil {
			logThis.Error(errors.Wrap(err, errorWritingJSONMetadata), NORMAL)
		} else {
			logThis.Info(fmt.Sprintf(infoArtistMetadataSaved, artistInfo.name, rm.Info.folder), VERBOSE)
		}
	}
	// generate blank user metadata json
	if err := rm.Summary.writeUserJSON(rm.Root); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingUserMetadataJSON), NORMAL)
	}
	// generate summary
	if err := rm.GenerateSummary(tracker); err != nil {
		logThis.Error(errors.Wrap(err, errorGeneratingSummary), NORMAL)
	}
	// download tracker cover to target folder
	if err := info.DownloadCover(filepath.Join(rm.Root, tracker.Name+"_"+trackerCoverFile)); err != nil {
		logThis.Error(errors.Wrap(err, errorDownloadingTrackerCover), NORMAL)
	} else {
		logThis.Info(infoCoverSaved+rm.Info.folder, VERBOSE)
	}
	logThis.Info(fmt.Sprintf(infoAllMetadataSaved, tracker.Name), VERBOSE)
	return nil
}