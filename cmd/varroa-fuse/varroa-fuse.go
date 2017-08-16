package main

import (
	"os"

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

	logThis.Info("Mounting FUSE filesystem in "+cli.mountPoint, varroa.NORMAL)
	logThis.Info("To quit cleanly, run 'fusermount -u "+cli.mountPoint+"'", varroa.NORMAL)
	if err := varroa.FuseMount(cli.targetDirectory, cli.mountPoint); err != nil {
		logThis.Error(err, varroa.NORMAL)
		return
	}
	logThis.Info("Unmounting FUSE filesystem, fusermount -u has presumably been called.", varroa.VERBOSE)
	return
}
