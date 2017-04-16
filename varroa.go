package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

const (
	varroa = "varroa musica"

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
		logThisError(errors.Wrap(err, errorArguments), NORMAL)
		return
	}
	if cli.builtin {
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
			// loading configuration
			if err := env.LoadConfiguration(); err != nil {
				logThisError(errors.Wrap(err, errorLoadingConfig), NORMAL)
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

	// loading configuration
	if err := env.LoadConfiguration(); err != nil {
		logThisError(errors.Wrap(err, errorLoadingConfig), NORMAL)
		return
	}

	// launching daemon
	if cli.start {
		// daemonizing process
		if err := env.Daemonize(os.Args); err != nil {
			logThisError(errors.Wrap(err, errorGettingDaemonContext), NORMAL)
			return
		}
		// if not in daemon, job is over; exiting.
		// the spawned daemon will continue.
		if !env.inDaemon {
			return
		}
		// setting up for the daemon
		if err := env.SetUp(); err != nil {
			logThisError(errors.Wrap(err, errorSettingUp), NORMAL)
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

	// at this point commands either require the daemon or can use it
	// assessing if daemon is running
	daemonProcess, err := env.FindDaemon()
	if err != nil {
		// no daemon found, running commands directly.
		if cli.requiresDaemon {
			logThisError(errors.Wrap(err, errorFindingDaemon), NORMAL)
			fmt.Println(infoUsage)
			return
		}
		// setting up since the daemon isn't running
		if err := env.SetUp(); err != nil {
			logThisError(errors.Wrap(err, errorSettingUp), NORMAL)
			return
		}
		// starting rate limiter
		go apiCallRateLimiter(env.limiter)
		// running the command
		if cli.stats {
			if err := generateStats(); err != nil {
				logThisError(errors.Wrap(err, errorGeneratingGraphs), NORMAL)
			}
		}
		if cli.refreshMetadata {
			if err := refreshMetadata(IntSliceToStringSlice(cli.torrentIDs)); err != nil {
				logThisError(errors.Wrap(err, errorRefreshingMetadata), NORMAL)
			}
		}
		if cli.snatch {
			if err := snatchTorrents(IntSliceToStringSlice(cli.torrentIDs)); err != nil {
				logThisError(errors.Wrap(err, errorSnatchingTorrent), NORMAL)
			}
		}
		if cli.checkLog {
			if err := checkLog(cli.logFile, env.tracker); err != nil {
				logThisError(errors.Wrap(err, errorCheckingLog), NORMAL)
			}
		}
	} else {
		// daemon is up, sending commands to the daemon through the unix socket
		if err := sendOrders(cli); err != nil {
			logThisError(errors.Wrap(err, errorSendingCommandToDaemon), NORMAL)
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
