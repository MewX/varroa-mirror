package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/gregdel/pushover"
	"github.com/thoj/go-ircevent"
)

const announcePattern = `(.*?) - (.*) \[([\d]{4})\] \[(Album|Soundtrack|Compilation|Anthology|EP|Single|Live album|Remix|Bootleg|Interview|Mixtape|Demo|Concert Recording|DJ Mix|Unknown)\] - (FLAC|MP3) / (Lossless|24bit Lossless|V0 \(VBR\)|320) /( (Log) /)?( (Cue) /)? ([\w]*) (/ (Scene) )?- (http[s]?://[\w\./:]*torrents\.php\?id=[\d]*) / (http[s]?://[\w\./:]*torrents\.php\?action=download&id=[\d]*) - ([\w\., ]*)`

func sendTorrentNotification(notification *pushover.Pushover, recipient *pushover.Recipient, torrent *Release, filterLabel string) {
	if notification == nil {
		return
	}
	// send notification
	message := pushover.NewMessageWithTitle(filterLabel+": Snatched "+torrent.ShortString(), "varroa musica")
	_, err := notification.SendMessage(message, recipient)
	if err != nil {
		log.Println(err.Error())
	}
}

func AnalyzeAnnounce(config Config, announced string, tracker GazelleTracker, notification *pushover.Pushover, recipient *pushover.Recipient) (*Release, error) {
	// getting information
	r := regexp.MustCompile(announcePattern)
	hits := r.FindAllStringSubmatch(announced, -1)
	if len(hits) != 0 {
		newTorrent, err := NewTorrent(hits[0])
		if err != nil {
			return nil, err
		}
		log.Println(newTorrent)

		// if satisfies a filter, download
		var downloadedTorrent bool
		var downloadedInfo bool
		var autoDownload bool
		var info *AdditionalInfo
		for _, filter := range config.filters {
			if newTorrent.Satisfies(filter) {
				log.Println("Caught by filter " + filter.label + ".")
				var dlErr error

				var wg sync.WaitGroup
				wg.Add(2)
				// goroutine1: download the torrent
				go func() {
					defer wg.Done()
					if !downloadedTorrent {
						_, dlErr = newTorrent.Download(tracker.client)
						if dlErr == nil {
							downloadedTorrent = true
							newTorrent.Parse()
						}
					}
				}()
				// goroutine2: get release info from tracker
				go func() {
					defer wg.Done()
					// get torrent info!
					if !downloadedInfo {
						info, err = tracker.GetTorrentInfo(newTorrent.torrentID)
						if err != nil {
							log.Println("Could not retrieve torrent info from tracker")
						} else {
							downloadedInfo = true
							log.Println(info)
						}
						// TODO save info in yaml file somewhere, in torrent dl folder
					}
				}()
				// sync
				wg.Wait()

				// if nothing was downloaded, abort
				if dlErr != nil {
					return nil, dlErr
				}
				// else check other criteria
				if newTorrent.PassesAdditionalChecks(filter, info) {
					log.Println("++ OK for auto-download, moving to watch folder.")
					// move to relevant subfolder
					destination := config.defaultDestinationFolder
					if filter.destinationFolder != "" {
						destination = filter.destinationFolder
					}
					if err := CopyFile(newTorrent.filename, filepath.Join(destination, newTorrent.filename)); err != nil {
						log.Println("Err: could not move to destination folder!")
					}
					sendTorrentNotification(notification, recipient, newTorrent, filter.label)
					autoDownload = true
					break
				} else {
					log.Println("Release does not pass additional checks, disregarding for this filter.")
				}
			}
		}
		// if torrent was downloaded, remove temp copy
		if downloadedTorrent {
			if err := os.Remove(newTorrent.filename); err != nil {
				log.Println("Err: could not remove temporary file!")
			}
			if !autoDownload {
				log.Println("++ No filter is interested in that release. Ignoring.")
			}
			return newTorrent, nil
		}
		log.Println("++ No filter is interested in that release. Ignoring.")
		return nil, nil

	}
	return nil, errors.New("No hits!")
}

func ircHandler(conf Config, tracker GazelleTracker, notification *pushover.Pushover, recipient *pushover.Recipient) {
	irccon := irc.IRC(conf.botName, conf.user)
	irccon.UseTLS = false
	irccon.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	irccon.AddCallback("001", func(e *irc.Event) {
		irccon.Privmsg("NickServ", "IDENTIFY "+conf.nickServPassword)
		irccon.Privmsg(conf.announcer, fmt.Sprintf("enter %s %s %s", conf.announceChannel, conf.user, conf.ircKey))
	})
	irccon.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Nick == conf.announcer {
			announced := e.Message()
			log.Println("++ Announced: " + announced)
			if _, err := AnalyzeAnnounce(conf, announced, tracker, notification, recipient); err != nil {
				log.Println("ERR: " + err.Error())
				return
			}
		}
	})

	err := irccon.Connect(conf.ircServer)
	if err != nil {
		fmt.Printf("Err %s", err)
		return
	}
	irccon.Loop()
}
