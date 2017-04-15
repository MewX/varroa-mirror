package main

import (
	"fmt"
	"os"
)

const (
	varroa = "varroa musica"

	errorLoadingConfig          = "Error loading configuration: "
	errorServingSignals         = "Error serving signals: "
	errorFindingDaemon          = "Error finding daemon: "
	errorReleasingDaemon        = "Error releasing daemon: "
	errorSendingSignal          = "Error sending signal to the daemon: "
	errorGettingDaemonContext   = "Error launching daemon: "
	errorCreatingStatsDir       = "Error creating stats directory: "
	errorShuttingDownServer     = "Error shutting down web server: "
	errorArguments              = "Error parsing command line arguments: "
	errorSendingCommandToDaemon = "Error sending command to daemon: "
	errorRemovingPID            = "Error removing pid file: "
	errorSettingUp              = "Error setting up: "
	errorGettingPassphrase      = "Error getting passphrase: "
	errorSettingEnv             = "Could not set env variable: "

	infoUserFilesArchived = "User files backed up."
	infoUsage             = "Before running a command that requires the daemon, run 'varroa start'."
	infoEncrypted         = "Configuration file encrypted. You can use this encrypted version in place of the unencrypted version."
	infoDecrypted         = "Configuration file has been decrypted to a plaintext YAML file."

	pidFile = "varroa_pid"
)

var (
	env *Environment
)

type boolFlag bool

func (b boolFlag) IsSet() bool {
	return bool(b)
}

func init() {
	env = NewEnvironment()
}

func main() {
	// parsing CLI
	cli := &varroaArguments{}
	if err := cli.parseCLI(os.Args[1:]); err != nil {
		fmt.Println(errorArguments+err.Error(), NORMAL)
		return
	}
	if cli.builtin {
		return
	}

	// launching daemon
	if cli.start {
		// if using encrypted config file, ask for the passphrase and retrieve it from the daemon side
		if err := env.SavePassphraseForDaemon(); err != nil {
			logThis(err.Error(), NORMAL)
			return
		}
		// daemonizing process
		if err := env.Daemonize(os.Args); err != nil {
			logThis(errorGettingDaemonContext+err.Error(), NORMAL)
			return
		}
		// if not in daemon, job is over; exiting.
		// the spawned daemon will continue.
		if !env.inDaemon {
			return
		}
		// setting up for the daemon
		if err := env.SetUp(); err != nil {
			logThis(errorSettingUp+err.Error(), NORMAL)
			return
		}
		// launch goroutines
		go ircHandler(env.config, env.tracker)
		go monitorStats(env.config, env.tracker)
		go apiCallRateLimiter(env.limiter)
		go webServer(env.config, env.serverHTTP, env.serverHTTPS)
		go awaitOrders()
		go automaticBackup()
		// wait until daemon is stopped.
		env.WaitForDaemonStop()
		return
	}

	// here commands that have no use for the daemon
	if !cli.canUseDaemon {
		if cli.backup {
			if err := archiveUserFiles(); err == nil {
				logThis(infoUserFilesArchived, NORMAL)
			}
		}
		if cli.showFilters {
			// load configuration
			if err := env.Reload(); err != nil {
				logThis(errorLoadingConfig+err.Error(), NORMAL)
				return
			}
			fmt.Print("Filters found in configuration file: \n\n")
			for _, f := range env.config.filters {
				fmt.Println(f)
			}
		}
		if cli.encrypt {
			if err := env.config.encrypt(); err != nil {
				logThis(err.Error(), NORMAL)
				return
			}
			logThis(infoEncrypted, NORMAL)
		}
		if cli.decrypt {
			if err := env.config.decrypt(); err != nil {
				logThis(err.Error(), NORMAL)
				return
			}
			logThis(infoDecrypted, NORMAL)
		}
		return
	}

	// at this point commands either require the daemon or can use it
	// assessing if daemon is running
	daemonProcess, err := env.FindDaemon()
	if err != nil {
		// no daemon found, running commands directly.
		if cli.requiresDaemon {
			logThis(errorFindingDaemon+err.Error(), NORMAL)
			fmt.Println(infoUsage)
			return
		}
		// setting up since the daemon isn't running
		if err := env.SetUp(); err != nil {
			logThis(errorSettingUp+err.Error(), NORMAL)
			return
		}
		// starting rate limiter
		go apiCallRateLimiter(env.limiter)
		// running the command
		if cli.stats {
			if err := generateStats(); err != nil {
				logThis(errorGeneratingGraphs+err.Error(), NORMAL)
			}
		}
		if cli.refreshMetadata {
			if err := refreshMetadata(IntSliceToStringSlice(cli.torrentIDs)); err != nil {
				logThis("Error refreshing metadata: "+err.Error(), NORMAL)
			}
		}
		if cli.snatch {
			if err := snatchTorrents(IntSliceToStringSlice(cli.torrentIDs)); err != nil {
				logThis("Error snatching torrents: "+err.Error(), NORMAL)
			}
		}
		if cli.checkLog {
			if err := checkLog(cli.logFile, env.tracker); err != nil {
				logThis("Error checking log: "+err.Error(), NORMAL)
			}
		}
	} else {
		// daemon is up, sending commands to the daemon through the unix socket
		if err := sendOrders(cli); err != nil {
			logThis(errorSendingCommandToDaemon+err.Error(), NORMAL)
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
