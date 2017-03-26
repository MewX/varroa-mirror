package main

import (
	docopt "github.com/docopt/docopt-go"
)

const (
	varroaVersion = "varroa musica -- v8."
	varroaUsage   = `
varroa musica.

Usage:
	varroa (start|reload|stop)
	varroa stats
	varroa --version

Options:
 	-h, --help             Show this screen.
  	--version              Show version.
`
)

type VarroaArguments struct {
	builtin bool
	start   bool
	stop    bool
	reload  bool
	stats   bool
}

func (b *VarroaArguments) parseCLI(osArgs []string) error {
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
	return nil
}
