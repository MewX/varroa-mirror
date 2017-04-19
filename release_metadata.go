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

func (rm *ReleaseMetadata) Synthetize() error {
	// fill rm.Summary
	var info GazelleTorrent
	if unmarshalErr := json.Unmarshal(rm.Info.fullJSON, &info.Response); unmarshalErr != nil {
		logThis("Error parsing torrent info JSON", NORMAL)
		return nil
	}
	if err := rm.Summary.fromGazelleInfo(info); err != nil {
		return err
	}
	// origin
	rm.Summary.LastUpdated = rm.Origin.LastUpdatedMetadata
	rm.Summary.IsAlive = rm.Origin.IsAlive

	// load user info
	return rm.Summary.loadUserJSON(rm.Root)
}

func (rm *ReleaseMetadata) GenerateSummary() error {
	if err := rm.Synthetize(); err != nil {
		return err
	}
	md := rm.Summary.toMD()
	return ioutil.WriteFile(filepath.Join(rm.Root, summaryFile), []byte(md), 0644)
}

// SaveFromTracker all of the associated metadata.
func (rm *ReleaseMetadata) SaveFromTracker(tracker *GazelleTracker, info *TrackerTorrentInfo) error {
	if !env.config.downloadFolderConfigured {
		return nil
	}

	rm.Root = filepath.Join(env.config.General.DownloadDir, html.UnescapeString(info.folder), metadataDir)
	rm.Info = *info

	// create metadata dir if necessary
	if err := os.MkdirAll(filepath.Join(rm.Root), 0775); err != nil {
		return errors.Wrap(err, errorCreatingMetadataDir)
	}
	// creating or updating origin.json
	if err := rm.Origin.Save(filepath.Join(rm.Root, originJSONFile), rm.Info); err != nil {
		return errors.Wrap(err, errorWithOriginJSON)
	}

	// NOTE: errors are not returned (for now) in case the following things can be retrieved

	// write tracker metadata to target folder
	if err := ioutil.WriteFile(filepath.Join(rm.Root, trackerMetadataFile), rm.Info.fullJSON, 0666); err != nil {
		logThisError(errors.Wrap(err, errorWritingJSONMetadata), NORMAL)
	} else {
		logThis(infoMetadataSaved+rm.Info.folder, VERBOSE)
	}
	// get torrent group info
	torrentGroupInfo, err := tracker.GetTorrentGroupInfo(rm.Info.groupID)
	if err != nil {
		logThis(fmt.Sprintf(errorRetrievingTorrentGroupInfo, rm.Info.groupID), NORMAL)
	} else {
		rm.Group = *torrentGroupInfo
		// write tracker artist metadata to target folder
		if err := ioutil.WriteFile(filepath.Join(rm.Root, trackerTGroupMetadataFile), rm.Group.fullJSON, 0666); err != nil {
			logThisError(errors.Wrap(err, errorWritingJSONMetadata), NORMAL)
		} else {
			logThis(fmt.Sprintf(infoTorrentGroupMetadataSaved, rm.Group.name, rm.Info.folder), VERBOSE)
		}
	}
	// get artist info
	for _, id := range info.ArtistIDs() {
		artistInfo, err := tracker.GetArtistInfo(id)
		if err != nil {
			logThis(fmt.Sprintf(errorRetrievingArtistInfo, id), NORMAL)
			continue
		}
		rm.Artists = append(rm.Artists, *artistInfo)
		// write tracker artist metadata to target folder
		// making sure the artistInfo.name+jsonExt is a valid filename
		if err := ioutil.WriteFile(filepath.Join(rm.Root, norma.Sanitize(artistInfo.name)+jsonExt), artistInfo.fullJSON, 0666); err != nil {
			logThisError(errors.Wrap(err, errorWritingJSONMetadata), NORMAL)
		} else {
			logThis(fmt.Sprintf(infoArtistMetadataSaved, artistInfo.name, rm.Info.folder), VERBOSE)
		}
	}
	// generate blank user metadata json
	if err := rm.Summary.writeUserJSON(rm.Root); err != nil {
		logThisError(errors.Wrap(err, errorGeneratingUserMetadataJSON), NORMAL)
	}
	// generate summary
	if err := rm.GenerateSummary(); err != nil {
		logThisError(errors.Wrap(err, errorGeneratingSummary), NORMAL)
	}
	// download tracker cover to target folder
	if err := info.DownloadCover(filepath.Join(rm.Root, trackerCoverFile)); err != nil {
		logThisError(errors.Wrap(err, errorDownloadingTrackerCover), NORMAL)
	} else {
		logThis(infoCoverSaved+rm.Info.folder, VERBOSE)
	}
	logThis(infoAllMetadataSaved, VERBOSE)
	return nil
}
