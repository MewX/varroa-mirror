package main

import (
	"fmt"
	"html"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
)

const (
	varroaSocket               = "varroa.sock"
	archivesDir                = "archives"
	archiveNameTemplate        = "varroa_%s.zip"
	defaultConfigurationFile   = "config.yaml"
	unixSocketMessageSeparator = "↑" // because it looks nice
)

func sendOrders(cli *varroaArguments) error {
	conn, err := net.Dial("unix", varroaSocket)
	if err != nil {
		return errors.Wrap(err, errorDialingSocket)
	}
	// sending command
	if _, err = conn.Write([]byte(cli.commandToDaemon())); err != nil {
		return errors.Wrap(err, errorWritingToSocket)
	}
Loop:
	for {
		// read answer
		buf := make([]byte, 512)
		n, err := conn.Read(buf[:])
		if err != nil {
			return errors.Wrap(err, errorReadingFromSocket)
		}
		output := string(buf[:n])
		if !strings.HasSuffix(output, unixSocketMessageSeparator) {
			logThis(errorReadingFromSocket+"Malformed buffer "+string(buf[:n]), NORMAL)
			break
		}
		for _, m := range strings.Split(output, unixSocketMessageSeparator) {
			switch m {
			case "":
			case "stop":
				break Loop
			default:
				fmt.Println(m)
			}
		}
	}
	return conn.Close()
}

func awaitOrders() {
	conn, err := net.Listen("unix", varroaSocket)
	if err != nil {
		logThisError(errors.Wrap(err, errorCreatingSocket), NORMAL)
		return
	}
	defer conn.Close()
	// channel to know when the connection with a specific instance is over
	endThisConnection := make(chan struct{})

	for {
		c, err := conn.Accept()
		if err != nil {
			logThis("Error acceptin from unix socket: "+err.Error(), NORMAL)
			break
		}
		// output back things to CLI
		env.expectedOutput = true

		// this goroutine will send back messages to the instance that sent the command
		go func() {
			for {
				messageToLog := <-env.sendBackToCLI
				// writing to socket with a separator, so that the other instance, reading more slowly,
				// can separate messages that might have been written one after the other
				if _, err = c.Write([]byte(messageToLog + unixSocketMessageSeparator)); err != nil {
					logThisError(errors.Wrap(err, errorWritingToSocket), NORMAL)
				}
				// we've just told the other instance talking was over, ending this connection.
				if messageToLog == "stop" {
					endThisConnection <- struct{}{}
					break
				}
			}
		}()

		buf := make([]byte, 512)
		n, err := c.Read(buf[:])
		if err != nil {
			logThisError(errors.Wrap(err, errorReadingFromSocket), NORMAL)
			continue
		}
		// NOTE: simple split, do something better if necessary
		fullCommand := strings.Split(string(buf[:n]), " ")
		if len(fullCommand) == 0 {
			continue
		}

		stopEverything := false
		switch fullCommand[0] {
		case "stats":
			if err := generateStats(); err != nil {
				logThisError(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
			}
		case "stop":
			logThis("Stopping daemon...", NORMAL)
			stopEverything = true
		case "reload":
			if err := env.Reload(); err != nil {
				logThisError(errors.Wrap(err, errorReloading), NORMAL)
			}
		case "refresh-metadata":
			if err := refreshMetadata(fullCommand[1:]); err != nil {
				logThisError(errors.Wrap(err, errorRefreshingMetadata), NORMAL)
			}
		case "snatch":
			if err := snatchTorrents(fullCommand[1:]); err != nil {
				logThisError(errors.Wrap(err, errorSnatchingTorrent), NORMAL)
			}
		case "check-log":
			if err := checkLog(strings.Join(fullCommand[1:], " "), env.tracker); err != nil {
				logThisError(errors.Wrap(err, errorCheckingLog), NORMAL)
			}
		}
		env.sendBackToCLI <- "stop"
		// waiting for the other instance to be warned that communication is over
		<-endThisConnection
		c.Close()
		env.expectedOutput = false
		if stopEverything {
			// shutting down the daemon, exiting look for socket cleanup
			break
		}
	}
}

func generateStats() error {
	logThis("Generating stats", VERBOSE)
	return env.history.GenerateGraphs()
}

func refreshMetadata(tracker *GazelleTracker, IDStrings []string) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}
	// find ids in history
	var found []string
	for _, r := range env.history.SnatchedReleases {
		if StringInSlice(r.TorrentID, IDStrings) {
			logThis("Found release with ID "+r.TorrentID+" in history: "+r.ShortString()+". Getting tracker metadata.", NORMAL)
			// get data from RED.
			info, err := tracker.GetTorrentInfo(r.TorrentID)
			if err != nil {
				logThisError(errors.Wrap(err, errorCouldNotGetTorrentInfo), NORMAL)
				break
			}
			if env.inDaemon {
				go r.Metadata.SaveFromTracker(tracker, info)
			} else {
				r.Metadata.SaveFromTracker(tracker, info)
			}
			found = append(found, r.TorrentID)
			break
		}
	}
	if len(found) != len(IDStrings) {
		// find the missing IDs
		missing := []string{}
		for _, id := range IDStrings {
			if !StringInSlice(id, found) {
				missing = append(missing, id)
			}
		}
		// try to find even if not in history
		if env.config.downloadFolderConfigured {
			for _, m := range missing {
				// get data from RED.
				info, err := tracker.GetTorrentInfo(m)
				if err != nil {
					logThisError(errors.Wrap(err, errorCouldNotGetTorrentInfo), NORMAL)
					break
				}
				fullFolder := filepath.Join(env.config.General.DownloadDir, html.UnescapeString(info.folder))
				if DirectoryExists(fullFolder) {
					r := info.Release()
					if env.inDaemon {
						go r.Metadata.SaveFromTracker(tracker, info)
					} else {
						r.Metadata.SaveFromTracker(tracker, info)
					}
				} else {
					logThis(fmt.Sprintf(errorCannotFindID, m), NORMAL)
				}
			}
		} else {
			return fmt.Errorf(errorCannotFindID, strings.Join(missing, ","))
		}
	}
	return nil
}

