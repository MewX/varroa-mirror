package varroa

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sevlyar/go-daemon"
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
	d.context.Args = os.Args
	child, err := d.context.Reborn()
	if err != nil {
		return err
	}
	if child != nil {
		logThis.Info("Starting daemon...", NORMAL)
	} else {
		logThis.Info("+ varroa musica daemon started ("+Version+")", NORMAL)
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
		logThis.Error(errors.Wrap(err, errorServingSignals), NORMAL)
	}
	logThis.Info("+ varroa musica stopped", NORMAL)
}

// Stop Daemon if running
func (d *Daemon) Stop(daemonProcess *os.Process) {
	daemon.AddCommand(boolFlag(true), syscall.SIGTERM, quitDaemon)
	if err := daemon.SendCommands(daemonProcess); err != nil {
		logThis.Error(errors.Wrap(err, errorSendingSignal), NORMAL)
	}
	if err := d.context.Release(); err != nil {
		logThis.Error(errors.Wrap(err, errorReleasingDaemon), NORMAL)
	}
	if err := os.Remove(d.context.PidFileName); err != nil {
		logThis.Error(errors.Wrap(err, errorRemovingPID), NORMAL)
	}
}

func quitDaemon(sig os.Signal) error {
	logThis.Info("+ terminating", VERBOSE)
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
