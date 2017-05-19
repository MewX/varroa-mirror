package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/vmihailenco/msgpack.v2"
)

type Downloads struct {
	Root      string
	DBFile    string
	Downloads []*DownloadFolder
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
	// TODO refresh d.Downloads
	// don't walk, use listdir, we only want the top-level directories here
	entries, err := ioutil.ReadDir(d.Root)
	if err != nil {
		log.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			dl, err := d.FindByFolder(entry.Name())
			if err != nil {
				// new entry
				fmt.Println("NEW ENTRY: " + entry.Name())
				if err := d.Add(entry.Name()); err != nil {
					logThis.Error(err, NORMAL)
					continue // or return errors.Wrap?
				}
			} else {
				// TODO UPDATE / CHECK?
				fmt.Println("FOUND KNOWN: " + dl.Path)
			}
		}
	}
	// TODO DELETE ALL FOLDERS IN DB NOT FOUND HERE
	return nil
}

func (d *Downloads) Add(path string) error {
	fmt.Println("ADDING " + path)
	dl := &DownloadFolder{Path: path, Root: d.Root}
	if err := dl.Load(); err != nil {
		logThis.Error(err, NORMAL)
		return err
	}
	d.Downloads = append(d.Downloads, dl)
	return nil
}

func (d *Downloads) FindByID(id string) (*DownloadFolder, error) {
	tID, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	for _, dl := range d.Downloads {
		if dl.ID == tID {
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

func (d *Downloads) FindByInfoHash(infoHash string) error {
	// TODO ?

	return nil
}
