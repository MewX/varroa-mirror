package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/thoj/go-ircevent"
)

const (
	announcePattern = `(.*?) - (.*) \[([\d]{4})\] \[(Album|Soundtrack|Compilation|Anthology|EP|Single|Live album|Remix|Bootleg|Interview|Mixtape|Demo|Concert Recording|DJ Mix|Unknown)\] - (FLAC|MP3) / (Lossless|24bit Lossless|V0 \(VBR\)|320) /( (Log) /)?( (Cue) /)? ([\w]*) (/ (Scene) )?- (http[s]?://[\w\./:]*torrents\.php\?id=[\d]*) / (http[s]?://[\w\./:]*torrents\.php\?action=download&id=[\d]*) - ([\w\., ]*)`

	errorDealingWithAnnounce    = "Error dealing with announced torrent: "
	errorConnectingToIRC        = "Error connecting to IRC: "
	errorCouldNotGetTorrentInfo = "Error retreiving torrent info from tracker"
	errorCouldNotMoveTorrent    = "Error moving torrent to destination folder: "
	errorDownloadingTorrent     = "Error downloading torrent: "
	errorRemovingTempFile       = "Error removing temporary file %s"


)

func AnalyzeAnnounce(announced string, tracker GazelleTracker) (*Release, error) {
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
		var info *AdditionalInfo
		for _, filter := range conf.filters {
			if release.Satisfies(filter) {
				// get torrent info!
				if !downloadedInfo {
					info, err = tracker.GetTorrentInfo(release.torrentID)
					if err != nil {
						return nil, errors.New(errorCouldNotGetTorrentInfo)
					}
					downloadedInfo = true
					logThis(info.String(), VERBOSE)
					// TODO save info in yaml file somewhere, in torrent dl folder
				}
				// else check other criteria
				if release.HasCompatibleTrackerInfo(filter, conf.blacklistedUploaders, info) {
					logThis(" -> " + release.ShortString() + " triggered filter " + filter.label + ", snatching.", NORMAL)
					if _, err := tracker.Download(release); err != nil {
						return nil, errors.New(errorDownloadingTorrent + err.Error())
					}
					downloadedTorrent = true
					// move to relevant subfolder
					destination := conf.defaultDestinationFolder
					if filter.destinationFolder != "" {
						destination = filter.destinationFolder
					}
					if err := CopyFile(release.filename, filepath.Join(destination, release.filename)); err != nil {
						return nil, errors.New(errorCouldNotMoveTorrent + err.Error())
					}
					// send notification
					if err := notification.Send(filter.label + ": Snatched " + release.ShortString()); err != nil {
						logThis(errorNotification+err.Error(), VERBOSE)
					}
					break
				}
			}
		}
		// if torrent was downloaded, remove temp copy
		if downloadedTorrent {
			if err := os.Remove(release.filename); err != nil {
				logThis(fmt.Sprintf(errorRemovingTempFile, release.filename), VERBOSE)
			}
			return release, nil
		}
		logThis("No filter is interested in that release. Ignoring.", VERBOSE)
		return nil, nil

	}
	return nil, errors.New("No hits!")
}

func ircHandler(tracker GazelleTracker) {
	IRCClient := irc.IRC(conf.botName, conf.user)
	IRCClient.UseTLS = conf.ircSSL
	IRCClient.TLSConfig = &tls.Config{}
	IRCClient.AddCallback("001", func(e *irc.Event) {
		IRCClient.Privmsg("NickServ", "IDENTIFY "+conf.nickServPassword)
		IRCClient.Privmsg(conf.announcer, fmt.Sprintf("enter %s %s %s", conf.announceChannel, conf.user, conf.ircKey))
	})
	IRCClient.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Nick == conf.announcer {
			announced := e.Message()
			logThis("++ Announced: "+announced, VERBOSE)
			if _, err := AnalyzeAnnounce(announced, tracker); err != nil {
				logThis(errorDealingWithAnnounce+err.Error(), VERBOSE)
				return
			}
		}
	})
	err := IRCClient.Connect(conf.ircServer)
	if err != nil {
		logThis(errorConnectingToIRC+err.Error(), NORMAL)
		return
	}
	IRCClient.Loop()
}
