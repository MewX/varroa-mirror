package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	varroaSocket = "varroa.sock"
)

func awaitOrders() {
	conn, err := net.Listen("unix", varroaSocket)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

Loop:
	for {
		c, err := conn.Accept()
		if err != nil {
			logThis("Error acceptin from unix socket: "+err.Error(), NORMAL)
			continue
		}

		buf := make([]byte, 512)
		n, err := c.Read(buf[:])
		if err != nil {
			logThis("Error reading from unix socket: "+err.Error(), NORMAL)
			continue
		}
		// NOTE: simple split, do something better if necessary
		fullCommand := strings.Split(string(buf[:n]), " ")
		if len(fullCommand) == 0 {
			continue
		}

		switch fullCommand[0] {
		case "stats":
			go func() {
				if err := generateStats(); err != nil {
					logThis(errorGeneratingGraphs+err.Error(), NORMAL)
				}
			}()
		case "stop":
			break Loop
		case "reload":
			go func() {
				if err := loadConfiguration(); err != nil {
					logThis("Error reloading", NORMAL)
				}
			}()

		case "refresh-metadata":
			go func() {
				if err := refreshMetadata(fullCommand[1:]); err != nil {
					logThis("Error refreshing metadata: "+err.Error(), NORMAL)
				}
			}()
		case "snatch":
			go func() {
				if err := snatchTorrents(fullCommand[1:]); err != nil {
					logThis("Error snatching torrents: "+err.Error(), NORMAL)
				}
			}()
		case "check-log":
			go func() {
				if err := checkLog(strings.Join(fullCommand[1:], " ")); err != nil {
					logThis("Error checking log: "+err.Error(), NORMAL)
				}
			}()

		}
		c.Close()
	}
}

func generateStats() error {
	logThis("- generating stats", VERBOSE)
	return history.GenerateGraphs()
}

func loadConfiguration() error {
	newConf := &Config{}
	if err := newConf.load("config.yaml"); err != nil {
		logThis(errorLoadingConfig+err.Error(), NORMAL)
		return err
	}
	conf = newConf
	logThis(" - Configuration reloaded.", NORMAL)
	disabledAutosnatching = false
	logThis(" - Autosnatching enabled.", NORMAL)
	// if server up
	thingsWentOK := true
	if serverHTTP.Addr != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := serverHTTP.Shutdown(ctx); err != nil {
			logThis(errorShuttingDownServer+err.Error(), NORMAL)
			thingsWentOK = false
		}
	}
	if serverHTTPS.Addr != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := serverHTTPS.Shutdown(ctx); err != nil {
			logThis(errorShuttingDownServer+err.Error(), NORMAL)
			thingsWentOK = false
		}
	}
	if thingsWentOK {
		// launch server again
		go webServer()
	}
	return nil
}

func refreshMetadata(IDStrings []string) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}
	// find ids in history
	var foundAtLeastOne bool
	for _, r := range history.SnatchedReleases {
		if StringInSlice(r.TorrentID, IDStrings) {
			foundAtLeastOne = true
			logThis("Found release with ID "+r.TorrentID+" in history: "+r.ShortString()+". Getting tracker metadata.", NORMAL)
			// get data from RED.
			info, err := tracker.GetTorrentInfo(r.TorrentID)
			if err != nil {
				logThis(errorCouldNotGetTorrentInfo, NORMAL)
			} else {
				saveTrackerMetadata(info)
			}
		}
	}
	if !foundAtLeastOne {
		return errors.New("Error: did not find matching ID(s) in history: " + strings.Join(IDStrings, ","))
	}
	return nil
}

func snatchTorrents(IDStrings []string) error {
	if len(IDStrings) == 0 {
		return errors.New("Error: no ID provided")
	}
	// snatch
	for _, id := range IDStrings {
		if release, err := snatchFromID(id); err != nil {
			return errors.New("Error snatching torrent with ID #" + id)
		} else {
			logThis("Successfully snatched torrent "+release.ShortString(), NORMAL)
		}
	}
	return nil
}

func checkLog(logPath string) error {
	score, err := tracker.GetLogScore(logPath)
	if err != nil {
		return errors.New("Error getting log score: " + err.Error())
	}
	logThis(fmt.Sprintf("Found score %s for log file %s.", score, logPath), NORMAL)
	return nil
}
