package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gitlab.com/passelecasque/varroa"
)

var logThis *varroa.LogThis

func main() {
	env := varroa.NewEnvironment()
	logThis = varroa.NewLogThis(env)

	// parsing CLI
	cli := &varroaArguments{}
	if err := cli.parseCLI(os.Args[1:]); err != nil {
		logThis.Error(errors.Wrap(err, varroa.ErrorArguments), varroa.NORMAL)
		return
	}
	if cli.builtin {
		return
	}

	// here commands that have no use for the daemon
	if !cli.canUseDaemon {
		if cli.backup {
			if err := varroa.ArchiveUserFiles(); err == nil {
				logThis.Info(varroa.InfoUserFilesArchived, varroa.NORMAL)
			}
			return
		}
		if cli.encrypt || cli.decrypt {
			// now dealing with encrypt/decrypt commands, which both require the passphrase from user
			if err := env.GetPassphrase(); err != nil {
				logThis.Error(errors.Wrap(err, "Error getting passphrase"), varroa.NORMAL)
			}
			if cli.encrypt {
				if err := env.Config.Encrypt(varroa.DefaultConfigurationFile, env.ConfigPassphrase); err != nil {
					logThis.Info(err.Error(), varroa.NORMAL)
					return
				}
				logThis.Info(varroa.InfoEncrypted, varroa.NORMAL)
			}
			if cli.decrypt {
				if err := env.Config.DecryptTo(varroa.DefaultConfigurationFile, env.ConfigPassphrase); err != nil {
					logThis.Error(err, varroa.NORMAL)
					return
				}
				logThis.Info(varroa.InfoDecrypted, varroa.NORMAL)
			}
			return
		}
		// commands that require the configuration
		// loading configuration
		if err := env.LoadConfiguration(); err != nil {
			logThis.Error(errors.Wrap(err, varroa.ErrorLoadingConfig), varroa.NORMAL)
			return
		}
		if cli.showConfig {
			fmt.Print("Found in configuration file: \n\n")
			fmt.Println(env.Config)
			return
		}
		if cli.downloadScan || cli.downloadSearch || cli.downloadInfo || cli.downloadSort || cli.downloadList || cli.downloadClean {
			if !env.Config.DownloadFolderConfigured {
				logThis.Error(errors.New("Cannot scan for downloads, downloads folder not configured"), varroa.NORMAL)
				return
			}
			downloads := varroa.Downloads{Root: env.Config.General.DownloadDir}
			// simple operation, only requires access to download folder, since it will clean unindexed folders
			if cli.downloadClean {
				if err := downloads.Clean(); err != nil {
					logThis.Error(err, varroa.NORMAL)
				} else {
					fmt.Println("Downloads directory cleaned of empty folders & folders containing only tracker metadata.")
				}
				return
			}
			// scanning
			fmt.Println(varroa.Green("Scanning downloads for new releases and updated metadata."))
			if err := downloads.LoadAndScan(filepath.Join(varroa.StatsDir, varroa.DownloadsDBFile+".db")); err != nil {
				logThis.Error(err, varroa.NORMAL)
				return
			}
			defer downloads.Save()

			if cli.downloadScan {
				fmt.Println(downloads.String())
				return
			}
			if cli.downloadSearch {
				hits := downloads.Releases.FilterArtist(cli.artistName)
				if len(hits) == 0 {
					fmt.Println("Nothing found.")
				} else {
					for _, dl := range hits {
						fmt.Println(dl.ShortString())
					}
				}
				return
			}
			if cli.downloadList {
				hits := downloads.FindByState(cli.downloadState)
				if len(hits) == 0 {
					fmt.Println("Nothing found.")
				} else {
					for _, dl := range hits {
						fmt.Println(dl.ShortString())
					}
				}
				return
			}
			if cli.downloadInfo {
				dl, err := downloads.FindByID(uint64(cli.torrentIDs[0]))
				if err != nil {
					logThis.Error(errors.Wrap(err, "Error finding such an ID in the downloads database"), varroa.NORMAL)
					return
				}
				fmt.Println(dl.Description())
				return
			}
			if cli.downloadSort {
				// setting up to load history, etc.
				if err := env.SetUp(false); err != nil {
					logThis.Error(errors.Wrap(err, varroa.ErrorSettingUp), varroa.NORMAL)
					return
				}

				if !env.Config.LibraryConfigured {
					logThis.Error(errors.New("Cannot sort downloads, library is not configured"), varroa.NORMAL)
					return
				}
				if len(cli.torrentIDs) == 0 {
					fmt.Println("Considering new or unsorted downloads.")
					if err := downloads.Sort(env); err != nil {
						logThis.Error(errors.Wrap(err, "Error sorting downloads"), varroa.NORMAL)
						return
					}
				} else {
					dl, err := downloads.FindByID(uint64(cli.torrentIDs[0]))
					if err != nil {
						logThis.Error(errors.Wrap(err, "Error finding such an ID in the downloads database"), varroa.NORMAL)
						return
					}
					if err := dl.Sort(env); err != nil {
						logThis.Error(errors.Wrap(err, "Error sorting selected download"), varroa.NORMAL)
						return
					}
				}
				return
			}
		}
		// using stormDB
		if cli.downloadFuse {
			logThis.Info("Mounting FUSE filesystem in "+cli.mountPoint, varroa.NORMAL)
			if err := varroa.FuseMount(env.Config.General.DownloadDir, cli.mountPoint); err != nil {
				logThis.Error(err, varroa.NORMAL)
				return
			}
			logThis.Info("Unmounting FUSE filesystem, fusermount -u has presumably been called.", varroa.VERBOSE)
			return
		}

	}

	// loading configuration
	if err := env.LoadConfiguration(); err != nil {
		logThis.Error(errors.Wrap(err, varroa.ErrorLoadingConfig), varroa.NORMAL)
		return
	}

	// launching daemon
	if cli.start {
		// daemonizing process
		if err := env.Daemonize(os.Args); err != nil {
			logThis.Error(errors.Wrap(err, varroa.ErrorGettingDaemonContext), varroa.NORMAL)
			return
		}
		// if not in daemon, job is over; exiting.
		// the spawned daemon will continue.
		if !env.InDaemon {
			return
		}
		// setting up for the daemon
		if err := env.SetUp(true); err != nil {
			logThis.Error(errors.Wrap(err, varroa.ErrorSettingUp), varroa.NORMAL)
			return
		}
		// launch goroutines
		env.GoGoRoutines()

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
			logThis.Error(errors.Wrap(err, varroa.ErrorFindingDaemon), varroa.NORMAL)
			fmt.Println(varroa.InfoUsage)
			return
		}
		// setting up since the daemon isn't running
		if err := env.SetUp(false); err != nil {
			logThis.Error(errors.Wrap(err, varroa.ErrorSettingUp), varroa.NORMAL)
			return
		}

		// general commands
		if cli.stats {
			if err := varroa.GenerateStats(env); err != nil {
				logThis.Error(errors.Wrap(err, varroa.ErrorGeneratingGraphs), varroa.NORMAL)
			}
			return
		}

		// commands that require tracker label
		tracker, err := env.Tracker(cli.trackerLabel)
		if err != nil {
			logThis.Info(fmt.Sprintf("Tracker %s not defined in configuration file", cli.trackerLabel), varroa.NORMAL)
			return
		}
		if cli.refreshMetadata {
			if err := varroa.RefreshMetadata(env, tracker, varroa.IntSliceToStringSlice(cli.torrentIDs)); err != nil {
				logThis.Error(errors.Wrap(err, varroa.ErrorRefreshingMetadata), varroa.NORMAL)
			}
		}
		if cli.snatch {
			if err := varroa.SnatchTorrents(env, tracker, varroa.IntSliceToStringSlice(cli.torrentIDs), cli.useFLToken); err != nil {
				logThis.Error(errors.Wrap(err, varroa.ErrorSnatchingTorrent), varroa.NORMAL)
			}
		}
		if cli.info {
			if err := varroa.ShowTorrentInfo(env, tracker, varroa.IntSliceToStringSlice(cli.torrentIDs)); err != nil {
				logThis.Error(errors.Wrap(err, varroa.ErrorShowingTorrentInfo), varroa.NORMAL)
			}
		}
		if cli.checkLog {
			if err := varroa.CheckLog(tracker, []string{cli.logFile}); err != nil {
				logThis.Error(errors.Wrap(err, varroa.ErrorCheckingLog), varroa.NORMAL)
			}
		}
	} else {
		// daemon is up, sending commands to the daemon through the unix socket
		if err := varroa.SendOrders(cli.commandToDaemon()); err != nil {
			logThis.Error(errors.Wrap(err, varroa.ErrorSendingCommandToDaemon), varroa.NORMAL)
			return
		}
		// at last, sending signals for shutdown
		if cli.stop {
			env.Notify("Stopping daemon!", "varroa daemon", "info")
			env.StopDaemon(daemonProcess)
			return
		}
	}
	return
}


