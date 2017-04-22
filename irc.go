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

func analyzeAnnounce(announced string, e *Environment, tracker *GazelleTracker, autosnatchConfig *ConfigAutosnatch) (*Release, error) {
	// getting information
	r := regexp.MustCompile(announcePattern)
	hits := r.FindAllStringSubmatch(announced, -1)
	if len(hits) != 0 {
		release, err := NewRelease(hits[0])
		if err != nil {
			return nil, err
		}
		logThis.Info(release.String(), VERBOSEST)

		// if satisfies a filter, download
		var downloadedInfo bool
		var downloadedTorrent bool
		var info *TrackerTorrentInfo
		for _, filter := range e.config.Filters {
			// checking if duplicate
			if !filter.AllowDuplicates && e.History[tracker.Name].HasDupe(release) {
				logThis.Info(infoNotSnatchingDuplicate, VERBOSE)
				continue
			}
			// checking if a torrent from the same group has already been downloaded
			if filter.UniqueInGroup && e.History[tracker.Name].HasReleaseFromGroup(release) {
				logThis.Info(infoNotSnatchingUniqueInGroup, VERBOSE)
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
					logThis.Info(info.String(), VERBOSE)
				}
				// else check other criteria
				if release.HasCompatibleTrackerInfo(filter, autosnatchConfig.BlacklistedUploaders, info) {
					logThis.Info(" -> "+release.ShortString()+" triggered filter "+filter.Name+", snatching.", NORMAL)
					// move to relevant watch directory
					destination := e.config.General.WatchDir
					if filter.WatchDir != "" {
						destination = filter.WatchDir
					}
					if err := tracker.DownloadTorrent(release, destination); err != nil {
						return nil, errors.Wrap(err, errorDownloadingTorrent)
					}
					downloadedTorrent = true
					// adding to history
					if err := e.History[tracker.Name].AddSnatch(release, filter.Name); err != nil {
						logThis.Error(errors.Wrap(err, errorAddingToHistory), NORMAL)
					}
					// send notification
					e.Notify(filter.Name + ": Snatched " + release.ShortString())
					// save metadata once the download folder is created
					if e.config.General.AutomaticMetadataRetrieval {
						go release.Metadata.SaveFromTracker(tracker, info, e.config.General.DownloadDir)
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
		logThis.Info(fmt.Sprintf(infoNotInteresting, release.ShortString()), VERBOSE)
		return nil, nil
	}
	logThis.Info(infoNotMusic, VERBOSE)
	return nil, nil
}

func ircHandler(e *Environment, tracker *GazelleTracker) {
	autosnatchConfig, err := e.config.GetAutosnatch(tracker.Name)
	if err != nil {
		logThis.Info("Cannot find autosnatch configuration for tracker "+tracker.Name, NORMAL)
		return
	}

	IRCClient := irc.IRC(autosnatchConfig.BotName, tracker.User)
	IRCClient.UseTLS = autosnatchConfig.IRCSSL
	IRCClient.TLSConfig = &tls.Config{InsecureSkipVerify: autosnatchConfig.IRCSSLSkipVerify}
	IRCClient.AddCallback("001", func(e *irc.Event) {
		IRCClient.Privmsg("NickServ", "IDENTIFY "+autosnatchConfig.NickservPassword)
		IRCClient.Privmsg(autosnatchConfig.Announcer, fmt.Sprintf("enter %s %s %s", autosnatchConfig.AnnounceChannel, tracker.User, autosnatchConfig.IRCKey))
	})
	IRCClient.AddCallback("PRIVMSG", func(ev *irc.Event) {
		if ev.Nick != autosnatchConfig.Announcer {
			return // spam
		}
		// e.Arguments's first element is the message's recipient, the second is the actual message
		switch ev.Arguments[0] {
		case autosnatchConfig.BotName:
			// if sent to the bot, it's now ok to join the announce channel
			// waiting for the announcer bot to actually invite us
			time.Sleep(100 * time.Millisecond)
			IRCClient.Join(autosnatchConfig.AnnounceChannel)
		case autosnatchConfig.AnnounceChannel:
			// if sent to the announce channel, it's a new release
			if !e.config.disabledAutosnatching {
				announced := ev.Message()
				logThis.Info("++ Announced: "+announced, VERBOSE)
				if _, err := analyzeAnnounce(announced, e, tracker, autosnatchConfig); err != nil {
					logThis.Error(errors.Wrap(err, errorDealingWithAnnounce), VERBOSE)
					return
				}
			}
		}
	})
	err = IRCClient.Connect(autosnatchConfig.IRCServer)
	if err != nil {
		logThis.Error(errors.Wrap(err, errorConnectingToIRC), NORMAL)
		return
	}
	IRCClient.Loop()
}
