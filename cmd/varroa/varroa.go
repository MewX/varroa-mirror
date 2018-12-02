package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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

	// prepare cleanup
	defer closeDB()

	// here commands that have no use for the daemon
	if !cli.canUseDaemon {
		if cli.backup {
			if err := varroa.ArchiveUserFiles(); err == nil {
				logThis.Info(varroa.InfoUserFilesArchived, varroa.NORMAL)
			}
			return
		}
		// loading configuration
		config, err := varroa.NewConfig(varroa.DefaultConfigurationFile)
		if err != nil {
			logThis.Error(errors.Wrap(err, varroa.ErrorLoadingConfig), varroa.NORMAL)
			return
		}
		env.SetConfig(config)

		if cli.encrypt || cli.decrypt {
			// now dealing with encrypt/decrypt commands, which both require the passphrase from user
			passphrase, err := varroa.GetPassphrase()
			if err != nil {
				logThis.Error(errors.Wrap(err, "Error getting passphrase"), varroa.NORMAL)
			}
			passphraseBytes := make([]byte, 32)
			copy(passphraseBytes[:], passphrase)
			if cli.encrypt {
				if err = config.Encrypt(varroa.DefaultConfigurationFile, passphraseBytes); err != nil {
					logThis.Info(err.Error(), varroa.NORMAL)
					return
				}
				logThis.Info(varroa.InfoEncrypted, varroa.NORMAL)
			}
			if cli.decrypt {
				if err = config.DecryptTo(varroa.DefaultConfigurationFile, passphraseBytes); err != nil {
					logThis.Error(err, varroa.NORMAL)
					return
				}
				logThis.Info(varroa.InfoDecrypted, varroa.NORMAL)
			}
			return
		}
		if cli.showConfig {
			fmt.Print("Found in configuration file: \n\n")
			fmt.Println(config)
			return
		}
		if cli.downloadSearch || cli.downloadInfo || cli.downloadSort || cli.downloadSortID || cli.downloadList || cli.downloadClean {
			if !config.DownloadFolderConfigured {
				logThis.Error(errors.New("Cannot scan for downloads, downloads folder not configured"), varroa.NORMAL)
				return
			}
			var additionalSources []string
			if config.LibraryConfigured {
				additionalSources = config.Library.AdditionalSources
			}
			downloads, err := varroa.NewDownloadsDB(varroa.DefaultDownloadsDB, config.General.DownloadDir, additionalSources)
			if err != nil {
				logThis.Error(err, varroa.NORMAL)
				return
			}
			defer downloads.Close()

			// simple operation, only requires access to download folder, since it will clean unindexed folders
			if cli.downloadClean {
				if err = downloads.Clean(); err != nil {
					logThis.Error(err, varroa.NORMAL)
				} else {
					fmt.Println("Downloads directory cleaned of empty folders & folders containing only tracker metadata.")
				}
				return
			}
			if cli.downloadSort || cli.downloadSortID {
				// setting up to load history, etc.
				if err = env.SetUp(false); err != nil {
					logThis.Error(errors.Wrap(err, varroa.ErrorSettingUp), varroa.NORMAL)
					return
				}
				if !config.LibraryConfigured {
					logThis.Error(errors.New("Cannot sort downloads, library is not configured"), varroa.NORMAL)
					return
				}
				// if no argument, sort everything
				if (cli.downloadSortID && len(cli.torrentIDs) == 0) || (cli.downloadSort && len(cli.paths) == 0) {
					// scanning
					fmt.Println(varroa.Green("Scanning downloads for new releases and updated metadata."))
					if err = downloads.Scan(); err != nil {
						logThis.Error(err, varroa.NORMAL)
						return
					}
					defer downloads.Close()
					fmt.Println("Considering new or unsorted downloads.")
					if err = downloads.Sort(env); err != nil {
						logThis.Error(errors.Wrap(err, "Error sorting downloads"), varroa.NORMAL)
						return
					}
					return
				}
				if cli.downloadSort {
					// scanning
					fmt.Println(varroa.Green("Scanning downloads for updated metadata."))
					for _, p := range cli.paths {
						if err = downloads.RescanPath(p); err != nil {
							logThis.Error(err, varroa.NORMAL)
							return
						}
						dl, err := downloads.FindByFolderName(p)
						if err != nil {
							logThis.Error(errors.Wrap(err, "error looking for "), varroa.NORMAL)
							return
						}
						cli.torrentIDs = append(cli.torrentIDs, dl.ID)
					}
				} else {
					// scanning
					fmt.Println(varroa.Green("Scanning downloads for updated metadata."))
					if err = downloads.RescanIDs(cli.torrentIDs); err != nil {
						logThis.Error(err, varroa.NORMAL)
						return
					}
				}
				fmt.Println("Sorting specific download folders.")
				for _, id := range cli.torrentIDs {
					if err = downloads.SortThisID(env, id); err != nil {
						logThis.Error(err, varroa.NORMAL)
					}
				}
				return
			}

			// all subsequent commands require scanning
			fmt.Println(varroa.Green("Scanning downloads for new releases and updated metadata."))
			if err = downloads.Scan(); err != nil {
				logThis.Error(err, varroa.NORMAL)
				return
			}
			defer downloads.Close()

			if cli.downloadSearch {
				hits := downloads.FindByArtist(cli.artistName)
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
				if cli.downloadState == "" {
					fmt.Println(downloads.String())
				} else {
					hits := downloads.FindByState(cli.downloadState)
					if len(hits) == 0 {
						fmt.Println("Nothing found.")
					} else {
						for _, dl := range hits {
							fmt.Println(dl.ShortString())
						}
					}
				}
				return
			}
			if cli.downloadInfo {
				dl, err := downloads.FindByID(cli.torrentIDs[0])
				if err != nil {
					logThis.Error(errors.Wrap(err, "Error finding such an ID in the downloads database"), varroa.NORMAL)
					return
				}
				fmt.Println(dl.Description(config.General.DownloadDir))
				return
			}
		}
		if cli.libraryReorg {
			if !config.LibraryConfigured {
				logThis.Info("Library is not configured, missing relevant configuration section.", varroa.NORMAL)
				return
			}
			logThis.Info("Reorganizing releases in the library directory. ", varroa.NORMAL)
			if cli.libraryReorgSimulate {
				fmt.Println(varroa.Green("This will simulate the library reorganization, applying the library folder template to all releases, using known tracker metadata. Nothing will actually be renamed or moved."))
			} else {
				fmt.Println(varroa.Green("This will apply the library folder template to all releases, using known tracker metadata. It will overwrite any specific name that may have been set manually."))
			}
			if varroa.Accept("Confirm") {
				if err = varroa.ReorganizeLibrary(cli.libraryReorgSimulate, cli.libraryReorgInteractive); err != nil {
					logThis.Error(err, varroa.NORMAL)
				}
			}
			return
		}
		// using stormDB
		if cli.downloadFuse {
			logThis.Info("Mounting FUSE filesystem in "+cli.mountPoint, varroa.NORMAL)
			if err = varroa.FuseMount(config.General.DownloadDir, cli.mountPoint, varroa.DefaultDownloadsDB); err != nil {
				logThis.Error(err, varroa.NORMAL)
				return
			}
			logThis.Info("Unmounting FUSE filesystem, fusermount -u has presumably been called.", varroa.VERBOSE)
			return
		}
		if cli.libraryFuse {
			if !config.LibraryConfigured {
				logThis.Info("Cannot mount FUSE filesystem for the library, missing relevant configuration section.", varroa.NORMAL)
				return
			}
			logThis.Info("Mounting FUSE filesystem in "+cli.mountPoint, varroa.NORMAL)
			if err = varroa.FuseMount(config.Library.Directory, cli.mountPoint, varroa.DefaultLibraryDB); err != nil {
				logThis.Error(err, varroa.NORMAL)
				return
			}
			logThis.Info("Unmounting FUSE filesystem, fusermount -u has presumably been called.", varroa.VERBOSE)
			return
		}
	}

	// loading configuration
	if err := env.LoadConfiguration(); err != nil {
		fmt.Println(errors.Wrap(err, varroa.ErrorLoadingConfig).Error())
		return
	}

	d := varroa.NewDaemon()
	// launching daemon
	if cli.start {
		// daemonizing process
		if err := d.Start(os.Args); err != nil {
			logThis.Error(errors.Wrap(err, varroa.ErrorGettingDaemonContext), varroa.NORMAL)
			return
		}
		// if not in daemon, job is over; exiting.
		// the spawned daemon will continue.
		if !d.IsRunning() {
			return
		}
		// setting up for the daemon
		if err := env.SetUp(true); err != nil {
			logThis.Error(errors.Wrap(err, varroa.ErrorSettingUp), varroa.NORMAL)
			return
		}
		// launch goroutines
		varroa.GoGoRoutines(env)

		// wait until daemon is stopped.
		d.WaitForStop()
		return
	}

	// at this point commands either require the daemon or can use it
	// assessing if daemon is running
	daemonProcess, err := d.Find()
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
		if cli.refreshMetadata {
			for _, r := range cli.toRefresh {
				tracker, err := env.Tracker(r.tracker)
				if err != nil {
					logThis.Info(fmt.Sprintf("Tracker %s not defined in configuration file", cli.trackerLabel), varroa.NORMAL)
					return
				}
				if err = varroa.RefreshLibraryMetadata(r.path, tracker, strconv.Itoa(r.id)); err != nil {
					logThis.Error(errors.Wrap(err, varroa.ErrorRefreshingMetadata), varroa.NORMAL)
				}
			}
			return
		}

		// commands that require tracker label
		tracker, err := env.Tracker(cli.trackerLabel)
		if err != nil {
			logThis.Info(fmt.Sprintf("Tracker %s not defined in configuration file", cli.trackerLabel), varroa.NORMAL)
			return
		}
		if cli.refreshMetadataByID {
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
		if cli.reseed {
			if err := varroa.Reseed(tracker, cli.paths); err != nil {
				logThis.Error(errors.Wrap(err, varroa.ErrorReseed), varroa.NORMAL)
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
			varroa.Notify("Stopping daemon!", varroa.FullName, "info")
			d.Stop(daemonProcess)
			return
		}
	}
}

func closeDB() {
	// closing statsDB properly
	if stats, err := varroa.NewDatabase(filepath.Join(varroa.StatsDir, varroa.DefaultHistoryDB)); err == nil {
		if closingErr := stats.Close(); closingErr != nil {
			logThis.Error(closingErr, varroa.NORMAL)
		}
	}
}
