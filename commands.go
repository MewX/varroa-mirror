package varroa

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/jasonlvhit/gocron"
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
	"github.com/sevlyar/go-daemon"
)

const (
	daemonSocket               = "varroa.sock"
	archivesDir                = "archives"
	archiveNameTemplate        = "varroa_%s.zip"
	unixSocketMessageSeparator = "↑" // because it looks nice
)

func SendOrders(command []byte) error {
	dcClient := NewDaemonComClient()
	go dcClient.RunClient()
	// goroutine to display anything that is sent back from the daemon
	go func() {
		for {
			a := <-dcClient.Incoming
			if string(a) != stopCommand {
				fmt.Println(string(a))
			}
		}
	}()
	// waiting for connection to unix domain socket
	<-dcClient.ClientConnected
	// sending command
	dcClient.Outgoing <- command
	// waiting for end of connection
	<-dcClient.ClientDisconnected
	return nil
}

func awaitOrders(e *Environment) {
	go e.daemonCom.RunServer()
	<-e.daemonCom.ServerUp

Loop:
	for {
		<-e.daemonCom.ClientConnected
		// output back things to CLI
		e.expectedOutput = true
	Loop2:
		for {
			select {
			case a := <-e.daemonCom.Incoming:
				orders := IncomingJSON{}
				if jsonErr := json.Unmarshal(a, &orders); jsonErr != nil {
					logThis.Error(errors.Wrap(jsonErr, "Error parsing incoming command from unix socket"), NORMAL)
					continue
				}
				var tracker *GazelleTracker
				var err error
				if orders.Site != "" {
					tracker, err = e.Tracker(orders.Site)
					if err != nil {
						logThis.Error(errors.Wrap(err, "Error parsing tracker label for command from unix socket"), NORMAL)
						continue
					}
				}

				switch orders.Command {
				case "stats":
					if err := GenerateStats(e); err != nil {
						logThis.Error(errors.Wrap(err, ErrorGeneratingGraphs), NORMAL)
					}
				case stopCommand:
					logThis.Info("Stopping daemon...", NORMAL)
					break Loop
				case "refresh-metadata":
					if err := RefreshMetadata(e, tracker, orders.Args); err != nil {
						logThis.Error(errors.Wrap(err, ErrorRefreshingMetadata), NORMAL)
					}
				case "snatch":
					if err := SnatchTorrents(e, tracker, orders.Args, orders.FLToken); err != nil {
						logThis.Error(errors.Wrap(err, ErrorSnatchingTorrent), NORMAL)
					}
				case "info":
					if err := ShowTorrentInfo(e, tracker, orders.Args); err != nil {
						logThis.Error(errors.Wrap(err, ErrorShowingTorrentInfo), NORMAL)
					}
				case "check-log":
					if err := CheckLog(tracker, orders.Args); err != nil {
						logThis.Error(errors.Wrap(err, ErrorCheckingLog), NORMAL)
					}
				case "uptime":
					if e.startTime.IsZero() {
						logThis.Info("Daemon is not running.", NORMAL)
					} else {
						logThis.Info("varroa musica daemon up for "+time.Since(e.startTime).String()+".", NORMAL)
					}
				}
				e.daemonCom.Outgoing <- []byte(stopCommand)
			case <-e.daemonCom.ClientDisconnected:
				// output back things to CLI
				e.expectedOutput = false
				break Loop2
			}
		}
	}
	e.daemonCom.StopCurrent()
}

func GenerateStats(e *Environment) error {
	atLeastOneError := false
	stats, err := NewStatsDB(filepath.Join(StatsDir, DefaultHistoryDB))
	if err != nil {
		return errors.Wrap(err, "could not access the stats database")
	}
	if err := stats.Update(); err != nil {
		return errors.Wrap(err, "error updating database")
	}

	// get tracker labels from config.
	config, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		return err
	}
	// generate graphs
	for _, tracker := range config.TrackerLabels() {
		if err := stats.GenerateAllGraphsForTracker(tracker); err != nil {
			logThis.Error(err, NORMAL)
			atLeastOneError = true
		}
	}

	// generate index.html
	if err := e.GenerateIndex(); err != nil {
		logThis.Error(errors.Wrap(err, "Error generating index.html"), NORMAL)
	}
	if atLeastOneError {
		return errors.New(ErrorGeneratingGraphs)
	}
	return nil
}

