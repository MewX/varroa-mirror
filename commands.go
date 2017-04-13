package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/mholt/archiver"
)

const (
	varroaSocket               = "varroa.sock"
	archivesDir                = "archives"
	archiveNameTemplate        = "varroa_%s.zip"
	defaultConfigurationFile   = "config.yaml"
	unixSocketMessageSeparator = "↑" // because it looks nice

	errorArchiving         = "Error while archiving user files: "
	errorDialingSocket     = "Error dialing to unix socket: "
	errorWritingToSocket   = "Error writing to unix socket: "
	errorReadingFromSocket = "Error reading from unix socket: "
)

func sendOrders(cli *varroaArguments) error {
	conn, err := net.Dial("unix", varroaSocket)
	if err != nil {
		return errors.New(errorDialingSocket + err.Error())
	}
	// sending command
	if _, err = conn.Write([]byte(cli.commandToDaemon())); err != nil {
		return errors.New(errorWritingToSocket + err.Error())
	}
Loop:
	for {
		// read answer
		buf := make([]byte, 512)
		n, err := conn.Read(buf[:])
		if err != nil {
			return errors.New(errorReadingFromSocket + err.Error())
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
		panic(err)
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
		expectedOutput = true

		// this goroutine will send back messages to the instance that sent the command
		go func() {
			for {
				messageToLog := <-sendBackToCLI
				// writing to socket with a separator, so that the other instance, reading more slowly,
				// can separate messages that might have been written one after the other
				if _, err = c.Write([]byte(messageToLog + unixSocketMessageSeparator)); err != nil {
					logThis("Error writing to unix socket: "+err.Error(), NORMAL)
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
			logThis("Error reading from unix socket: "+err.Error(), NORMAL)
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
				logThis(errorGeneratingGraphs+err.Error(), NORMAL)
			}
		case "stop":
			logThis("Stopping daemon...", NORMAL)
			stopEverything = true
		case "reload":
			if err := loadConfiguration(); err != nil {
				logThis("Error reloading", NORMAL)
			}
		case "refresh-metadata":
			if err := refreshMetadata(fullCommand[1:]); err != nil {
				logThis("Error refreshing metadata: "+err.Error(), NORMAL)
			}
		case "snatch":
			if err := snatchTorrents(fullCommand[1:]); err != nil {
				logThis("Error snatching torrents: "+err.Error(), NORMAL)
			}
		case "check-log":
			if err := checkLog(strings.Join(fullCommand[1:], " ")); err != nil {
				logThis("Error checking log: "+err.Error(), NORMAL)
			}
		}
		sendBackToCLI <- "stop"
		// waiting for the other instance to be warned that communication is over
		<-endThisConnection
		c.Close()
		expectedOutput = false
		if stopEverything {
			// shutting down the daemon, exiting look for socket cleanup
			break
		}
	}
}

func generateStats() error {
	logThis("Generating stats", VERBOSE)
	return history.GenerateGraphs()
}

func loadConfiguration() error {
	newConf := &Config{}

	// if using encrypted file
	encryptedConfigurationFile := strings.TrimSuffix(defaultConfigurationFile, yamlExt) + encryptedExt
	if FileExists(encryptedConfigurationFile) && !FileExists(defaultConfigurationFile) {
		// if this env variable is set, we're using the encrypted config file and already have the passphrase
		if !inDaemon && os.Getenv(envPassphrase) == "" {
			// getting passphrase from user
			passphrase, err := getPassphrase()
			if err != nil {
				return err
			}
			copy(configPassphrase[:], passphrase)
		}
		configBytes, err := decrypt(encryptedConfigurationFile, configPassphrase)
		if err != nil {
			return err
		}
		if err := newConf.loadFromBytes(configBytes); err != nil {
			logThis(errorLoadingConfig+err.Error(), NORMAL)
			return err
		}
	} else {
		if err := newConf.load(defaultConfigurationFile); err != nil {
			logThis(errorLoadingConfig+err.Error(), NORMAL)
			return err
		}
	}
	if conf.user != "" {
		// if conf.user exists, the configuration had been loaded previously
		logThis("Configuration reloaded.", NORMAL)
	}
	conf = newConf
	if disabledAutosnatching {
		disabledAutosnatching = false
		logThis("Autosnatching enabled.", NORMAL)
	}
	// if server up
	thingsWentOK := true
	serverWasUp := false
	if serverHTTP.Addr != "" {
		serverWasUp = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := serverHTTP.Shutdown(ctx); err != nil {
			logThis(errorShuttingDownServer+err.Error(), NORMAL)
			thingsWentOK = false
		}
	}
	if serverHTTPS.Addr != "" {
		serverWasUp = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := serverHTTPS.Shutdown(ctx); err != nil {
			logThis(errorShuttingDownServer+err.Error(), NORMAL)
			thingsWentOK = false
		}
	}
	if serverWasUp && thingsWentOK {
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
			break
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

func archiveUserFiles() error {
	// generate Timestamp
	timestamp := time.Now().Format("2006-01-02_15h04m05s")
	archiveName := fmt.Sprintf(archiveNameTemplate, timestamp)
	if !DirectoryExists(archivesDir) {
		if err := os.MkdirAll(archivesDir, 0755); err != nil {
			logThis(errorArchiving+err.Error(), NORMAL)
			return err
		}
	}
	// generate file
	err := archiver.Zip.Make(filepath.Join(archivesDir, archiveName), []string{statsDir, defaultConfigurationFile})
	if err != nil {
		logThis(errorArchiving+err.Error(), NORMAL)
	}
	return err
}

func automaticBackup() {
	gocron.Every(1).Day().At("00:00").Do(archiveUserFiles)
	<-gocron.Start()
}
