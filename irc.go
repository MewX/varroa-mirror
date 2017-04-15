package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/thoj/go-ircevent"
)

const (
	announcePattern = `(.*?) - (.*) \[([\d]{4})\] \[(Album|Soundtrack|Compilation|Anthology|EP|Single|Live album|Remix|Bootleg|Interview|Mixtape|Demo|Concert Recording|DJ Mix|Unknown)\] - (FLAC|MP3|AAC) / (Lossless|24bit Lossless|V0 \(VBR\)|V2 \(VBR\)|320|256) /( (Log) /)?( (-*\d+)\% /)?( (Cue) /)? (CD|DVD|Vinyl|Soundboard|SACD|DAT|Cassette|WEB|Blu-Ray) (/ (Scene) )?- (http[s]?://[\w\./:]*torrents\.php\?id=[\d]*) / (http[s]?://[\w\./:]*torrents\.php\?action=download&id=[\d]*) - ([\w\., ]*)`

	errorDealingWithAnnounce    = "Error dealing with announced torrent: "
	errorConnectingToIRC        = "Error connecting to IRC: "
	errorCouldNotGetTorrentInfo = "Error retreiving torrent info from tracker"
	errorCouldNotMoveTorrent    = "Error moving torrent to destination folder: "
	errorDownloadingTorrent     = "Error downloading torrent: "
	errorRemovingTempFile       = "Error removing temporary file %s"
	errorAddingToHistory        = "Error adding release to history"

	infoNotInteresting = "No filter is interested in release: %s. Ignoring."
	infoNotMusic       = "Not a music release, ignoring."

	notSnatchingDuplicate = "Similar release already downloaded, and duplicates are not allowed"
)

func analyzeAnnounce(announced string, config *Config, tracker *GazelleTracker) (*Release, error) {
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
		for _, filter := range config.filters {
			// checking if duplicate
			if !filter.allowDuplicate && env.history.HasDupe(release) {
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
				if release.HasCompatibleTrackerInfo(filter, config.blacklistedUploaders, info) {
					logThis(" -> "+release.ShortString()+" triggered filter "+filter.label+", snatching.", NORMAL)
					if err := tracker.Download(release); err != nil {
						return nil, errors.New(errorDownloadingTorrent + err.Error())
					}
					downloadedTorrent = true
					// move to relevant watch directory
					destination := env.config.defaultDestinationFolder
					if filter.destinationFolder != "" {
						destination = filter.destinationFolder
					}
					if err := CopyFile(release.TorrentFile, filepath.Join(destination, release.TorrentFile)); err != nil {
						return nil, errors.New(errorCouldNotMoveTorrent + err.Error())
					}
					// adding to history
					if err := env.history.SnatchHistory.Add(release, filter.label); err != nil {
						logThis(errorAddingToHistory, NORMAL)
					}
					// send notification
					env.Notify(filter.label + ": Snatched " + release.ShortString())
					// save metadata once the download folder is created
					go release.Metadata.SaveFromTracker(info)
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
		logThis(fmt.Sprintf(infoNotInteresting, release.ShortString()), VERBOSE)
		return nil, nil
	}
	logThis(infoNotMusic, VERBOSE)
	return nil, nil
}

func ircHandler(config *Config, tracker *GazelleTracker) {
	IRCClient := irc.IRC(config.irc.botName, config.user)
	IRCClient.UseTLS = config.irc.SSL
	IRCClient.TLSConfig = &tls.Config{InsecureSkipVerify: config.irc.SSLSkipVerify}
	IRCClient.AddCallback("001", func(e *irc.Event) {
		IRCClient.Privmsg("NickServ", "IDENTIFY "+config.irc.nickServPassword)
		IRCClient.Privmsg(config.irc.announcer, fmt.Sprintf("enter %s %s %s", config.irc.announceChannel, config.user, config.irc.key))
	})
	IRCClient.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Nick != config.irc.announcer {
			return // spam
		}
		// e.Arguments's first element is the message's recipient, the second is the actual message
		switch e.Arguments[0] {
		case config.irc.botName:
			// if sent to the bot, it's now ok to join the announce channel
			// waiting for the announcer bot to actually invite us
			time.Sleep(100 * time.Millisecond)
			IRCClient.Join(config.irc.announceChannel)
		case config.irc.announceChannel:
			// if sent to the announce channel, it's a new release
			if !config.disabledAutosnatching {
				announced := e.Message()
				logThis("++ Announced: "+announced, VERBOSE)
				if _, err := analyzeAnnounce(announced, config, tracker); err != nil {
					logThis(errorDealingWithAnnounce+err.Error(), VERBOSE)
					return
				}
			}
		}
	})
	err := IRCClient.Connect(config.irc.server)
	if err != nil {
		logThis(errorConnectingToIRC+err.Error(), NORMAL)
		return
	}
	IRCClient.Loop()
}
