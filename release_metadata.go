package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/subosito/norma"
)

const (
	errorWritingJSONMetadata        = "Error writing metadata file: "
	errorDownloadingTrackerCover    = "Error downloading tracker cover: "
	errorCreatingMetadataDir        = "Error creating metadata directory: "
	errorRetrievingArtistInfo       = "Error getting info for artist %d"
	errorRetrievingTorrentGroupInfo = "Error getting torrent group info for %d"
	errorWithOriginJSON             = "Error creating or updating origin.json: "
	errorInfoNoMatchForOrigin       = "Error updating origin.json, no match for tracker and/or torrent ID: "
	errorGeneratingUserMetadataJSON = "Error generating user metadata JSON: "
	errorGeneratingSummary          = "Error generating metadata summary: "

	infoAllMetadataSaved          = "All metadata saved."
	infoMetadataSaved             = "Metadata saved to: "
	infoArtistMetadataSaved       = "Artist Metadata for %s saved to: %s"
	infoTorrentGroupMetadataSaved = "Torrent Group Metadata for %s saved to: %s"
	infoCoverSaved                = "Cover saved to: "

	originJSONFile            = "origin.json"
	trackerMetadataFile       = "Release.json"
	trackerTGroupMetadataFile = "ReleaseGroup.json"
	trackerCoverFile          = "Cover"
	metadataDir               = "TrackerMetadata"
	summaryFile               = "Release.md"
	trackPattern              = `(.*){{{(\d*)}}}`
)

type ReleaseMetadata struct {
	Root    string
	Artists []TrackerArtistInfo
	Info    TrackerTorrentInfo
	Group   TrackerTorrentGroupInfo
	Origin  TrackerOriginJSON
	Summary ReleaseInfo
}

func (rm *ReleaseMetadata) Load(folder string) error {
	rm.Root = folder
	// TODO load all JSON , folder == release.folder + metadataDir
	// TODO load custom file for the user to set new information or force existing information to new values
	return nil
}

func (rm *ReleaseMetadata) Synthetize() error {
	// TODO: load all JSONs or check they're loaded

	// fill rm.Summary
	var info GazelleTorrent
	if unmarshalErr := json.Unmarshal(rm.Info.fullJSON, &info.Response); unmarshalErr != nil {
		logThis("Error parsing torrent info JSON", NORMAL)
		return nil
	}
	rm.Summary.Title = info.Response.Group.Name
	allArtists := info.Response.Group.MusicInfo.Artists
	allArtists = append(allArtists, info.Response.Group.MusicInfo.Composers...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.Conductor...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.Dj...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.Producer...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.RemixedBy...)
	allArtists = append(allArtists, info.Response.Group.MusicInfo.With...)
	for _, a := range allArtists {
		rm.Summary.Artists = append(rm.Summary.Artists, ReleaseInfoArtist{ID: a.ID, Name:a.Name})
	}
	rm.Summary.CoverPath = trackerCoverFile + filepath.Ext(info.Response.Group.WikiImage)
	rm.Summary.Tags = info.Response.Group.Tags
	rm.Summary.ReleaseType = getGazelleReleaseType(info.Response.Group.ReleaseType)
	rm.Summary.Format = info.Response.Torrent.Format
	rm.Summary.Source = info.Response.Torrent.Media
	// TODO add if cue/log/logscore + scene
	rm.Summary.Quality = info.Response.Torrent.Encoding
	rm.Summary.Year = info.Response.Group.Year
	rm.Summary.RemasterYear = info.Response.Torrent.RemasterYear
	rm.Summary.RemasterLabel = info.Response.Torrent.RemasterRecordLabel
	rm.Summary.RemasterCatalogNumber = info.Response.Torrent.RemasterCatalogueNumber
	rm.Summary.RecordLabel = info.Response.Group.RecordLabel
	rm.Summary.CatalogNumber = info.Response.Group.CatalogueNumber
	rm.Summary.EditionName = info.Response.Torrent.RemasterTitle

	// TODO find other info, parse for discogs/musicbrainz/itunes links in both descriptions
	rm.Summary.Lineage = append(rm.Summary.Lineage, ReleaseInfoLineage{Source: "TorrentDescription", LinkOrDescription: info.Response.Torrent.Description})

	r := regexp.MustCompile(trackPattern)
	files := strings.Split(info.Response.Torrent.FileList, "|||")
	for _, f := range files {
		track := ReleaseInfoTrack{}
		hits := r.FindAllStringSubmatch(f, -1)
		if len(hits) != 0 {
			// TODO instead of path, actually find the title
			track.Title = hits[0][1]
			size, _ := strconv.ParseUint(hits[0][2], 10, 64)
			track.Size = humanize.IBytes(size)
			rm.Summary.Tracks = append(rm.Summary.Tracks, track)
			// TODO Duration  + Disc + number
		} else {
			logThis("Could not parse filelist.", NORMAL)
		}

	}
	// TODO TotalTime

	rm.Summary.TrackerURL = conf.url + "/torrents.php?torrentid=" + strconv.Itoa(info.Response.Torrent.ID)
	// TODO de-wikify
	rm.Summary.Description = info.Response.Group.WikiBody

	// origin
	rm.Summary.LastUpdated = rm.Origin.LastUpdatedMetadata
	rm.Summary.IsAlive = rm.Origin.IsAlive

	return nil
}

