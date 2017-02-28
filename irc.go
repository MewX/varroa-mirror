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

const announcePattern = `(.*?) - (.*) \[([\d]{4})\] \[(Album|Soundtrack|Compilation|Anthology|EP|Single|Live album|Remix|Bootleg|Interview|Mixtape|Demo|Concert Recording|DJ Mix|Unknown)\] - (FLAC|MP3) / (Lossless|24bit Lossless|V0 \(VBR\)|320) /( (Log) /)?( (Cue) /)? ([\w]*) (/ (Scene) )?- (http[s]?://[\w\./:]*torrents\.php\?id=[\d]*) / (http[s]?://[\w\./:]*torrents\.php\?action=download&id=[\d]*) - ([\w\., ]*)`

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
						return nil, errors.New("Could not retrieve torrent info from tracker")
					}
					downloadedInfo = true
					log.Println(info)
					// TODO save info in yaml file somewhere, in torrent dl folder
				}
				// else check other criteria
				if newTorrent.PassesAdditionalChecks(filter, conf.blacklistedUploaders, info) {
					log.Println("++ " + filter.label + ": OK for auto-download, moving to watch folder.")
					if _, err := newTorrent.Download(tracker.client); err != nil {
						return nil, err
					}
					downloadedTorrent = true
					// move to relevant subfolder
					destination := conf.defaultDestinationFolder
					if filter.destinationFolder != "" {
						destination = filter.destinationFolder
					}
					if err := CopyFile(newTorrent.filename, filepath.Join(destination, newTorrent.filename)); err != nil {
						log.Println("Err: could not move to destination folder!")
					}
					// send notification
					if err := notification.Send(filter.label + ": Snatched " + newTorrent.ShortString()); err != nil {
						log.Println(err.Error())
					}
					break
				}
			}
		}
		// if torrent was downloaded, remove temp copy
		if downloadedTorrent {
			if err := os.Remove(newTorrent.filename); err != nil {
				log.Println("Err: could not remove temporary file!")
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
				log.Println("ERR: " + err.Error())
				return
			}
		}
	})

	err := IRCClient.Connect(conf.ircServer)
	if err != nil {
		fmt.Printf("Err %s", err)
		return
	}
	IRCClient.Loop()
}
