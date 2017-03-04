package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
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
		newTorrent, err := NewTorrent(hits[0])
		if err != nil {
			return nil, err
		}
		//log.Println(newTorrent)

		// if satisfies a filter, download
		var downloadedInfo bool
		var downloadedTorrent bool
		var info *AdditionalInfo
		for _, filter := range conf.filters {
			if newTorrent.Satisfies(filter) {
				// get torrent info!
				if !downloadedInfo {
					info, err = tracker.GetTorrentInfo(newTorrent.torrentID)
					if err != nil {
						return nil, errors.New(errorCouldNotGetTorrentInfo)
					}
					downloadedInfo = true
					log.Println(info)
					// TODO save info in yaml file somewhere, in torrent dl folder
				}
				// else check other criteria
				if newTorrent.PassesAdditionalChecks(filter, conf.blacklistedUploaders, info) {
					log.Println("++ " + filter.label + ": filter triggered, autosnatching and moving to watch folder.")
					if _, err := tracker.Download(newTorrent); err != nil {
						return nil, errors.New(errorDownloadingTorrent + err.Error())
					}
					downloadedTorrent = true
					// move to relevant subfolder
					destination := conf.defaultDestinationFolder
					if filter.destinationFolder != "" {
						destination = filter.destinationFolder
					}
					if err := CopyFile(newTorrent.filename, filepath.Join(destination, newTorrent.filename)); err != nil {
						return nil, errors.New(errorCouldNotMoveTorrent + err.Error())
					}
					// send notification
					if err := notification.Send(filter.label + ": Snatched " + newTorrent.ShortString()); err != nil {
						log.Println(errorNotification + err.Error())
					}
					break
				}
			}
		}
		// if torrent was downloaded, remove temp copy
		if downloadedTorrent {
			if err := os.Remove(newTorrent.filename); err != nil {
				log.Println(fmt.Sprintf(errorRemovingTempFile, newTorrent.filename))
			}
			return newTorrent, nil
		}
		log.Println("++ No filter is interested in that release. Ignoring.")
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
			log.Println("++ Announced: " + announced)
			if _, err := AnalyzeAnnounce(announced, tracker); err != nil {
				log.Println(errorDealingWithAnnounce + err.Error())
				return
			}
		}
	})
	err := IRCClient.Connect(conf.ircServer)
	if err != nil {
		log.Println(errorConnectingToIRC + err.Error())
		return
	}
	IRCClient.Loop()
}
