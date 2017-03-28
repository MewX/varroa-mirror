package main

import (
	"context"
	"errors"
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
	if server.Addr != "" {
		// shut down gracefully, but wait no longer than 5 seconds before halting
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logThis(errorShuttingDownServer+err.Error(), NORMAL)
		}
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
