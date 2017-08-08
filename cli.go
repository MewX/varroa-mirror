package main

import (
	"encoding/json"
	"fmt"
	"strings"

	docopt "github.com/docopt/docopt-go"
	"github.com/pkg/errors"
)

const (
	varroaUsage = `
	_  _ ____ ____ ____ ____ ____    _  _ _  _ ____ _ ____ ____
	|  | |__| |__/ |__/ |  | |__|    |\/| |  | [__  | |    |__|
	 \/  |  | |  \ |  \ |__| |  |    |  | |__| ___] | |___ |  |


Description:

	varroa musica is a personal assistant for your favorite tracker.

	It can:
	- snatch, and autosnatch torrents with quite thorough filters
	- monitor your stats and generate graphs
	- host said graphs on its embedded webserver or on Gitlab Pages
	- save and update all snatched torrents metadata
	- be remotely controlled from your browser with a GreaseMonkey script.
	- send notifications to your Android device about stats and snatches.
	- check local logs agains logchecker.php

Daemon Commands:

	The daemon is used for autosnatching, stats monitoring and hosting,
	and remotely triggering snatches from the GM script or any
	pyWhatAuto remote (including the Android App).

	start:
		starts the daemon.
	stop:
		stops it.
	reload:
		reloads the configuration file (allows updating filters without
		restarting the daemon).

Commands:

	stats:
		generates the stats immediately based on currently saved
		history.
	refresh-metadata:
		retrieves all metadata for all torrents with IDs given as
		arguments, updating the files that were downloaded when they
		were first snatched (allows updating local metadata if a
		torrent has been edited since upload).
	check-log:
		upload a given log file to the tracker's logchecker.php and
		returns its score.
	snatch:
		snatch all torrents with IDs given as arguments.
	info:
		output info about the torrent IDs given as argument.
	backup:
		backup user files (stats, history, configuration file) to a
		timestamped zip file. Automatically triggered every day.
	downloads scan:
		scan the downloads folder and refreshes the database of known
		downloads.
	downloads search:
		return all known downloads on which an artist has worked.
	downloads metadata:
		return information about a specific download. Takes downloads
		db ID as argument.
	downloads sort:
		sort all unsorted downloads, or sort a specific release
		(identified by its db ID). sorting allows you to tag which
		release to keep and which to only seed; selected downloads
		can be exported to an external folder.
	downloads list:
		list downloads by state: unsorted, accepted, exported, rejected.
	downloads clean:
		clean up the downloads directory by moving all empty folders,
		and folders with only tracker metadata, to a dedicated subfolder.

Configuration Commands:

	show-config:
		displays what varroa has parsed from the configuration file
		(useful for checking the YAML is correctly formatted, and the
		filters are correctly interpreted).
	encrypt:
		encrypts your configuration file. The encrypted version can
		be used in place of the plaintext version, if you're
		uncomfortable having passwords lying around in an simple text
		file. You will be prompted for a passphrase which you will
		have to enter again every time you run varroa. Your passwords
		will still be decoded in memory while varroa is up. This
		command does not remove the plaintext version.
	decrypt:
		decrypts your encrypted configuration file.

Usage:
	varroa (start|reload|stop)
	varroa stats
	varroa refresh-metadata <TRACKER> <ID>...
	varroa check-log <TRACKER> <LOG_FILE>
	varroa snatch [--fl] <TRACKER> <ID>...
	varroa info <TRACKER> <ID>...
	varroa backup
	varroa show-config
	varroa downloads (scan|search <ARTIST>|metadata <ID>|sort [<ID>]|list <STATE>|clean)
	varroa (encrypt|decrypt)
	varroa --version

Options:
 	-h, --help             Show this screen.
 	--fl                   Use personal Freeleech torrent if available.
  	--version              Show version.
`
)

type varroaArguments struct {
	builtin         bool
	start           bool
	stop            bool
	reload          bool
	stats           bool
	refreshMetadata bool
	checkLog        bool
	snatch          bool
	info            bool
	backup          bool
	showConfig      bool
	encrypt         bool
	decrypt         bool
	downloadScan    bool
	downloadSearch  bool
	downloadInfo    bool
	downloadSort    bool
	downloadList    bool
	downloadState   string
	downloadClean   bool
	useFLToken      bool
	torrentIDs      []int
	logFile         string
	trackerLabel    string
	artistName      string
	requiresDaemon  bool
	canUseDaemon    bool
}

