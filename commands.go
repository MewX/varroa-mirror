package varroa

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

// SendOrders from the CLI to the running daemon
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

// awaitOrders in the daemon from the CLI
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
				case "refresh-metadata-by-id":
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
				case "status":
					if e.startTime.IsZero() {
						logThis.Info("Daemon is not running.", NORMAL)
					} else {
						logThis.Info(statusString(e), NORMAL)
					}
				case "reseed":
					if err := Reseed(tracker, orders.Args); err != nil {
						logThis.Error(errors.Wrap(err, ErrorReseed), NORMAL)
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

func statusString(e *Environment) string {
	// version
	status := fmt.Sprintf(FullVersion+"\n", FullName, Version)
	// uptime
	status += "Daemon up since " + e.startTime.Format("2006.01.02 15h04") + " (uptime: " + time.Since(e.startTime).String() + ").\n"
	// autosnatch enabled?
	conf, err := NewConfig(DefaultConfigurationFile)
	if err == nil {
		for _, as := range conf.Autosnatch {
			status += "Autosnatching for tracker " + as.Tracker + ": "
			if as.disabledAutosnatching {
				status += "disabled!\n"
			} else {
				status += "enabled.\n"
			}
		}
	}

	// TODO last autosnatched release for tracker X: date
	return status
}

// GenerateStats for all labels and the associated HTML index.
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
	config, configErr := NewConfig(DefaultConfigurationFile)
	if configErr != nil {
		return configErr
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

// RefreshMetadata for a list of releases on a tracker
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
		var info *TrackerMetadata
		var infoErr error
		findIDsQuery := q.And(q.Eq("Tracker", tracker.Name), q.Eq("TorrentID", id))
		if err := stats.db.DB.Select(findIDsQuery).First(&found); err != nil {
			if err == storm.ErrNotFound {
				// not found, try to locate download directory nonetheless
				if e.config.DownloadFolderConfigured {
					logThis.Info("Release not found in history, trying to locate in downloads directory.", NORMAL)
					// get data from tracker
					info, infoErr = tracker.GetTorrentMetadata(id)
					if infoErr != nil {
						logThis.Error(errors.Wrap(infoErr, errorCouldNotGetTorrentInfo), NORMAL)
						break
					}
					fullFolder := filepath.Join(e.config.General.DownloadDir, info.FolderName)
					if DirectoryExists(fullFolder) {
						if daemon.WasReborn() {
							go info.SaveFromTracker(fullFolder, tracker)
						} else {
							info.SaveFromTracker(fullFolder, tracker)
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
			info, infoErr = tracker.GetTorrentMetadata(found.TorrentID)
			if infoErr != nil {
				logThis.Error(errors.Wrap(infoErr, errorCouldNotGetTorrentInfo), NORMAL)
				continue
			}
			fullFolder := filepath.Join(e.config.General.DownloadDir, info.FolderName)
			if daemon.WasReborn() {
				go info.SaveFromTracker(fullFolder, tracker)
			} else {
				info.SaveFromTracker(fullFolder, tracker)
			}
		}
		// check the number of active seeders
		if !info.IsWellSeeded() {
			logThis.Info("This torrent has less than "+strconv.Itoa(minimumSeeders)+" seeders; if that is not already the case, consider reseeding it.", NORMAL)
		}

	}
	return nil
}

// RefreshLibraryMetadata for a list of releases on a tracker, using the given location instead of assuming they are in the download directory.
func RefreshLibraryMetadata(path string, tracker *GazelleTracker, id string) error {
	if !DirectoryContainsMusicAndMetadata(path) {
		return fmt.Errorf(ErrorFindingMusicAndMetadata, path)
	}
	// get data from tracker
	info, infoErr := tracker.GetTorrentMetadata(id)
	if infoErr != nil {
		return errors.Wrap(infoErr, errorCouldNotGetTorrentInfo)
	}
	return info.SaveFromTracker(path, tracker)
}

// SnatchTorrents on a tracker using their TorrentIDs
func SnatchTorrents(e *Environment, tracker *GazelleTracker, IDStrings []string, useFLToken bool) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}
	// snatch
	for _, id := range IDStrings {
		release, err := manualSnatchFromID(e, tracker, id, useFLToken)
		if err != nil {
			return errors.New("Error snatching torrent with ID #" + id)
		}
		logThis.Info("Successfully snatched torrent "+release.ShortString(), NORMAL)
	}
	return nil
}

// ShowTorrentInfo of a list of releases on a tracker
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
		info, err := tracker.GetTorrentMetadata(id)
		if err != nil {
			logThis.Error(errors.Wrap(err, fmt.Sprintf("Could not get info about torrent %s on %s, may not exist", id, tracker.Name)), NORMAL)
			continue
		}
		release := info.Release()
		logThis.Info(info.TextDescription(true)+"\n", NORMAL)

		// find if in history
		var found Release
		if selectErr := stats.db.DB.Select(q.And(q.Eq("Tracker", tracker.Name), q.Eq("TorrentID", id))).First(&found); selectErr != nil {
			logThis.Info("+ This torrent has not been snatched with varroa.", NORMAL)
		} else {
			logThis.Info("+ This torrent has been snatched with varroa.", NORMAL)
		}

		// checking the files are still there (if snatched with or without varroa)
		if e.config.DownloadFolderConfigured {
			releaseFolder := filepath.Join(e.config.General.DownloadDir, info.FolderName)
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

// Reseed a release using local files and tracker metadata
func Reseed(tracker *GazelleTracker, path []string) error {
	// get config.
	conf, configErr := NewConfig(DefaultConfigurationFile)
	if configErr != nil {
		return configErr
	}
	if !conf.DownloadFolderConfigured {
		return errors.New("impossible to reseed release if downloads directory is not configured")
	}
	// parse metadata for tracker, and get tid
	// assuming reseeding one at a time only (as limited by CLI)
	toc := TrackerOriginJSON{Path: filepath.Join(path[0], metadataDir, originJSONFile)}
	if err := toc.load(); err != nil {
		return errors.Wrap(err, "error reading origin.json")
	}
	// check that tracker is in list of origins
	oj, ok := toc.Origins[tracker.Name]
	if !ok {
		return errors.New("release does not originate from tracker " + tracker.Name)
	}

	// copy files if necessary
	// if the relative path of the downloads directory and the release path is the folder name, it means the path is
	// directly inside the downloads directory, where we want it to reseed.
	// if it is not, we need to copy the files.
	// TODO: maybe hard link instead if in the same filesystem
	// TODO : deal with more than one path
	rel, err := filepath.Rel(conf.General.DownloadDir, path[0])
	if err != nil {
		return errors.Wrap(err, "error trying to locate the target path relatively to the downloads directory")
	}
	// copy files if not in downloads directory
	if rel != filepath.Base(path[0]) {
		if err := CopyDir(path[0], filepath.Join(conf.General.DownloadDir, filepath.Base(path[0])), false); err != nil {
			return errors.Wrap(err, "error copying files to downloads directory")
		}
		logThis.Info("Release files have been copied inside the downloads directory", NORMAL)
	}

	// TODO TO A TEMP DIR, then compare torrent description with path contents; if OK only copy .torrent to conf.General.WatchDir
	// downloading torrent
	if err := tracker.DownloadTorrentFromID(strconv.Itoa(oj.ID), conf.General.WatchDir, false); err != nil {
		return errors.Wrap(err, "error downloading torrent file")
	}
	logThis.Info("Torrent downloaded, your bittorrent client should be able to reseed the release.", NORMAL)
	return nil
}

// CheckLog on a tracker's logchecker
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

// ArchiveUserFiles in a timestamped compressed archive.
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
	var backupFiles []string
	// find all .db files, save them along with the configuration file
	f, err := os.Open(StatsDir)
	if err != nil {
		return errors.Wrap(err, "Error opening "+StatsDir)
	}
	contents, err := f.Readdirnames(-1)
	if err != nil {
		return errors.Wrap(err, "Error reading directory "+StatsDir)
	}
	f.Close()
	for _, c := range contents {
		if filepath.Ext(c) == msgpackExt {
			backupFiles = append(backupFiles, filepath.Join(StatsDir, c))
		}
	}
	// backup the configuration file
	if FileExists(DefaultConfigurationFile) {
		backupFiles = append(backupFiles, DefaultConfigurationFile)
	}
	encryptedConfigurationFile := strings.TrimSuffix(DefaultConfigurationFile, yamlExt) + encryptedExt
	if FileExists(encryptedConfigurationFile) {
		backupFiles = append(backupFiles, encryptedConfigurationFile)
	}
	// generate archive
	err = archiver.Zip.Make(filepath.Join(archivesDir, archiveName), backupFiles)
	if err != nil {
		logThis.Error(errors.Wrap(err, errorArchiving), NORMAL)
	}
	return err
}

// parseQuota output to find out what remains available
func parseQuota(cmdOut string) (float32, int64, error) {
	output := strings.TrimSpace(cmdOut)
	if output == "" {
		return -1, -1, errors.New("no quota defined for user")
	}
	lines := strings.Split(output, "\n")
	if len(lines) != 3 {
		return -1, -1, errors.New("unexpected quota output")
	}
	var relevantParts []string
	for _, p := range strings.Split(lines[2], " ") {
		if strings.TrimSpace(p) != "" {
			relevantParts = append(relevantParts, p)
		}
	}
	used, err := strconv.Atoi(relevantParts[1])
	if err != nil {
		return -1, -1, errors.New("error parsing quota output")
	}
	quota, err := strconv.Atoi(relevantParts[2])
	if err != nil {
		return -1, -1, errors.New("error parsing quota output")
	}
	// assuming blocks of 1kb
	return 100 * float32(used) / float32(quota), int64(quota-used) * 1024, nil
}

// checkQuota on the machine the daemon is run
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

// checkFreeDiskSpace based on the main download directory's location.
func checkFreeDiskSpace() error {
	// get config.
	conf, configErr := NewConfig(DefaultConfigurationFile)
	if configErr != nil {
		return configErr
	}
	if conf.DownloadFolderConfigured {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(conf.General.DownloadDir, &stat); err != nil {
			return errors.Wrap(err, "error finding free disk space")
		}
		// Available blocks * size per block = available space in bytes
		freeBytes := stat.Bavail * uint64(stat.Bsize)
		allBytes := stat.Blocks * uint64(stat.Bsize)
		pcRemaining := 100 * float32(freeBytes) / float32(allBytes)
		// send warning if this is worrying
		if pcRemaining <= 2 {
			logThis.Info(veryLowDiskSpace, NORMAL)
			return Notify(veryLowDiskSpace, FullName, "info")
		} else if pcRemaining <= 10 {
			logThis.Info(lowDiskSpace, NORMAL)
			return Notify(lowDiskSpace, FullName, "info")
		}
		return nil
	}
	return errors.New("download directory not configured, cannot check free disk space")
}

// automatedTasks is a list of cronjobs for maintenance, backup, or non-critical operations
func automatedTasks(e *Environment) {
	// new scheduler
	s := gocron.NewScheduler()

	// 1. every day, backup user files
	s.Every(1).Day().At("00:00").Do(ArchiveUserFiles)
	// 2. a little later, also compress the git repository if gitlab pages are configured
	if e.config.gitlabPagesConfigured {
		s.Every(1).Day().At("00:05").Do(e.git.Compress)
	}
	// 3. check quota is available
	_, err := exec.LookPath("quota")
	if err != nil {
		logThis.Info("The command 'quota' is not available on this system, not able to check disk quota", NORMAL)
	} else {
		// first check
		if err := checkQuota(); err != nil {
			logThis.Error(errors.Wrap(err, "error checking user quota: quota usage monitoring off"), NORMAL)
		} else {
			// scheduler for subsequent quota checks
			s.Every(1).Hour().Do(checkQuota)
		}
	}
	// 4. check disk space is available
	// first check
	if err := checkFreeDiskSpace(); err != nil {
		logThis.Error(errors.Wrap(err, "error checking free disk space: disk usage monitoring off"), NORMAL)
	} else {
		// scheduler for subsequent quota checks
		s.Every(1).Hour().Do(checkFreeDiskSpace)
	}
	// 5. update database stats
	s.Every(1).Day().At("00:10").Do(GenerateStats, e)
	// launch scheduler
	<-s.Start()
}
