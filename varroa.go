package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

var logThis LogThis

func main() {
	env := NewEnvironment()
	logThis.env = env

	// parsing CLI
	cli := &varroaArguments{}
	if err := cli.parseCLI(os.Args[1:]); err != nil {
		logThis.Error(errors.Wrap(err, errorArguments), NORMAL)
		return
	}
	if cli.builtin {
		return
	}

	// here commands that have no use for the daemon
	if !cli.canUseDaemon {
		if cli.backup {
			if err := archiveUserFiles(); err == nil {
				logThis.Info(infoUserFilesArchived, NORMAL)
			}
			return
		}
		if cli.encrypt || cli.decrypt {
			// now dealing with encrypt/decrypt commands, which both require the passphrase from user
			if err := env.GetPassphrase(); err != nil {
				logThis.Error(errors.Wrap(err, "Error getting passphrase"), NORMAL)
			}
			if cli.encrypt {
				if err := env.config.Encrypt(defaultConfigurationFile, env.configPassphrase); err != nil {
					logThis.Info(err.Error(), NORMAL)
					return
				}
				logThis.Info(infoEncrypted, NORMAL)
			}
			if cli.decrypt {
				if err := env.config.DecryptTo(defaultConfigurationFile, env.configPassphrase); err != nil {
					logThis.Error(err, NORMAL)
					return
				}
				logThis.Info(infoDecrypted, NORMAL)
			}
			return
		}
		// commands that require the configuration
		// loading configuration
		if err := env.LoadConfiguration(); err != nil {
			logThis.Error(errors.Wrap(err, errorLoadingConfig), NORMAL)
			return
		}
		if cli.showConfig {
			fmt.Print("Found in configuration file: \n\n")
			fmt.Println(env.config)
			return
		}
		if cli.downloadScan {
			if err := env.Downloads.Load(filepath.Join(statsDir, downloadsDBFile+msgpackExt)); err != nil {
				logThis.Error(errors.Wrap(err, "Error loading downloads database"), NORMAL)
				return
			}
			if err := env.Downloads.Scan(); err != nil {
				logThis.Error(errors.Wrap(err, "Error scanning downloads"), NORMAL)
				return
			}
			if err := env.Downloads.Save(); err != nil {
				logThis.Error(errors.Wrap(err, "Error saving downloads database"), NORMAL)
				return
			}
			return
		}
	}

	// loading configuration
	if err := env.LoadConfiguration(); err != nil {
		logThis.Error(errors.Wrap(err, errorLoadingConfig), NORMAL)
		return
	}

	// launching daemon
	if cli.start {
		// daemonizing process
		if err := env.Daemonize(os.Args); err != nil {
			logThis.Error(errors.Wrap(err, errorGettingDaemonContext), NORMAL)
			return
		}
		// if not in daemon, job is over; exiting.
		// the spawned daemon will continue.
		if !env.inDaemon {
			return
		}
		// setting up for the daemon
		if err := env.SetUp(true); err != nil {
			logThis.Error(errors.Wrap(err, errorSettingUp), NORMAL)
			return
		}
		// launch goroutines
		goGoRoutines(env)

		// wait until daemon is stopped.
		env.WaitForDaemonStop()
		return
	}

	// at this point commands either require the daemon or can use it
	// assessing if daemon is running
	daemonProcess, err := env.FindDaemon()
	if err != nil {
		// no daemon found, running commands directly.
		if cli.requiresDaemon {
			logThis.Error(errors.Wrap(err, errorFindingDaemon), NORMAL)
			fmt.Println(infoUsage)
			return
		}
		// setting up since the daemon isn't running
		if err := env.SetUp(false); err != nil {
			logThis.Error(errors.Wrap(err, errorSettingUp), NORMAL)
			return
		}

		// general commands
		if cli.stats {
			if err := generateStats(env); err != nil {
				logThis.Error(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
			}
			return
		}

		// commands that require tracker label
		tracker, err := env.Tracker(cli.trackerLabel)
		if err != nil {
			logThis.Info(fmt.Sprintf("Tracker %s not defined in configuration file", cli.trackerLabel), NORMAL)
			return
		}
		if cli.refreshMetadata {
			if err := refreshMetadata(env, tracker, IntSliceToStringSlice(cli.torrentIDs)); err != nil {
				logThis.Error(errors.Wrap(err, errorRefreshingMetadata), NORMAL)
			}
		}
		if cli.snatch {
			if err := snatchTorrents(env, tracker, IntSliceToStringSlice(cli.torrentIDs)); err != nil {
				logThis.Error(errors.Wrap(err, errorSnatchingTorrent), NORMAL)
			}
		}
		if cli.checkLog {
			if err := checkLog(tracker, []string{cli.logFile}); err != nil {
				logThis.Error(errors.Wrap(err, errorCheckingLog), NORMAL)
			}
		}
	} else {
		// daemon is up, sending commands to the daemon through the unix socket
		if err := sendOrders(cli); err != nil {
			logThis.Error(errors.Wrap(err, errorSendingCommandToDaemon), NORMAL)
			return
		}
		// at last, sending signals for shutdown
		if cli.stop {
			env.StopDaemon(daemonProcess)
			return
		}
	}
	return
}

func goGoRoutines(e *Environment) {
	//  tracker-dependent goroutines
	for _, t := range e.Trackers {
		if e.config.autosnatchConfigured {
			go ircHandler(e, t)
		}
	}
	// general goroutines
	if e.config.statsConfigured {
		go monitorAllStats(e)
	}
	if e.config.webserverConfigured {
		go webServer(e, e.serverHTTP, e.serverHTTPS)
	}
	// background goroutines
	go awaitOrders(e)
	go automaticBackup()
}
