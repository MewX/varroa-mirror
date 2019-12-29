package varroa

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sevlyar/go-daemon"
	"gitlab.com/catastrophic/assistance/logthis"
)

type boolFlag bool

func (b boolFlag) IsSet() bool {
	return bool(b)
}

// Daemon encapsulates daemon.Context for providing a simple API for varroa
type Daemon struct {
	context *daemon.Context
}

// NewDaemon create a new daemon.
func NewDaemon() *Daemon {
	return &Daemon{
		context: &daemon.Context{
			PidFileName: pidFile,
			PidFilePerm: 0644,
			LogFileName: "log",
			LogFilePerm: 0640,
			WorkDir:     "./",
			Umask:       0002,
		},
	}
}

// Start the daemon and return true if in child process.
func (d *Daemon) Start(args []string) error {
	d.context.Args = args
	child, err := d.context.Reborn()
	if err != nil {
		return err
	}
	if child != nil {
		logthis.Info("Starting daemon...", logthis.NORMAL)
	} else {
		logthis.Info("+ varroa musica daemon started ("+Version+")", logthis.NORMAL)
		// now in the daemon
		daemon.AddCommand(boolFlag(false), syscall.SIGTERM, quitDaemon)
	}
	return nil
}

// IsRunning return true if it is.
func (d *Daemon) IsRunning() bool {
	return daemon.WasReborn()
}

// Find the process, if it is running.
func (d *Daemon) Find() (*os.Process, error) {
	// trying to talk to existing daemon
	return d.context.Search()
}

// WaitForStop and clean exit
func (d *Daemon) WaitForStop() {
	if err := daemon.ServeSignals(); err != nil {
		logthis.Error(errors.Wrap(err, errorServingSignals), logthis.NORMAL)
	}
	logthis.Info("+ varroa musica stopped", logthis.NORMAL)
}

// Stop Daemon if running
func (d *Daemon) Stop(daemonProcess *os.Process) {
	daemon.AddCommand(boolFlag(true), syscall.SIGTERM, quitDaemon)
	if err := daemon.SendCommands(daemonProcess); err != nil {
		logthis.Error(errors.Wrap(err, errorSendingSignal), logthis.NORMAL)
	}
	if err := d.context.Release(); err != nil {
		logthis.Error(errors.Wrap(err, errorReleasingDaemon), logthis.NORMAL)
	}
	if err := os.Remove(d.context.PidFileName); err != nil {
		logthis.Error(errors.Wrap(err, errorRemovingPID), logthis.NORMAL)
	}
}

func quitDaemon(_ os.Signal) error {
	logthis.Info("+ terminating", logthis.VERBOSE)
	return daemon.ErrStop
}

// RunOrGo depending on whether we're in the daemon or not.
func RunOrGo(f func() error) error {
	if daemon.WasReborn() {
		go f()
		return nil
	}
	return f()
}
