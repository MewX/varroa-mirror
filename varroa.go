package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/gregdel/pushover"
	daemon "github.com/sevlyar/go-daemon"
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

	pidFile       = "varroa_pid"
	envPassphrase = "_VARROA_PASSPHRASE"
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

	if !daemon.WasReborn() {
		// here we're expecting output
		env.expectedOutput = true
	}

	// launching daemon
	if cli.start {
		// if using encrypted config file, ask for the passphrase and retrieve it from the daemon side
		if err := savePassphraseForDaemon(); err != nil {
			logThis(err.Error(), NORMAL)
			return
		}
		// daemonizing process
		env.daemon.Args = os.Args
		child, err := env.daemon.Reborn()
		if err != nil {
			logThis(errorGettingDaemonContext+err.Error(), NORMAL)
			return
		}
		if child != nil {
			logThis("Starting daemon...", NORMAL)
			return
		}
		// now in the daemon
		daemon.AddCommand(boolFlag(false), syscall.SIGTERM, quitDaemon)
		//defer daemonContext.Release()
		logThis("+ varroa musica started", NORMAL)
		env.inDaemon = true
		// setting up for the daemon
		if err := settingUp(); err != nil {
			logThis(errorSettingUp+err.Error(), NORMAL)
			return
		}
		// launch goroutines
		go ircHandler()
		go monitorStats()
		go apiCallRateLimiter()
		go webServer()
		go awaitOrders()
		go automaticBackup()

		if err := daemon.ServeSignals(); err != nil {
			logThis(errorServingSignals+err.Error(), NORMAL)
		}
		logThis("+ varroa musica stopped", NORMAL)
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
			if err := loadConfiguration(); err != nil {
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

	// assessing if daemon is running
	var daemonIsUp bool
	// trying to talk to existing daemon
	env.daemon.Args = os.Args
	d, searchErr := env.daemon.Search()
	if searchErr == nil {
		daemonIsUp = true
	}
	// at this point commands either require the daemon or can use it
	if daemonIsUp {
		// sending commands to the daemon through the unix socket
		if err := sendOrders(cli); err != nil {
			logThis(errorSendingCommandToDaemon+err.Error(), NORMAL)
			return
		}
		// at last, sending signals for shutdown
		if cli.stop {
			daemon.AddCommand(boolFlag(cli.stop), syscall.SIGTERM, quitDaemon)
			if err := daemon.SendCommands(d); err != nil {
				logThis(errorSendingSignal+err.Error(), NORMAL)
			}
			if err := env.daemon.Release(); err != nil {
				fmt.Println(errorReleasingDaemon + err.Error())
			}
			if err := os.Remove(pidFile); err != nil {
				fmt.Println(errorRemovingPID + err.Error())
			}
			return
		}
	} else {
		if cli.requiresDaemon {
			logThis(errorFindingDaemon+searchErr.Error(), NORMAL)
			fmt.Println(infoUsage)
			return
		}
		// setting up since the daemon isn't running
		if err := settingUp(); err != nil {
			logThis(errorSettingUp+err.Error(), NORMAL)
			return
		}
		// starting rate limiter
		go apiCallRateLimiter()
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
			if err := checkLog(cli.logFile); err != nil {
				logThis("Error checking log: "+err.Error(), NORMAL)
			}
		}
	}
	return
}

func quitDaemon(sig os.Signal) error {
	logThis("+ terminating", VERBOSE)
	return daemon.ErrStop
}

func settingUp() error {
	// load configuration
	if err := loadConfiguration(); err != nil {
		return errors.New(errorLoadingConfig + err.Error())
	}
	// prepare directory for stats if necessary
	if !DirectoryExists(statsDir) {
		if err := os.MkdirAll(statsDir, 0777); err != nil {
			return errors.New(errorCreatingStatsDir + err.Error())
		}
	}
	// init notifications with pushover
	if env.config.pushoverConfigured() {
		env.notification.client = pushover.New(env.config.pushover.token)
		env.notification.recipient = pushover.NewRecipient(env.config.pushover.user)
	}
	// log in tracker
	env.tracker = &GazelleTracker{rootURL: env.config.url}
	if err := env.tracker.Login(env.config.user, env.config.password); err != nil {
		return err
	}
	logThis("Logged in tracker.", NORMAL)
	// load history
	return env.history.LoadAll(statsFile, historyFile)
}

func savePassphraseForDaemon() error {
	encryptedConfigurationFile := strings.TrimSuffix(defaultConfigurationFile, yamlExt) + encryptedExt
	if !daemon.WasReborn() {
		// if necessary, ask for passphrase and add to env
		if !FileExists(defaultConfigurationFile) && FileExists(encryptedConfigurationFile) {
			stringPass, err := getPassphrase()
			if err != nil {
				return errors.New(errorGettingPassphrase + err.Error())
			}
			// testing
			copy(env.configPassphrase[:], stringPass)
			configBytes, err := decrypt(encryptedConfigurationFile, env.configPassphrase)
			if err != nil {
				return errors.New(errorLoadingConfig + err.Error())
			}
			newConf := &Config{}
			if err := newConf.loadFromBytes(configBytes); err != nil {
				return errors.New(errorLoadingConfig + err.Error())

			}
			// saving to env for the daemon to pick up later
			if err := os.Setenv(envPassphrase, stringPass); err != nil {
				return errors.New(errorSettingEnv + err.Error())

			}
		}
	} else {
		// getting passphrase from env if necessary
		if !FileExists(defaultConfigurationFile) && FileExists(encryptedConfigurationFile) {
			passphrase := os.Getenv(envPassphrase)
			copy(env.configPassphrase[:], passphrase)
		}
	}
	return nil
}
