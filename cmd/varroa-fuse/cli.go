package main

import (
	"fmt"
	"os/exec"

	docopt "github.com/docopt/docopt-go"
	"github.com/pkg/errors"
	"gitlab.com/passelecasque/varroa"
)

const (
	varroaFuseUsage = `
	_  _ ____ ____ ____ ____ ____    _  _ _  _ ____ _ ____ ____
	|  | |__| |__/ |__/ |  | |__|    |\/| |  | [__  | |    |__|
	 \/  |  | |  \ |  \ |__| |  |    |  | |__| ___] | |___ |  |


Description:

	varroa musica fuse is a stand-alone application that creates
	and mounts a read-only FUSE filesystem based on tracker metadata
	(JSONs saved by varroa musica).
	It does not require a configuration file.

	To unmount, run 'fusermount -u <MOUNT_POINT>'.

Usage:
	varroa-fuse <MUSIC_DIRECTORY> <MOUNT_POINT>
	varroa-fuse --version
	varroa-fuse --help

Options:
 	-h, --help             Show this screen.
  	--version              Show version.
`
	varroaFuseFullName = "varroa-fuse"
)

type varroaArguments struct {
	builtin         bool
	targetDirectory string
	mountPoint      string
}

func (b *varroaArguments) parseCLI(osArgs []string) error {
	// parse arguments and options
	args, err := docopt.Parse(varroaFuseUsage, osArgs, true, fmt.Sprintf(varroa.FullVersion, varroaFuseFullName, varroa.Version), false, false)
	if err != nil {
		return errors.Wrap(err, varroa.ErrorInfoBadArguments)
	}
	if len(args) == 0 {
		// builtin command, nothing to do.
		b.builtin = true
		return nil
	}

	// checking fusermount is available
	_, err = exec.LookPath("fusermount")
	if err != nil {
		return errors.New("fusermount is not available on this system, cannot use the fuse command")
	}

	b.targetDirectory = args["<MUSIC_DIRECTORY>"].(string)
	if !varroa.DirectoryExists(b.targetDirectory) {
		return errors.New("Target directory does not exist")
	}

	b.mountPoint = args["<MOUNT_POINT>"].(string)
	if !varroa.DirectoryExists(b.mountPoint) {
		return errors.New("Fuse mount point does not exist")
	}

	// check it's empty
	if isEmpty, err := varroa.DirectoryIsEmpty(b.mountPoint); err != nil {
		return errors.New("Could not open Fuse mount point")
	} else if !isEmpty {
		return errors.New("Fuse mount point is not empty")
	}

	return nil
}