func (b *varroaArguments) parseCLI(osArgs []string) error {
	// parse arguments and options
	args, err := docopt.Parse(varroaUsage, osArgs, true, fmt.Sprintf(varroaVersion, varroa, version), false, false)
	if err != nil {
		return errors.Wrap(err, errorInfoBadArguments)
	}
	if len(args) == 0 {
		// builtin command, nothing to do.
		b.builtin = true
		return nil
	}

	// commands
	b.start = args["start"].(bool)
	b.stop = args["stop"].(bool)
	b.reload = args["reload"].(bool)
	b.stats = args["stats"].(bool)
	b.refreshMetadata = args["refresh-metadata"].(bool)
	b.checkLog = args["check-log"].(bool)
	b.snatch = args["snatch"].(bool)
	b.backup = args["backup"].(bool)
	b.info = args["info"].(bool)
	b.showConfig = args["show-config"].(bool)
	b.encrypt = args["encrypt"].(bool)
	b.decrypt = args["decrypt"].(bool)
	if args["downloads"].(bool) {
		b.downloadScan = args["scan"].(bool)
		b.downloadSearch = args["search"].(bool)
		if b.downloadSearch {
			b.artistName = args["<ARTIST>"].(string)
		}
		b.downloadInfo = args["metadata"].(bool)
		b.downloadSort = args["sort"].(bool)
		b.downloadList = args["list"].(bool)
		b.downloadClean = args["clean"].(bool)
	}
	// arguments
	if b.refreshMetadata || b.snatch || b.downloadInfo || b.downloadSort || b.info {
		IDs, ok := args["<ID>"].([]string)
		if !ok {
			return errors.New("Invalid torrent IDs.")
		}
		b.torrentIDs, err = StringSliceToIntSlice(IDs)
		if err != nil {
			return errors.New("Invalid torrent IDs, must be integers.")
		}
	}
	if b.downloadList {
		b.downloadState = args["<STATE>"].(string)
		if !StringInSlice(b.downloadState, downloadFolderStates) {
			return errors.New("Invalid download state, must be among: " + strings.Join(downloadFolderStates, ", "))
		}
	}
	if b.snatch {
		b.useFLToken = args["--fl"].(bool)
	}
	if b.checkLog {
		logPath := args["<LOG_FILE>"].(string)
		if !FileExists(logPath) {
			return errors.New("Invalid log file, does not exist.")
		}
		b.logFile = logPath
	}
	if b.refreshMetadata || b.snatch || b.checkLog || b.info {
		b.trackerLabel = args["<TRACKER>"].(string)
	}

	// sorting which commands can use the daemon if it's there but should manage if it is not
	b.requiresDaemon = true
	b.canUseDaemon = true
	if b.refreshMetadata || b.snatch || b.checkLog || b.backup || b.stats || b.downloadScan || b.downloadSearch || b.downloadInfo || b.downloadSort || b.downloadList || b.info || b.downloadClean {
		b.requiresDaemon = false
	}
	// sorting which commands should not interact with the daemon in any case
	if b.backup || b.showConfig || b.decrypt || b.encrypt || b.downloadScan || b.downloadSearch || b.downloadInfo || b.downloadSort || b.downloadList || b.downloadClean {
		b.canUseDaemon = false
	}
	return nil
}

func (b *varroaArguments) commandToDaemon() []byte {
	out := IncomingJSON{Site: b.trackerLabel}
	if b.stats {
		out.Command = "stats"
	}
	if b.reload {
		out.Command = "reload"
	}
	if b.stop {
		// to cleanly close the unix socket
		out.Command = "stop"
	}
	if b.refreshMetadata {
		out.Command = "refresh-metadata"
		out.Args = IntSliceToStringSlice(b.torrentIDs)
	}
	if b.snatch {
		out.Command = "snatch"
		out.Args = IntSliceToStringSlice(b.torrentIDs)
		out.FLToken = b.useFLToken
	}
	if b.info {
		out.Command = "info"
		out.Args = IntSliceToStringSlice(b.torrentIDs)
	}
	if b.checkLog {
		out.Command = "check-log"
		out.Args = []string{b.logFile}
	}
	commandBytes, err := json.Marshal(out)
	if err != nil {
		logThis.Error(errors.Wrap(err, "Cannot parse command"), NORMAL)
		return []byte{}
	}
	return commandBytes
}
