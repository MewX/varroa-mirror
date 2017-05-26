package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
	"gopkg.in/vmihailenco/msgpack.v2"
)

type Downloads struct {
	Root      string
	DBFile    string
	MaxIndex  uint64
	Downloads []*DownloadFolder
}

func (d *Downloads) String() string {
	txt := "Downloads in database:\n"
	for _, dl := range d.Downloads {
		txt += "\t" + dl.ShortString() + "\n"
	}
	return txt
}

func (d *Downloads) Load(path string) error {
	d.DBFile = path
	// load db
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		logThis.Error(errors.Wrap(err, "Error reading history file"), NORMAL)
		return err
	}
	if len(bytes) == 0 {
		// newly created file
		return nil
	}
	// load releases from history to in-memory slice
	err = msgpack.Unmarshal(bytes, &d.Downloads)
	if err != nil {
		logThis.Error(errors.Wrap(err, "Error loading releases from history file"), NORMAL)
	}
	// get max index
	for _, dl := range d.Downloads {
		if dl.Index > d.MaxIndex {
			d.MaxIndex = dl.Index
		}
	}
	return err
}

func (d *Downloads) Save() error {
	// saving to msgpack
	b, err := msgpack.Marshal(d.Downloads)
	if err != nil {
		return err
	}
	// write to history file
	return ioutil.WriteFile(d.DBFile, b, 0640)
}

func (d *Downloads) Scan() error {
	// list of loaded folders
	knownDownloads := []string{}
	for _, dl := range d.Downloads {
		knownDownloads = append(knownDownloads, dl.Path)
	}

	// don't walk, we only want the top-level directories here
	entries, err := ioutil.ReadDir(d.Root)
	if err != nil {
		log.Fatal(err)
	}

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = "Scanning"
	s.Start()
	for _, entry := range entries {
		if entry.IsDir() {
			dl, err := d.FindByFolder(entry.Name())
			if err != nil {
				// new entry
				// logThis.Info("Found new download: "+entry.Name(), VERBOSEST)
				if err := d.Add(entry.Name()); err != nil {
					logThis.Error(err, NORMAL)
					continue
				}
			} else {
				// logThis.Info("Updating known download: "+dl.Path, VERBOSEST)
				// TODO might be time-consuming to reload everything...
				if err := dl.Load(); err != nil {
					logThis.Error(err, NORMAL)
					continue
				}
			}
			knownDownloads = RemoveFromSlice(entry.Name(), knownDownloads)
		}
	}
	s.Stop()

	// remove from db folders that are no longer in the filesystem
	if len(knownDownloads) != 0 {
		for _, dl := range knownDownloads {
			// logThis.Info("Removing from download db: "+dl, VERBOSEST)
			if err := d.RemoveByFolder(dl); err != nil {
				logThis.Error(err, NORMAL)
			}
		}
	}
	return nil
}

func (d *Downloads) LoadAndScan(path string) error {
	if err := d.Load(path); err != nil {
		return errors.New("Error loading downloads database")
	}
	if err := d.Scan(); err != nil {
		return errors.New("Error scanning downloads")
	}
	return nil
}

func (d *Downloads) Add(path string) error {
	dl := &DownloadFolder{Index: d.MaxIndex + 1, Path: path, Root: d.Root, State: stateUnsorted}
	if err := dl.Load(); err != nil {
		logThis.Error(err, NORMAL)
		return err
	}
	d.Downloads = append(d.Downloads, dl)
	d.MaxIndex += 1
	return nil
}

func (d *Downloads) FindByID(id uint64) (*DownloadFolder, error) {
	for _, dl := range d.Downloads {
		if dl.Index == id {
			return dl, nil
		}
	}
	return nil, errors.New("ID not found")
}

func (d *Downloads) FindByFolder(folder string) (*DownloadFolder, error) {
	for _, dl := range d.Downloads {
		if dl.Path == folder {
			return dl, nil
		}
	}
	return nil, errors.New("folder not found")
}

func (d *Downloads) RemoveByFolder(folder string) error {
	for i, v := range d.Downloads {
		if v.Path == folder {
			d.Downloads = append(d.Downloads[:i], d.Downloads[i+1:]...)
			return nil
		}
	}
	return errors.New("Folder not found in DB.")
}

func (d *Downloads) FindByArtist(artist string) []*DownloadFolder {
	hits := []*DownloadFolder{}
	for _, dl := range d.Downloads {
		if dl.HasInfo {
			for _, info := range dl.Metadata {
				if StringInSlice(artist, info.ArtistNames()) {
					hits = append(hits, dl)
				}
			}
		}
	}
	return hits
}

func (d *Downloads) FindByInfoHash(infoHash string) error {
	// TODO ?

	return nil
}

func (d *Downloads) FindByTrackerID(tracker, id string) error {
	// TODO ?

	return nil
}

func (d *Downloads) Sort(libraryPath, folderTemplate string, useHardLinks bool) error {
	for _, dl := range d.Downloads {
		if dl.State == stateUnsorted {
			if !Accept(fmt.Sprintf("Sorting download #%d (%s), continue ", dl.Index, dl.Path)) {
				return nil
			}
			if err := dl.Sort(libraryPath, folderTemplate, useHardLinks); err != nil {
				return errors.Wrap(err, "Error sorting download "+strconv.FormatUint(dl.Index, 10))
			}
		}
	}
	return nil
}
