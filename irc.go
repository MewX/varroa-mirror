package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/subosito/norma"
	"github.com/thoj/go-ircevent"
)

const (
	announcePattern = `(.*?) - (.*) \[([\d]{4})\] \[(Album|Soundtrack|Compilation|Anthology|EP|Single|Live album|Remix|Bootleg|Interview|Mixtape|Demo|Concert Recording|DJ Mix|Unknown)\] - (FLAC|MP3|AAC) / (Lossless|24bit Lossless|V0 \(VBR\)|V2 \(VBR\)|320|256) /( (Log) /)?( (-*\d+)\% /)?( (Cue) /)? (CD|DVD|Vinyl|Soundboard|SACD|DAT|Cassette|WEB|Blu-Ray) (/ (Scene) )?- (http[s]?://[\w\./:]*torrents\.php\?id=[\d]*) / (http[s]?://[\w\./:]*torrents\.php\?action=download&id=[\d]*) - ([\w\., ]*)`
	originJSONFile  = "origin.json"

	errorDealingWithAnnounce        = "Error dealing with announced torrent: "
	errorConnectingToIRC            = "Error connecting to IRC: "
	errorCouldNotGetTorrentInfo     = "Error retreiving torrent info from tracker"
	errorCouldNotMoveTorrent        = "Error moving torrent to destination folder: "
	errorDownloadingTorrent         = "Error downloading torrent: "
	errorRemovingTempFile           = "Error removing temporary file %s"
	errorAddingToHistory            = "Error adding release to history"
	errorWritingJSONMetadata        = "Error writing metadata file: "
	errorDownloadingTrackerCover    = "Error downloading tracker cover: "
	errorCreatingMetadataDir        = "Error creating metadata directory: "
	errorRetrievingArtistInfo       = "Error getting info for artist %d"
	errorRetrievingTorrentGroupInfo = "Error getting torrent group info for %d"
	errorWithOriginJSON             = "Error creating or updating origin.json: "
	errorInfoNoMatchForOrigin       = "Error updating origin.json, no match for tracker and/or torrent ID: "

	notSnatchingDuplicate     = "Similar release already downloaded, and duplicates are not allowed"
	metadataSaved             = "Metadata saved to: "
	artistMetadataSaved       = "Artist Metadata for %s saved to: %s"
	torrentGroupMetadataSaved = "Torrent Group Metadata for %s saved to: %s"
	coverSaved                = "Cover saved to: "
	trackerMetadataFile       = "Release.json"
	trackerTGroupMetadataFile = "ReleaseGroup.json"
	trackerCoverFile          = "Cover"
	metadataDir               = "TrackerMetadata"
)

type OriginJSON struct {
	Tracker             string
	ID                  int
	TimeSnatched        int64
	LastUpdatedMetadata int64
	IsAlive             bool
	TimeOfDeath         int64
}

func saveTrackerMetadata(info *TrackerTorrentInfo) {
	if !conf.downloadFolderConfigured() {
		return
	}
	go func() {
		completePath := filepath.Join(conf.downloadFolder, html.UnescapeString(info.folder))
		// create metadata dir
		if err := os.MkdirAll(filepath.Join(completePath, metadataDir), 0775); err != nil {
			logThis(errorCreatingMetadataDir+err.Error(), NORMAL)
			return
		}
		// creating or updating origin.json
		origin := filepath.Join(completePath, metadataDir, originJSONFile)
		var originJSON *OriginJSON
		var writeOrigin bool
		if FileExists(origin) {
			// updating metadata
			b, err := ioutil.ReadFile(origin)
			if err != nil {
				logThis(errorWithOriginJSON+err.Error(), NORMAL)
			} else {
				if err := json.Unmarshal(b, &originJSON); err != nil {
					logThis(errorWithOriginJSON+err.Error(), NORMAL)
				} else {
					if originJSON.ID == info.id && originJSON.Tracker == conf.url {
						originJSON.LastUpdatedMetadata = time.Now().Unix()
						writeOrigin = true
					} else {
						logThis(errorInfoNoMatchForOrigin+err.Error(), NORMAL)
					}
					// TODO if GetTorrentInfo errors out: origin.IsAlive = false and set TimeOfDeath
				}
			}
		} else {
			// creating origin
			originJSON = &OriginJSON{Tracker: conf.url, ID: info.id, TimeSnatched: time.Now().Unix(), LastUpdatedMetadata: time.Now().Unix(), IsAlive: true}
			writeOrigin = true
		}
		// writing origin.json
		if writeOrigin {
			b, err := json.MarshalIndent(originJSON, "", "    ")
			if err != nil {
				logThis(errorWithOriginJSON+err.Error(), NORMAL)
			} else {
				if err := ioutil.WriteFile(origin, b, 0644); err != nil {
					logThis(errorWithOriginJSON+err.Error(), NORMAL)
				}
			}
		}

		// write tracker metadata to target folder
		if err := ioutil.WriteFile(filepath.Join(completePath, metadataDir, trackerMetadataFile), info.fullJSON, 0666); err != nil {
			logThis(errorWritingJSONMetadata+err.Error(), NORMAL)
		} else {
			logThis(metadataSaved+info.folder, VERBOSE)
		}
		// download tracker cover to target folder
		if err := info.DownloadCover(filepath.Join(completePath, metadataDir, trackerCoverFile)); err != nil {
			logThis(errorDownloadingTrackerCover+err.Error(), NORMAL)
		} else {
			logThis(coverSaved+info.folder, VERBOSE)
		}
		torrentGroupInfo, err := tracker.GetTorrentGroupInfo(info.groupID)
		if err != nil {
			logThis(fmt.Sprintf(errorRetrievingTorrentGroupInfo, info.groupID), NORMAL)
		} else {
			// write tracker artist metadata to target folder
			if err := ioutil.WriteFile(filepath.Join(completePath, metadataDir, trackerTGroupMetadataFile), torrentGroupInfo.fullJSON, 0666); err != nil {
				logThis(errorWritingJSONMetadata+err.Error(), NORMAL)
			} else {
				logThis(fmt.Sprintf(torrentGroupMetadataSaved, torrentGroupInfo.name, info.folder), VERBOSE)
			}
		}
		// get artist info
		for _, id := range info.ArtistIDs() {
			artistInfo, err := tracker.GetArtistInfo(id)
			if err != nil {
				logThis(fmt.Sprintf(errorRetrievingArtistInfo, id), NORMAL)
				break
			}
			// write tracker artist metadata to target folder
			// making sure the artistInfo.name+jsonExt is a valid filename
			if err := ioutil.WriteFile(filepath.Join(completePath, metadataDir, norma.Sanitize(artistInfo.name)+jsonExt), artistInfo.fullJSON, 0666); err != nil {
				logThis(errorWritingJSONMetadata+err.Error(), NORMAL)
			} else {
				logThis(fmt.Sprintf(artistMetadataSaved, artistInfo.name, info.folder), VERBOSE)
			}
		}
	}()
}