func (rm *ReleaseMetadata) GenerateSummary() error {
	if err := rm.Synthetize(); err != nil {
		return err
	}
	md := rm.Summary.toMD()
	return ioutil.WriteFile(filepath.Join(rm.Root, summaryFile), []byte(md), 0644)
}

// SaveFromTracker all of the associated metadata.
func (rm *ReleaseMetadata) SaveFromTracker(info *TrackerTorrentInfo) error {
	if !conf.downloadFolderConfigured() {
		return nil
	}

	rm.Root = filepath.Join(conf.downloadFolder, html.UnescapeString(info.folder), metadataDir)
	rm.Info = *info

	// create metadata dir if necessary
	if err := os.MkdirAll(filepath.Join(rm.Root), 0775); err != nil {
		return errors.New(errorCreatingMetadataDir + err.Error())
	}
	// creating or updating origin.json
	if err := rm.Origin.Save(filepath.Join(rm.Root, originJSONFile), rm.Info); err != nil {
		return errors.New(errorWithOriginJSON + err.Error())
	}

	// NOTE: errors are not returned (for now) in case the following things can be retrieved

	// write tracker metadata to target folder
	if err := ioutil.WriteFile(filepath.Join(rm.Root, trackerMetadataFile), rm.Info.fullJSON, 0666); err != nil {
		logThis(errorWritingJSONMetadata+err.Error(), NORMAL)
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
			logThis(errorWritingJSONMetadata+err.Error(), NORMAL)
		} else {
			logThis(fmt.Sprintf(infoTorrentGroupMetadataSaved, rm.Group.name, rm.Info.folder), VERBOSE)
		}
	}
	// get artist info
	for _, id := range info.ArtistIDs() {
		artistInfo, err := tracker.GetArtistInfo(id)
		if err != nil {
			logThis(fmt.Sprintf(errorRetrievingArtistInfo, id), NORMAL)
			break
		}
		rm.Artists = append(rm.Artists, *artistInfo)
		// write tracker artist metadata to target folder
		// making sure the artistInfo.name+jsonExt is a valid filename
		if err := ioutil.WriteFile(filepath.Join(rm.Root, norma.Sanitize(artistInfo.name)+jsonExt), artistInfo.fullJSON, 0666); err != nil {
			logThis(errorWritingJSONMetadata+err.Error(), NORMAL)
		} else {
			logThis(fmt.Sprintf(infoArtistMetadataSaved, artistInfo.name, rm.Info.folder), VERBOSE)
		}
	}
	// generate blank user metadata json
	if err := rm.Summary.toJSON(rm.Root); err != nil {
		logThis(errorGeneratingUserMetadataJSON+err.Error(), NORMAL)
	}
	// generate summary
	if err := rm.GenerateSummary(); err != nil {
		logThis(errorGeneratingSummary+err.Error(), NORMAL)
	}
	// download tracker cover to target folder
	if err := info.DownloadCover(filepath.Join(rm.Root, trackerCoverFile)); err != nil {
		logThis(errorDownloadingTrackerCover+err.Error(), NORMAL)
	} else {
		logThis(infoCoverSaved+rm.Info.folder, VERBOSE)
	}
	logThis(infoAllMetadataSaved, VERBOSE)
	return nil
}