func RefreshMetadata(e *Environment, tracker *GazelleTracker, IDStrings []string) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}

	stats, err := NewStatsDB(filepath.Join(StatsDir, DefaultHistoryDB))
	if err != nil {
		return errors.Wrap(err, "could not access the stats database")
	}

	for _, id := range IDStrings {
		var found Release
		findIDsQuery := q.And(q.Eq("Tracker", tracker.Name), q.Eq("TorrentID", id))
		if err := stats.db.DB.Select(findIDsQuery).First(&found); err != nil {
			if err == storm.ErrNotFound {
				// not found, try to locate download directory nonetheless
				if e.config.DownloadFolderConfigured {
					logThis.Info("Release with ID "+found.TorrentID+" not found in history, trying to locate in downloads directory.", NORMAL)
					// get data from tracker
					info, err := tracker.GetTorrentInfo(id)
					if err != nil {
						logThis.Error(errors.Wrap(err, errorCouldNotGetTorrentInfo), NORMAL)
						break
					}
					fullFolder := filepath.Join(e.config.General.DownloadDir, html.UnescapeString(info.folder))
					if DirectoryExists(fullFolder) {
						if daemon.WasReborn() {
							go SaveMetadataFromTracker(tracker, info, e.config.General.DownloadDir)
						} else {
							SaveMetadataFromTracker(tracker, info, e.config.General.DownloadDir)
						}
					} else {
						logThis.Info(fmt.Sprintf(errorCannotFindID, id), NORMAL)
					}

				} else {
					logThis.Info(fmt.Sprintf(errorCannotFindID, id), NORMAL)
					continue
				}
			} else {
				logThis.Error(errors.Wrap(err, errorCouldNotGetTorrentInfo), NORMAL)
				continue
			}
		} else {
			// was found
			logThis.Info("Found release with ID "+found.TorrentID+" in history: "+found.ShortString()+". Getting tracker metadata.", NORMAL)
			// get data from tracker
			info, err := tracker.GetTorrentInfo(found.TorrentID)
			if err != nil {
				logThis.Error(errors.Wrap(err, errorCouldNotGetTorrentInfo), NORMAL)
				continue
			}
			if daemon.WasReborn() {
				go SaveMetadataFromTracker(tracker, info, e.config.General.DownloadDir)
			} else {
				SaveMetadataFromTracker(tracker, info, e.config.General.DownloadDir)
			}
		}
	}
	return nil
}

func SnatchTorrents(e *Environment, tracker *GazelleTracker, IDStrings []string, useFLToken bool) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}
	// snatch
	for _, id := range IDStrings {
		if release, err := manualSnatchFromID(e, tracker, id, useFLToken); err != nil {
			return errors.New("Error snatching torrent with ID #" + id)
		} else {
			logThis.Info("Successfully snatched torrent "+release.ShortString(), NORMAL)
		}
	}
	return nil
}

func ShowTorrentInfo(e *Environment, tracker *GazelleTracker, IDStrings []string) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}

	stats, err := NewStatsDB(filepath.Join(StatsDir, DefaultHistoryDB))
	if err != nil {
		return errors.Wrap(err, "could not access the stats database")
	}

	// get info
	for _, id := range IDStrings {
		logThis.Info(fmt.Sprintf("+ Info about %s / %s: \n", tracker.Name, id), NORMAL)
		// get release info from ID
		info, err := tracker.GetTorrentInfo(id)
		if err != nil {
			logThis.Error(errors.Wrap(err, fmt.Sprintf("Could not get info about torrent %s on %s, may not exist", id, tracker.Name)), NORMAL)
			continue
		}
		release := info.Release(tracker.Name)
		// TODO better output, might need to add a new info.FullString()
		logThis.Info(release.String(), NORMAL)
		logThis.Info(info.String()+"\n", NORMAL)

		// find if in history
		var found Release
		if err := stats.db.DB.Select(q.And(q.Eq("Tracker", tracker.Name), q.Eq("TorrentID", id))).First(&found); err != nil {
			logThis.Info("+ This torrent has not been snatched with varroa.", NORMAL)
		} else {
			logThis.Info("+ This torrent has been snatched with varroa.", NORMAL)
		}

		// checking the files are still there (if snatched with or without varroa)
		if e.config.DownloadFolderConfigured {
			releaseFolder := filepath.Join(e.config.General.DownloadDir, html.UnescapeString(info.folder))
			if DirectoryExists(releaseFolder) {
				logThis.Info(fmt.Sprintf("Files seem to still be in the download directory: %s", releaseFolder), NORMAL)
				// TODO maybe display when the metadata was last updated?
			} else {
				logThis.Info("The files could not be found in the download directory.", NORMAL)
			}
		}

		// check and print if info/release triggers filters
		autosnatchConfig, err := e.config.GetAutosnatch(tracker.Name)
		if err != nil {
			logThis.Info("Cannot find autosnatch configuration for tracker "+tracker.Name, NORMAL)
		} else {
			logThis.Info("+ Showing autosnatch filters results for this release:\n", NORMAL)
			for _, filter := range e.config.Filters {
				// checking if filter is specifically set for this tracker (if nothing is indicated, all trackers match)
				if len(filter.Tracker) != 0 && !StringInSlice(tracker.Name, filter.Tracker) {
					logThis.Info(fmt.Sprintf(infoFilterIgnoredForTracker, filter.Name, tracker.Name), NORMAL)
					continue
				}
				// checking if a filter is triggered
				if release.Satisfies(filter) && release.HasCompatibleTrackerInfo(filter, autosnatchConfig.BlacklistedUploaders, info) {
					// checking if duplicate
					if !filter.AllowDuplicates && stats.AlreadySnatchedDuplicate(release) {
						logThis.Info(infoNotSnatchingDuplicate, NORMAL)
						continue
					}
					// checking if a torrent from the same group has already been downloaded
					if filter.UniqueInGroup && stats.AlreadySnatchedFromGroup(release) {
						logThis.Info(infoNotSnatchingUniqueInGroup, NORMAL)
						continue
					}
					logThis.Info(filter.Name+": OK!", NORMAL)
				}
			}
		}
	}
	return nil
}