func analyzeAnnounce(announced string, tracker *GazelleTracker) (*Release, error) {
	// getting information
	r := regexp.MustCompile(announcePattern)
	hits := r.FindAllStringSubmatch(announced, -1)
	if len(hits) != 0 {
		release, err := NewRelease(hits[0])
		if err != nil {
			return nil, err
		}
		logThis(release.String(), VERBOSEST)

		// if satisfies a filter, download
		var downloadedInfo bool
		var downloadedTorrent bool
		var info *TrackerTorrentInfo
		for _, filter := range conf.filters {
			// checking if duplicate
			if !filter.allowDuplicate && history.HasDupe(release) {
				logThis(notSnatchingDuplicate, VERBOSE)
				continue
			}
			// checking if a filter is triggered
			if release.Satisfies(filter) {
				// get torrent info!
				if !downloadedInfo {
					info, err = tracker.GetTorrentInfo(release.TorrentID)
					if err != nil {
						return nil, errors.New(errorCouldNotGetTorrentInfo)
					}
					downloadedInfo = true
					logThis(info.String(), VERBOSE)
				}
				// else check other criteria
				if release.HasCompatibleTrackerInfo(filter, conf.blacklistedUploaders, info) {
					logThis(" -> "+release.ShortString()+" triggered filter "+filter.label+", snatching.", NORMAL)
					if err := tracker.Download(release); err != nil {
						return nil, errors.New(errorDownloadingTorrent + err.Error())
					}
					downloadedTorrent = true
					// move to relevant watch directory
					destination := conf.defaultDestinationFolder
					if filter.destinationFolder != "" {
						destination = filter.destinationFolder
					}
					if err := CopyFile(release.TorrentFile, filepath.Join(destination, release.TorrentFile)); err != nil {
						return nil, errors.New(errorCouldNotMoveTorrent + err.Error())
					}
					// adding to history
					if err := history.SnatchHistory.Add(release, filter.label); err != nil {
						logThis(errorAddingToHistory, NORMAL)
					}
					// send notification
					if err := notification.Send(filter.label + ": Snatched " + release.ShortString()); err != nil {
						logThis(errorNotification+err.Error(), VERBOSE)
					}
					// save metadata once the download folder is created
					saveTrackerMetadata(info)
					// no need to consider other filters
					break
				}
			}
		}
		// if torrent was downloaded, remove temp copy
		if downloadedTorrent {
			if err := os.Remove(release.TorrentFile); err != nil {
				logThis(fmt.Sprintf(errorRemovingTempFile, release.TorrentFile), VERBOSE)
			}
			return release, nil
		}
		logThis("No filter is interested in that release. Ignoring.", VERBOSE)
		return nil, nil
	}
	return nil, errors.New("No hits!")
}

func ircHandler() {
	IRCClient := irc.IRC(conf.irc.botName, conf.user)
	IRCClient.UseTLS = conf.irc.SSL
	IRCClient.TLSConfig = &tls.Config{InsecureSkipVerify: conf.irc.SSLSkipVerify}
	IRCClient.AddCallback("001", func(e *irc.Event) {
		IRCClient.Privmsg("NickServ", "IDENTIFY "+conf.irc.nickServPassword)
		IRCClient.Privmsg(conf.irc.announcer, fmt.Sprintf("enter %s %s %s", conf.irc.announceChannel, conf.user, conf.irc.key))
	})
	IRCClient.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Nick != conf.irc.announcer {
			return // spam
		}
		// e.Arguments's first element is the message's recipient, the second is the actual message
		switch e.Arguments[0] {
		case conf.irc.botName:
			// if sent to the bot, it's now ok to join the announce channel
			// waiting for the announcer bot to actually invite us
			time.Sleep(100 * time.Millisecond)
			IRCClient.Join(conf.irc.announceChannel)
		case conf.irc.announceChannel:
			// if sent to the announce channel, it's a new release
			if !disabledAutosnatching {
				announced := e.Message()
				logThis("++ Announced: "+announced, VERBOSE)
				if _, err := analyzeAnnounce(announced, tracker); err != nil {
					logThis(errorDealingWithAnnounce+err.Error(), VERBOSE)
					return
				}
			}
		}
	})
	err := IRCClient.Connect(conf.irc.server)
	if err != nil {
		logThis(errorConnectingToIRC+err.Error(), NORMAL)
		return
	}
	IRCClient.Loop()
}
