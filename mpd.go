package varroa

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fhs/gompd/mpd"
	"github.com/pkg/errors"
)

const varroaMPDSubdir = "VARROA"

type MPD struct {
	client *mpd.Client
	root   string
}

// IsRunning checks for an active MPD daemon
func (m *MPD) IsRunning() bool {
	if m.client == nil {
		return false
	}
	return m.client.Ping() == nil
}

// Connect to MPD server
func (m *MPD) Connect(c *ConfigMPD) error {
	var client *mpd.Client
	var err error
	if c.Password == "" {
		client, err = mpd.Dial("tcp", c.Server)
	} else {
		client, err = mpd.DialAuthenticated("tcp", c.Server, c.Password)
	}
	if err != nil {
		return err
	}
	m.client = client
	m.root = c.Library
	return nil
}

// Close the connection to the server
func (m *MPD) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// Enable a path outside of MPD library
// This adds the symlink inside the MPD library to the specified path
func (m *MPD) Enable(path string) error {
	// http://mpd.wikia.com/wiki/Using_Multiple_Directories_Under_Parent
	// make symbolic link for path inside mpd.library

	// make sure path is absolute
	path, err := filepath.Abs(path)
	if err != nil {
		return errors.Wrap(err, "error getting absolute path for download directory")
	}
	if !DirectoryExists(path) {
		return errors.New("download directory cannot be found and symlinked into MPD library")
	}
	symlinkName := filepath.Join(m.root, varroaMPDSubdir)
	if !DirectoryExists(symlinkName) {
		return os.Symlink(path, symlinkName)
	}

	// if it exists, check it points to what we want
	target, err := filepath.EvalSymlinks(symlinkName)
	if err != nil {
		return errors.New("error resolving symbolic link target")
	}
	if path != target {
		return errors.New(fmt.Sprintf("the MPD library symlink %s already exists, and points to an unexpected directory", varroaMPDSubdir))
	}
	// symlink exists and points to the right directory
	return nil
}

// Disable the varroa MPD subdir
// This removes the symlink inside the MPD library
func (m *MPD) Disable(path string) error {
	// remove symbolic link for path inside mpd.library
	symlinkName := filepath.Join(m.root, varroaMPDSubdir)
	if DirectoryExists(symlinkName) {
		// check it points to what we want
		target, err := filepath.EvalSymlinks(symlinkName)
		if err != nil {
			return errors.New("error resolving symbolic link target")
		}
		source, err := filepath.Abs(path)
		if err != nil {
			return errors.Wrap(err, "error getting absolute path for download directory")
		}

		if source == target {
			return os.Remove(symlinkName)
		} else {
			return errors.New("the MPD library symlink points to an unexpected directory")
		}
	}
	return nil
}

// Update the MPD database for the varroa MPD subdir
func (m *MPD) Update() error {
	// path == name of symbolic link
	_, err := m.client.Update(varroaMPDSubdir)
	if err != nil {
		return err
	}

	w, err := mpd.NewWatcher("tcp", ":6600", "", "update")
	if err != nil {
		return err
	}
	defer w.Close()

	// wait until next update event
	done := make(chan struct{})
	go func() {
		for range w.Event {
			done <- struct{}{}
		}
	}()
	// waiting until update is done
	<-done
	return nil
}

// Add path (relative to mpd library root) of thing to play
func (m *MPD) Add(path string) error {
	return m.client.Add(path)
}

// Clear playlist
func (m *MPD) Clear() error {
	return m.client.Clear()
}

// Play at current position
func (m *MPD) Play() error {
	return m.client.Play(-1)
}

// Stop playing music
func (m *MPD) Stop() error {
	return m.client.Stop()
}

func (m *MPD) SendAndPlay(dlFolder, release string) error {
	// enable
	if err := m.Enable(filepath.Join(dlFolder, release)); err != nil {
		return err
	}
	// update
	if err := m.Update(); err != nil {
		return errors.Wrap(err, "error updating MPD database")
	}
	// add
	if err := m.Add(varroaMPDSubdir); err != nil {
		return errors.Wrap(err, "error adding release to MPD playlist")
	}
	// play
	return m.Play()
}

func (m *MPD) DisableAndDisconnect(dlFolder, release string) error {
	// disable
	if err := m.Disable(filepath.Join(dlFolder, release)); err != nil {
		return err
	}
	// update
	if err := m.Update(); err != nil {
		return err
	}
	// play
	return m.Close()
}
