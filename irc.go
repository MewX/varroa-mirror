package main

import (
	"crypto/tls"
	"fmt"
	"regexp"
	"time"

	"github.com/pkg/errors"
	"github.com/thoj/go-ircevent"
)

const announcePattern = `(.*?) - (.*) \[([\d]{4})\] \[(Album|Soundtrack|Compilation|Anthology|EP|Single|Live album|Remix|Bootleg|Interview|Mixtape|Demo|Concert Recording|DJ Mix|Unknown)\] - (FLAC|MP3|AAC) / (Lossless|24bit Lossless|V0 \(VBR\)|V2 \(VBR\)|320|256) /( (Log) /)?( (-*\d+)\% /)?( (Cue) /)? (CD|DVD|Vinyl|Soundboard|SACD|DAT|Cassette|WEB|Blu-Ray) (/ (Scene) )?- (http[s]?://[\w\./:]*torrents\.php\?id=[\d]*) / (http[s]?://[\w\./:]*torrents\.php\?action=download&id=[\d]*) - ([\w\., ]*)`

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
		for _, filter := range config.Filters {
			// checking if duplicate
			if !filter.AllowDuplicates && env.history.HasDupe(release) {
				logThis(infoNotSnatchingDuplicate, VERBOSE)
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
				if release.HasCompatibleTrackerInfo(filter, config.Autosnatch[0].BlacklistedUploaders, info) {
					logThis(" -> "+release.ShortString()+" triggered filter "+filter.Name+", snatching.", NORMAL)
					// move to relevant watch directory
					destination := config.General.WatchDir
					if filter.WatchDir != "" {
						destination = filter.WatchDir
					}
					if err := tracker.DownloadTorrent(release, destination); err != nil {
						return nil, errors.Wrap(err, errorDownloadingTorrent)
					}
					downloadedTorrent = true
					// adding to history
					if err := env.history.AddSnatch(release, filter.Name); err != nil {
						logThis(errorAddingToHistory, NORMAL)
					}
					// send notification
					env.Notify(filter.Name + ": Snatched " + release.ShortString())
					// save metadata once the download folder is created
					if env.config.General.AutomaticMetadataRetrieval {
						go release.Metadata.SaveFromTracker(info)
					}
					// no need to consider other filters
					break
				}
			}
		}
		// if torrent was downloaded, remove temp copy
		if downloadedTorrent {
			return release, nil
		}
		logThis(fmt.Sprintf(infoNotInteresting, release.ShortString()), VERBOSE)
		return nil, nil
	}
	logThis(infoNotMusic, VERBOSE)
	return nil, nil
}

func ircHandler(config *Config, tracker *GazelleTracker) {
	IRCClient := irc.IRC(config.Autosnatch[0].BotName, config.Trackers[0].User)
	IRCClient.UseTLS = config.Autosnatch[0].IRCSSL
	IRCClient.TLSConfig = &tls.Config{InsecureSkipVerify: config.Autosnatch[0].IRCSSLSkipVerify}
	IRCClient.AddCallback("001", func(e *irc.Event) {
		IRCClient.Privmsg("NickServ", "IDENTIFY "+config.Autosnatch[0].NickservPassword)
		IRCClient.Privmsg(config.Autosnatch[0].Announcer, fmt.Sprintf("enter %s %s %s", config.Autosnatch[0].AnnounceChannel, config.Trackers[0].User, config.Autosnatch[0].IRCKey))
	})
	IRCClient.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Nick != config.Autosnatch[0].Announcer {
			return // spam
		}
		// e.Arguments's first element is the message's recipient, the second is the actual message
		switch e.Arguments[0] {
		case config.Autosnatch[0].BotName:
			// if sent to the bot, it's now ok to join the announce channel
			// waiting for the announcer bot to actually invite us
			time.Sleep(100 * time.Millisecond)
			IRCClient.Join(config.Autosnatch[0].AnnounceChannel)
		case config.Autosnatch[0].AnnounceChannel:
			// if sent to the announce channel, it's a new release
			if !config.disabledAutosnatching {
				announced := e.Message()
				logThis("++ Announced: "+announced, VERBOSE)
				if _, err := analyzeAnnounce(announced, config, tracker); err != nil {
					logThisError(errors.Wrap(err, errorDealingWithAnnounce), VERBOSE)
					return
				}
			}
		}
	})
	err := IRCClient.Connect(config.Autosnatch[0].IRCServer)
	if err != nil {
		logThisError(errors.Wrap(err, errorConnectingToIRC), NORMAL)
		return
	}
	IRCClient.Loop()
}
