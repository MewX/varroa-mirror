package main

import (
	"errors"

	docopt "github.com/docopt/docopt-go"
)

const (
	varroaVersion = "varroa musica -- v9."
	varroaUsage   = `
varroa musica.

Usage:
	varroa (start|reload|stop)
	varroa stats
	varroa refresh-metadata <ID>...
	varroa check-log <LOG_FILE>
	varroa snatch <ID>...
	varroa backup
	varroa show-filters
	varroa --version

Options:
 	-h, --help             Show this screen.
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
	backup          bool
	showFilters	bool
	torrentIDs      []int
	logFile         string
	requiresDaemon  bool
	canUseDaemon    bool
}

func (b *varroaArguments) parseCLI(osArgs []string) error {
	// parse arguments and options
	args, err := docopt.Parse(varroaUsage, osArgs, true, varroaVersion, false, false)
	if err != nil {
		return err
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
	b.showFilters = args["show-filters"].(bool)
	// arguments
	if b.refreshMetadata || b.snatch {
		IDs, ok := args["<ID>"].([]string)
		if !ok {
			return errors.New("Invalid torrent IDs.")
		}
		b.torrentIDs, err = StringSliceToIntSlice(IDs)
		if err != nil {
			return errors.New("Invalid torrent IDs, must be integers.")
		}
	}
	if b.checkLog {
		logPath := args["<LOG_FILE>"].(string)
		if !FileExists(logPath) {
			return errors.New("Invalid log file, does not exist.")
		}
		b.logFile = logPath
	}

	// sorting which commands can use the daemon if it's there but should manage if it is not
	b.requiresDaemon = true
	b.canUseDaemon = true
	if b.refreshMetadata || b.snatch || b.checkLog || b.backup || b.stats {
		b.requiresDaemon = false
	}
	// sorting which commands should not interact with the daemon in any case
	if b.backup || b.showFilters {
		b.canUseDaemon = false
	}
	return nil
}

func (b *varroaArguments) commandToDaemon() string {
	if b.stats {
		return "stats"
	}
	if b.reload {
		return "reload"
	}
	if b.stop {
		// to cleanly close the unix socket
		return "stop"
	}
	if b.refreshMetadata {
		return "refresh-metadata " + IntSliceToString(b.torrentIDs)
	}
	if b.snatch {
		return "snatch " + IntSliceToString(b.torrentIDs)
	}
	if b.checkLog {
		return "check-log " + b.logFile
	}
	return ""
}
