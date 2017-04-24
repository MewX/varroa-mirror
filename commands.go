package main

import (
	"encoding/json"
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
	if _, err = conn.Write(cli.commandToDaemon()); err != nil {
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
			logThis.Info(errorReadingFromSocket+"Malformed buffer "+string(buf[:n]), NORMAL)
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

func awaitOrders(e *Environment) {
	conn, err := net.Listen("unix", varroaSocket)
	if err != nil {
		logThis.Error(errors.Wrap(err, errorCreatingSocket), NORMAL)
		return
	}
	defer conn.Close()
	// channel to know when the connection with a specific instance is over
	endThisConnection := make(chan struct{})

	for {
		c, err := conn.Accept()
		if err != nil {
			logThis.Info("Error acceptin from unix socket: "+err.Error(), NORMAL)
			break
		}
		// output back things to CLI
		e.expectedOutput = true

		// this goroutine will send back messages to the instance that sent the command
		go func() {
			for {
				messageToLog := <-e.sendBackToCLI
				// writing to socket with a separator, so that the other instance, reading more slowly,
				// can separate messages that might have been written one after the other
				if _, err = c.Write([]byte(messageToLog + unixSocketMessageSeparator)); err != nil {
					logThis.Error(errors.Wrap(err, errorWritingToSocket), NORMAL)
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
			logThis.Error(errors.Wrap(err, errorReadingFromSocket), NORMAL)
			continue
		}

		orders := IncomingJSON{}
		if jsonErr := json.Unmarshal(buf[:n], &orders); jsonErr != nil {
			logThis.Error(errors.Wrap(jsonErr, "Error parsing incoming command from unix socket"), NORMAL)
			continue
		}
		var tracker *GazelleTracker
		if orders.Site != "" {
			tracker, err = e.Tracker(orders.Site)
			if err != nil {
				logThis.Error(errors.Wrap(err, "Error parsing tracker label for command from unix socket"), NORMAL)
				continue
			}
		}

		stopEverything := false
		switch orders.Command {
		case "stats":
			if err := generateStats(e); err != nil {
				logThis.Error(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
			}
		case "stop":
			logThis.Info("Stopping daemon...", NORMAL)
			stopEverything = true
		case "reload":
			if err := e.Reload(); err != nil {
				logThis.Error(errors.Wrap(err, errorReloading), NORMAL)
			}
		case "refresh-metadata":
			if err := refreshMetadata(e, tracker, orders.Args); err != nil {
				logThis.Error(errors.Wrap(err, errorRefreshingMetadata), NORMAL)
			}
		case "snatch":
			if err := snatchTorrents(e, tracker, orders.Args); err != nil {
				logThis.Error(errors.Wrap(err, errorSnatchingTorrent), NORMAL)
			}
		case "check-log":
			if err := checkLog(tracker, orders.Args); err != nil {
				logThis.Error(errors.Wrap(err, errorCheckingLog), NORMAL)
			}
		}
		e.sendBackToCLI <- "stop"
		// waiting for the other instance to be warned that communication is over
		<-endThisConnection
		c.Close()
		e.expectedOutput = false
		if stopEverything {
			// shutting down the daemon, exiting look for socket cleanup
			break
		}
	}
}

func generateStats(e *Environment) error {
	logThis.Info("Generating stats", VERBOSE)
	for _, h := range e.History {
		if err := h.GenerateGraphs(e); err != nil {
			return err
		}
	}
	return nil
}

func refreshMetadata(e *Environment, tracker *GazelleTracker, IDStrings []string) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}
	// find ids in history
	var found []string
	for _, r := range e.History[tracker.Name].SnatchedReleases {
		if StringInSlice(r.TorrentID, IDStrings) {
			logThis.Info("Found release with ID "+r.TorrentID+" in history: "+r.ShortString()+". Getting tracker metadata.", NORMAL)
			// get data from RED.
			info, err := tracker.GetTorrentInfo(r.TorrentID)
			if err != nil {
				logThis.Error(errors.Wrap(err, errorCouldNotGetTorrentInfo), NORMAL)
				break
			}
			if e.inDaemon {
				go r.Metadata.SaveFromTracker(tracker, info, e.config.General.DownloadDir)
			} else {
				r.Metadata.SaveFromTracker(tracker, info, e.config.General.DownloadDir)
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
		if e.config.downloadFolderConfigured {
			for _, m := range missing {
				// get data from RED.
				info, err := tracker.GetTorrentInfo(m)
				if err != nil {
					logThis.Error(errors.Wrap(err, errorCouldNotGetTorrentInfo), NORMAL)
					break
				}
				fullFolder := filepath.Join(e.config.General.DownloadDir, html.UnescapeString(info.folder))
				if DirectoryExists(fullFolder) {
					r := info.Release()
					if e.inDaemon {
						go r.Metadata.SaveFromTracker(tracker, info, e.config.General.DownloadDir)
					} else {
						r.Metadata.SaveFromTracker(tracker, info, e.config.General.DownloadDir)
					}
				} else {
					logThis.Info(fmt.Sprintf(errorCannotFindID, m), NORMAL)
				}
			}
		} else {
			return fmt.Errorf(errorCannotFindID, strings.Join(missing, ","))
		}
	}
	return nil
}

func snatchTorrents(e *Environment, tracker *GazelleTracker, IDStrings []string) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}
	// snatch
	for _, id := range IDStrings {
		if release, err := snatchFromID(e, tracker, id); err != nil {
			return errors.New("Error snatching torrent with ID #" + id)
		} else {
			logThis.Info("Successfully snatched torrent "+release.ShortString(), NORMAL)
		}
	}
	return nil
}

func checkLog(tracker *GazelleTracker, logPaths []string) error {
	for _, log := range logPaths {
		score, err := tracker.GetLogScore(log)
		if err != nil {
			return errors.Wrap(err, errorGettingLogScore)
		}
		logThis.Info(fmt.Sprintf("Found score %s for log file %s.", score, log), NORMAL)
	}
	return nil
}

func archiveUserFiles() error {
	// generate Timestamp
	timestamp := time.Now().Format("2006-01-02_15h04m05s")
	archiveName := fmt.Sprintf(archiveNameTemplate, timestamp)
	if !DirectoryExists(archivesDir) {
		if err := os.MkdirAll(archivesDir, 0755); err != nil {
			logThis.Error(errors.Wrap(err, errorArchiving), NORMAL)
			return errors.Wrap(err, errorArchiving)
		}
	}
	// generate file
	err := archiver.Zip.Make(filepath.Join(archivesDir, archiveName), []string{statsDir, defaultConfigurationFile})
	if err != nil {
		logThis.Error(errors.Wrap(err, errorArchiving), NORMAL)
	}
	return err
}

func automaticBackup() {
	gocron.Every(1).Day().At("00:00").Do(archiveUserFiles)
	<-gocron.Start()
}
