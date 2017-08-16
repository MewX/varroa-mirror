package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"gitlab.com/passelecasque/varroa"
)

const (
	defaultVarroaFuseDBPath = "varroa-fuse.db"
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

	fmt.Println(varroa.Green("Mounting FUSE filesystem in " + cli.mountPoint))
	fmt.Println(varroa.Green("To quit cleanly, run 'fusermount -u " + cli.mountPoint + "'"))
	if err := varroa.FuseMount(cli.targetDirectory, cli.mountPoint, defaultVarroaFuseDBPath); err != nil {
		logThis.Error(err, varroa.NORMAL)
		return
	}
	fmt.Println(varroa.Green("Unmounting FUSE filesystem, fusermount -u has presumably been called."))
	return
}
