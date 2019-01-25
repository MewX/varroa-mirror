package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gitlab.com/catastrophic/assistance/logthis"
	"gitlab.com/catastrophic/assistance/ui"
	"gitlab.com/passelecasque/varroa"
)

const (
	defaultVarroaFuseDBPath = "varroa-fuse-%s.db"
)

func main() {
	// parsing CLI
	cli := &varroaArguments{}
	if err := cli.parseCLI(os.Args[1:]); err != nil {
		logthis.Error(errors.Wrap(err, varroa.ErrorArguments), logthis.NORMAL)
		return
	}
	if cli.builtin {
		return
	}

	fmt.Println(ui.Green("Mounting FUSE filesystem in " + cli.mountPoint))
	fmt.Println(ui.Green("To quit cleanly, run 'fusermount -u " + cli.mountPoint + "'"))
	if err := varroa.FuseMount(cli.targetDirectory, cli.mountPoint, fmt.Sprintf(defaultVarroaFuseDBPath, filepath.Base(cli.targetDirectory))); err != nil {
		logthis.Error(err, logthis.NORMAL)
		return
	}
	fmt.Println(ui.Green("Unmounting FUSE filesystem, fusermount -u has presumably been called."))
}
