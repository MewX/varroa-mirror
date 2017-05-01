package main

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"
)

type TrackerOriginJSON struct {
	Path                string `json:"-"`
	Tracker             string `json:"tracker"`
	ID                  int    `json:"id"`
	TimeSnatched        int64  `json:"time_snatched"`
	LastUpdatedMetadata int64  `json:"last_updated"`
	IsAlive             bool   `json:"is_alive"`
	TimeOfDeath         int64  `json:"time_of_death"`
}

func (toc *TrackerOriginJSON) Save(path string, tracker *GazelleTracker, info TrackerTorrentInfo) error {
	toc.Path = path
	if FileExists(toc.Path) {
		if err := toc.load(); err != nil {
			return err
		}
		if toc.ID == info.id && toc.Tracker == tracker.URL {
			toc.LastUpdatedMetadata = time.Now().Unix()
		} else {
			return errors.New(errorInfoNoMatchForOrigin)
		}
		// TODO if GetTorrentInfo errors out: origin.IsAlive = false and set TimeOfDeath
	} else {
		// creating origin
		toc.Tracker = tracker.URL
		toc.ID = info.id
		toc.TimeSnatched = time.Now().Unix()
		toc.LastUpdatedMetadata = time.Now().Unix()
		toc.IsAlive = true
	}
	return toc.write()
}

func (toc *TrackerOriginJSON) load() error {
	if toc.Path == "" {
		return errors.New("No path defined")
	}
	if !FileExists(toc.Path) {
		return errors.New("Path does not exist: " + toc.Path)
	}
	b, err := ioutil.ReadFile(toc.Path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &toc)
}

func (toc *TrackerOriginJSON) write() error {
	if toc.Path == "" {
		return errors.New("No path defined")
	}
	b, err := json.MarshalIndent(toc, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(toc.Path, b, 0644)
}