func CheckLog(tracker *GazelleTracker, logPaths []string) error {
	for _, log := range logPaths {
		score, err := tracker.GetLogScore(log)
		if err != nil {
			return errors.Wrap(err, errorGettingLogScore)
		}
		logThis.Info(fmt.Sprintf("Logchecker results: %s.", score), NORMAL)
	}
	return nil
}

func ArchiveUserFiles() error {
	// generate Timestamp
	timestamp := time.Now().Format("2006-01-02_15h04m05s")
	archiveName := fmt.Sprintf(archiveNameTemplate, timestamp)
	if !DirectoryExists(archivesDir) {
		if err := os.MkdirAll(archivesDir, 0755); err != nil {
			logThis.Error(errors.Wrap(err, errorArchiving), NORMAL)
			return errors.Wrap(err, errorArchiving)
		}
	}

	// find all .csv + .db files, save them along with the configuration file
	f, err := os.Open(StatsDir)
	if err != nil {
		return errors.Wrap(err, "Error opening "+StatsDir)
	}
	contents, err := f.Readdirnames(-1)
	if err != nil {
		return errors.Wrap(err, "Error reading directory "+StatsDir)
	}
	f.Close()

	var backupFiles []string
	if FileExists(DefaultConfigurationFile) {
		backupFiles = append(backupFiles, DefaultConfigurationFile)
	}
	encryptedConfigurationFile := strings.TrimSuffix(DefaultConfigurationFile, yamlExt) + encryptedExt
	if FileExists(encryptedConfigurationFile) {
		backupFiles = append(backupFiles, encryptedConfigurationFile)
	}
	for _, c := range contents {
		if filepath.Ext(c) == msgpackExt || filepath.Ext(c) == csvExt {
			backupFiles = append(backupFiles, filepath.Join(StatsDir, c))
		}
	}

	// generate file
	err = archiver.Zip.Make(filepath.Join(archivesDir, archiveName), backupFiles)
	if err != nil {
		logThis.Error(errors.Wrap(err, errorArchiving), NORMAL)
	}
	return err
}

func parseQuota(cmdOut string) (float32, int64, error) {
	output := strings.TrimSpace(cmdOut)
	if output == "" {
		return -1, -1, errors.New("No quota defined for user")
	}
	lines := strings.Split(output, "\n")
	if len(lines) != 3 {
		return -1, -1, errors.New("Unexpected quota output")
	}
	var relevantParts []string
	for _, p := range strings.Split(lines[2], " ") {
		if strings.TrimSpace(p) != "" {
			relevantParts = append(relevantParts, p)
		}
	}
	used, err := strconv.Atoi(relevantParts[1])
	if err != nil {
		return -1, -1, errors.New("Error parsing quota output")
	}
	quota, err := strconv.Atoi(relevantParts[2])
	if err != nil {
		return -1, -1, errors.New("Error parsing quota output")
	}
	// assuming blocks of 1kb
	return 100 * float32(used) / float32(quota), int64(quota-used) * 1024, nil
}

func checkQuota() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	// parse quota -u $(whoami)
	cmdOut, err := exec.Command("quota", "-u", u.Username, "-w").Output()
	if err != nil {
		return err
	}
	pc, remaining, err := parseQuota(string(cmdOut))
	if err != nil {
		return err
	}
	logThis.Info(fmt.Sprintf(currentUsage, pc, readableInt64(remaining)), NORMAL)
	// send warning if this is worrying
	if pc >= 98 {
		logThis.Info(veryLowDiskSpace, NORMAL)
		return Notify(veryLowDiskSpace, FullName, "info")
	} else if pc >= 95 {
		logThis.Info(lowDiskSpace, NORMAL)
		return Notify(lowDiskSpace, FullName, "info")
	}
	return nil
}

func automatedTasks(e *Environment) {
	// new scheduler
	s := gocron.NewScheduler()

	// 1. every day, backup user files
	s.Every(1).Day().At("00:00").Do(ArchiveUserFiles)
	// 2. a little later, also compress the git repository if gitlab pages are configured
	if e.config.gitlabPagesConfigured {
		s.Every(1).Day().At("00:05").Do(e.git.Compress)
	}
	// 3. checking quota is available
	_, err := exec.LookPath("quota")
	if err != nil {
		logThis.Info("The command 'quota' is not available on this system, not able to check disk usage", NORMAL)
	} else {
		// first check
		if err := checkQuota(); err != nil {
			logThis.Error(errors.Wrap(err, "error checking user quota: disk usage monitoring off"), NORMAL)
		} else {
			// scheduler for subsequent quota checks
			s.Every(1).Hour().Do(checkQuota)
		}
	}
	// 4. update database stats
	s.Every(1).Day().At("00:10").Do(GenerateStats, e)
	// launch scheduler
	<-s.Start()
}
