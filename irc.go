package varroa

import (
	"crypto/tls"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/catastrophic/assistance/logthis"
	"gitlab.com/catastrophic/assistance/strslice"
	irc "gitlab.com/catastrophic/go-ircevent"
)

const (
	announcePattern            = `(.*?) - (.*) \[([\d]{4})\] \[(Album|Soundtrack|Compilation|Anthology|EP|Single|Live album|Remix|Bootleg|Interview|Mixtape|Demo|Concert Recording|DJ Mix|Unknown)\] - (FLAC|MP3|AAC) / (Lossless|24bit Lossless|V0 \(VBR\)|V2 \(VBR\)|320|256) /( (Log) /)?( (-*\d+)\% /)?( (Cue) /)? (CD|DVD|Vinyl|Soundboard|SACD|DAT|Cassette|WEB|Blu-Ray) (/ (Scene) )?- (http[s]?://[\w\./:]*torrents\.php\?id=[\d]*) / (http[s]?://[\w\./:]*torrents\.php\?action=download&id=[\d]*) - ([\w\., ]*)`
	alternativeAnnouncePattern = `(.*?) - (.*) \[([\d]{4})\] \[(Album|Soundtrack|Compilation|Anthology|EP|Single|Live album|Remix|Bootleg|Interview|Mixtape|Demo|Concert Recording|DJ Mix|Unknown)\] - (FLAC|MP3|AAC) / (Lossless|24bit Lossless|V0 \(VBR\)|V2 \(VBR\)|320|256) /( (Log) /)?( (-*\d+)\% /)?( (Cue) /)? (CD|DVD|Vinyl|Soundboard|SACD|DAT|Cassette|WEB|Blu-Ray) (/ (Scene) )?- ([\w\., ]*) - (http[s]?://[\w\./:]*torrents\.php\?id=[\d]*) / (http[s]?://[\w\./:]*torrents\.php\?action=download&id=[\d]*)`
)

func analyzeAnnounce(announced string, e *Environment, tracker *GazelleTracker, autosnatchConfig *ConfigAutosnatch) error {
	stats, err := NewStatsDB(filepath.Join(StatsDir, DefaultHistoryDB))
	if err != nil {
		return errors.Wrap(err, "could not access the stats database")
	}

	// getting information, trying the alternative pattern if the main one fails
	r := regexp.MustCompile(announcePattern)
	r2 := regexp.MustCompile(alternativeAnnouncePattern)
	alternative := false
	hits := r.FindAllStringSubmatch(announced, -1)
	if len(hits) == 0 {
		hits = r2.FindAllStringSubmatch(announced, -1)
		alternative = true
	}

	if len(hits) != 0 {
		release, err := NewRelease(tracker.Name, hits[0], alternative)
		if err != nil {
			return err
		}
		logthis.Info(release.String(), logthis.VERBOSEST)

		// if satisfies a filter, download
		var downloadedInfo bool
		var downloadedTorrent bool
		var info *TrackerMetadata
		for _, filter := range e.config.Filters {
			// checking if filter is specifically set for this tracker (if nothing is indicated, all trackers match)
			if len(filter.Tracker) != 0 && !strslice.Contains(filter.Tracker, tracker.Name) {
				logthis.Info(fmt.Sprintf(infoFilterIgnoredForTracker, filter.Name, tracker.Name), logthis.VERBOSE)
				continue
			}
			// checking if a filter is triggered
			if release.Satisfies(filter) {
				// get torrent info!
				if !downloadedInfo {
					info, err = tracker.GetTorrentMetadata(release.TorrentID)
					if err != nil {
						return errors.New(errorCouldNotGetTorrentInfo)
					}
					downloadedInfo = true
					logthis.Info(info.TextDescription(false), logthis.VERBOSE)
				}
				// else check other criteria
				if release.HasCompatibleTrackerInfo(filter, autosnatchConfig.BlacklistedUploaders, info) {
					release.Filter = filter.Name

					// checking if duplicate
					if !filter.AllowDuplicates && stats.AlreadySnatchedDuplicate(release) {
						logthis.Info(filter.Name+": "+infoNotSnatchingDuplicate, logthis.VERBOSE)
						continue
					}
					// checking if a torrent from the same group has already been downloaded
					if filter.UniqueInGroup && stats.AlreadySnatchedFromGroup(release) {
						logthis.Info(filter.Name+": "+infoNotSnatchingUniqueInGroup, logthis.VERBOSE)
						continue
					}
					logthis.Info(" -> "+release.ShortString()+" triggered filter "+filter.Name+", snatching.", logthis.NORMAL)
					// move to relevant watch directory
					destination := e.config.General.WatchDir
					if filter.WatchDir != "" {
						destination = filter.WatchDir
					}
					if err := tracker.DownloadTorrent(release.torrentURL, release.TorrentFile(), destination); err != nil {
						return errors.Wrap(err, errorDownloadingTorrent)
					}
					downloadedTorrent = true
					// adding to history
					if err := stats.AddSnatch(*release); err != nil {
						logthis.Error(errors.Wrap(err, errorAddingToHistory), logthis.NORMAL)
					}
					// send notification
					if err := Notify(filter.Name+": Snatched "+release.ShortString(), tracker.Name, "info", e); err != nil {
						logthis.Error(err, logthis.NORMAL)
					}
					// save metadata once the download folder is created
					if e.config.General.AutomaticMetadataRetrieval {
						go info.SaveFromTracker(filepath.Join(e.config.General.DownloadDir, info.FolderName), tracker)
					}
					// no need to consider other filters
					break
				}
			}
		}
		if !downloadedTorrent {
			logthis.Info(fmt.Sprintf(infoNotInteresting, release.ShortString()), logthis.VERBOSE)
		}
		return nil
	}
	logthis.Info(infoNotMusic, logthis.VERBOSE)
	return nil
}

