package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
	"github.com/sevlyar/go-daemon"
	"gopkg.in/vmihailenco/msgpack.v2"
)

type Downloads struct {
	Root     string
	DBFile   string
	MaxIndex uint64
	Releases DownloadFolders
}

func (d *Downloads) String() string {
	txt := "Downloads in database:\n"
	for _, dl := range d.Releases {
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
	err = msgpack.Unmarshal(bytes, &d.Releases)
	if err != nil {
		logThis.Error(errors.Wrap(err, "Error loading releases from history file"), NORMAL)
	}
	// get max index
	for _, dl := range d.Releases {
		if dl.Index > d.MaxIndex {
			d.MaxIndex = dl.Index
		}
	}
	return err
}

func (d *Downloads) Save() error {
	// saving to msgpack, won't save TrackerTorrentInfo though...
	b, err := msgpack.Marshal(d.Releases)
	if err != nil {
		return err
	}
	// write to history file
	return ioutil.WriteFile(d.DBFile, b, 0640)
}

func (d *Downloads) Scan() error {
	// list of loaded folders
	knownDownloads := []string{}
	for _, dl := range d.Releases {
		knownDownloads = append(knownDownloads, dl.Path)
	}

	// don't walk, we only want the top-level directories here
	entries, err := ioutil.ReadDir(d.Root)
	if err != nil {
		log.Fatal(err)
	}

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = "Scanning"
	if !daemon.WasReborn() {
		s.Start()
	}
	for _, entry := range entries {
		if entry.IsDir() {
			dl, err := d.FindByFolderName(entry.Name())
			if err != nil {
				// new entry
				// logThis.Info("Found new download: "+entry.Name(), VERBOSEST)
				if err := d.Add(entry.Name()); err != nil {
					logThis.Error(err, VERBOSEST)
					continue
				}
			} else {
				// logThis.Info("Updating known download: "+dl.Path, VERBOSEST)
				if err := dl.Load(); err != nil {
					logThis.Error(err, VERBOSEST)
					continue
				}
			}
			knownDownloads = RemoveFromSlice(entry.Name(), knownDownloads)
		}
	}
	if !daemon.WasReborn() {
		s.Stop()
	}

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
		return errors.New(errorLoadingDownloadsDB)
	}
	if err := d.Scan(); err != nil {
		return errors.New("Error scanning downloads")
	}
	return nil
}

func (d *Downloads) Add(path string) error {
	dl := &DownloadFolder{Index: d.MaxIndex + 1, Path: path, Root: d.Root, State: stateUnsorted}
	if err := dl.Load(); err != nil {
		return err
	}
	d.Releases = append(d.Releases, dl)
	d.MaxIndex += 1
	return nil
}

func (d *Downloads) FindByID(id uint64) (*DownloadFolder, error) {
	return d.Releases.FindByID(id)
}

func (d *Downloads) FindByFolderName(folder string) (*DownloadFolder, error) {
	return d.Releases.FindByPath(folder)
}

func (d *Downloads) RemoveByFolder(folder string) error {
	for i, v := range d.Releases {
		if v.Path == folder {
			d.Releases = append(d.Releases[:i], d.Releases[i+1:]...)
			return nil
		}
	}
	return errors.New("Folder not found in DB.")
}

func (d Downloads) FilterByArtist(artist string) DownloadFolders {
	return d.Releases.FilterArtist(artist)
}

func (d Downloads) FilterByTag(tag string) DownloadFolders {
	return d.Releases.FilterTag(tag)
}

func (d Downloads) FilterByState(state string) DownloadFolders {
	if !StringInSlice(state, downloadFolderStates) {
		logThis.Info("Invalid state", NORMAL)
	}
	dlState := DownloadState(-1).Get(state)
	return d.Releases.FilterSortedState(dlState)
}

func (d *Downloads) AllArtists() []string {
	return d.Releases.AllArtists()
}

func (d *Downloads) AllTags() []string {
	return d.Releases.AllTags()
}

func (d *Downloads) AllLabels() []string {
	return d.Releases.AllRecordLabels()
}

func (d *Downloads) FindByInfoHash(infoHash string) error {
	// TODO ?

	return nil
}

func (d *Downloads) FindByTrackerID(tracker, id string) error {
	// TODO ?

	return nil
}

func (d *Downloads) Sort(e *Environment) error {
	for _, dl := range d.Releases {
		if dl.State == stateUnsorted {
			if !Accept(fmt.Sprintf("Sorting download #%d (%s), continue ", dl.Index, dl.Path)) {
				return nil
			}
			if err := dl.Sort(e); err != nil {
				return errors.Wrap(err, "Error sorting download "+strconv.FormatUint(dl.Index, 10))
			}
		} else if dl.State == stateAccepted {
			if Accept(fmt.Sprintf("Do you want to export already accepted release #%d (%s) ", dl.Index, dl.Path)) {
				if err := dl.export(e.config); err != nil {
					return errors.Wrap(err, "Error exporting download "+strconv.FormatUint(dl.Index, 10))
				}
			} else {
				fmt.Println("The release was not exported. It can be exported later by sorting again.")
			}
		}
	}
	return nil
}

func (d *Downloads) Clean() error {
	// prepare directory for cleaned folders if necessary
	cleanDir := filepath.Join(d.Root, downloadsCleanDir)
	if !DirectoryExists(cleanDir) {
		if err := os.MkdirAll(cleanDir, 0777); err != nil {
			return errors.Wrap(err, errorCreatingDownloadsCleanDir)
		}
	}

	// don't walk, we only want the top-level directories here
	toBeMoved := []os.FileInfo{}

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = "Scanning"
	if !daemon.WasReborn() {
		s.Start()
	}

	// don't walk, we only want the top-level directories here
	entries, err := ioutil.ReadDir(d.Root)
	if err != nil {
		log.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != downloadsCleanDir && entry.IsDir() {
			// read at most 2 entries insinde entry
			f, err := os.Open(filepath.Join(d.Root, entry.Name()))
			if err != nil {
				logThis.Error(errors.Wrap(err, "Error opening "+entry.Name()), VERBOSE)
				continue
			}
			contents, err := f.Readdir(2)
			f.Close()

			if err != nil {
				if err == io.EOF {
					toBeMoved = append(toBeMoved, entry)
				} else {
					logThis.Error(errors.Wrap(err, "Error listing contents of "+entry.Name()), VERBOSE)
				}
			} else if len(contents) == 1 && contents[0].IsDir() && contents[0].Name() == metadataDir {
				toBeMoved = append(toBeMoved, entry)
			}
		}
	}
	if !daemon.WasReborn() {
		s.Stop()
	}

	// clean
	for _, r := range toBeMoved {
		if err := os.Rename(filepath.Join(d.Root, r.Name()), filepath.Join(cleanDir, r.Name())); err != nil {
			return errors.Wrap(err, errorCleaningDownloads+r.Name())
		}
	}
	return nil
}