func snatchTorrents(tracker *GazelleTracker, IDStrings []string) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}
	// snatch
	for _, id := range IDStrings {
		if release, err := snatchFromID(tracker, id); err != nil {
			return errors.New("Error snatching torrent with ID #" + id)
		} else {
			logThis("Successfully snatched torrent "+release.ShortString(), NORMAL)
		}
	}
	return nil
}

func checkLog(tracker *GazelleTracker, logPath string) error {
	score, err := tracker.GetLogScore(logPath)
	if err != nil {
		return errors.Wrap(err, errorGettingLogScore)
	}
	logThis(fmt.Sprintf("Found score %s for log file %s.", score, logPath), NORMAL)
	return nil
}

func archiveUserFiles() error {
	// generate Timestamp
	timestamp := time.Now().Format("2006-01-02_15h04m05s")
	archiveName := fmt.Sprintf(archiveNameTemplate, timestamp)
	if !DirectoryExists(archivesDir) {
		if err := os.MkdirAll(archivesDir, 0755); err != nil {
			logThisError(errors.Wrap(err, errorArchiving), NORMAL)
			return errors.Wrap(err, errorArchiving)
		}
	}
	// generate file
	err := archiver.Zip.Make(filepath.Join(archivesDir, archiveName), []string{statsDir, defaultConfigurationFile})
	if err != nil {
		logThisError(errors.Wrap(err, errorArchiving), NORMAL)
	}
	return err
}

func automaticBackup() {
	gocron.Every(1).Day().At("00:00").Do(archiveUserFiles)
	<-gocron.Start()
}