func ircHandler(e *Environment, tracker *GazelleTracker) {
	// general replacer to remove color codes and other useless things from announces.
	r := strings.NewReplacer("\x02TORRENT:\x02 ", "", "\x0303", "", "\x0304", "", "\x0310", "", "\x0312", "", "\x03", "")

	autosnatchConfig, err := e.config.GetAutosnatch(tracker.Name)
	if err != nil {
		logthis.Info("Cannot find autosnatch configuration for tracker "+tracker.Name, logthis.NORMAL)
		return
	}

	IRCClient := irc.IRC(autosnatchConfig.BotName, tracker.User)
	if autosnatchConfig.LocalAddress != "" {
		IRCClient.LocalAddress = autosnatchConfig.LocalAddress
	}
	IRCClient.UseTLS = autosnatchConfig.IRCSSL
	IRCClient.TLSConfig = &tls.Config{InsecureSkipVerify: autosnatchConfig.IRCSSLSkipVerify}
	IRCClient.AddCallback("001", func(_ *irc.Event) {
		IRCClient.Privmsg("NickServ", "IDENTIFY "+autosnatchConfig.NickservPassword)
		IRCClient.Privmsg(autosnatchConfig.Announcer, fmt.Sprintf("enter %s %s %s", autosnatchConfig.AnnounceChannel, tracker.User, autosnatchConfig.IRCKey))
		if e.config.ircNotifsConfigured {
			IRCClient.Privmsg(e.config.Notifications.Irc.User, "varroa bot, connected.")
		}
	})
	IRCClient.AddCallback("PRIVMSG", func(ev *irc.Event) {
		if ev.Nick != autosnatchConfig.Announcer {
			return // spam
		}
		if strings.HasPrefix(ev.Message(), announcerBadCredentials) {
			logthis.Info("error connecting to IRC: IRC key rejected by "+autosnatchConfig.Announcer+"; disconnecting.", logthis.NORMAL)
			return
		}
		// e.Arguments's first element is the message's recipient, the second is the actual message
		switch strings.ToLower(ev.Arguments[0]) {
		case strings.ToLower(autosnatchConfig.BotName):
			// if sent to the bot, it's now ok to join the announce channel
			// waiting for the announcer bot to actually invite us
			time.Sleep(100 * time.Millisecond)
			IRCClient.Join(autosnatchConfig.AnnounceChannel)
		case strings.ToLower(autosnatchConfig.AnnounceChannel):
			// if sent to the announce channel, it's a new release
			e.mutex.RLock()
			canSnatch := !autosnatchConfig.disabledAutosnatching
			e.mutex.RUnlock()
			if canSnatch {
				announced := r.Replace(ev.Message())
				logthis.Info("++ Announced on "+tracker.Name+": "+announced, logthis.VERBOSE)
				if err = analyzeAnnounce(announced, e, tracker, autosnatchConfig); err != nil {
					logthis.Error(errors.Wrap(err, errorDealingWithAnnounce), logthis.VERBOSE)
					return
				}
			}
		}
	})
	err = IRCClient.Connect(autosnatchConfig.IRCServer)
	if err != nil {
		logthis.Error(errors.Wrap(err, errorConnectingToIRC), logthis.NORMAL)
		return
	}
	e.ircClient = IRCClient
	IRCClient.Loop()
}
