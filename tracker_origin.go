package main

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"
)

// TrackerOriginJSON contains the list of trackers of origin for a release.
type TrackerOriginJSON struct {
	Path    string                 `json:"-"`
	Origins map[string]*OriginJSON `json:"known_origins"`
}

type OriginJSON struct {
	Tracker             string `json:"tracker"`
	ID                  int    `json:"id"`
	GroupID             int    `json:"group_id"`
	TimeSnatched        int64  `json:"time_snatched"`
	LastUpdatedMetadata int64  `json:"last_updated"`
	IsAlive             bool   `json:"is_alive"`
	TimeOfDeath         int64  `json:"time_of_death"`
}

func (toc *TrackerOriginJSON) Save(path string, tracker *GazelleTracker, info TrackerTorrentInfo) error {
	toc.Path = path
	foundOrigin := false
	if FileExists(toc.Path) {
		if err := toc.load(); err != nil {
			return err
		}
		for i, o := range toc.Origins {
			if i == tracker.Name && o.ID == info.id {
				toc.Origins[i].LastUpdatedMetadata = time.Now().Unix()
				// may have been edited
				toc.Origins[i].GroupID = info.groupID
				foundOrigin = true
			}
			// TODO if GetTorrentInfo errors out: origin.IsAlive = false and set TimeOfDeath
		}
	}
	if !foundOrigin {
		if toc.Origins == nil {
			toc.Origins = make(map[string]*OriginJSON)
		}
		// creating origin
		toc.Origins[tracker.Name] = &OriginJSON{Tracker: tracker.URL, ID: info.id, GroupID: info.groupID, TimeSnatched: time.Now().Unix(), LastUpdatedMetadata: time.Now().Unix(), IsAlive: true}
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
	if err := json.Unmarshal(b, &toc); err != nil || len(toc.Origins) == 0 {
		// if it fails, try loading as the old format
		old := &OriginJSON{}
		if err := json.Unmarshal(b, &old); err != nil || old.Tracker == "" {
			return errors.New("Cannot parse " + originJSONFile + " in " + toc.Path)
		}
		// copy into new format
		if toc.Origins == nil {
			toc.Origins = make(map[string]*OriginJSON)
		}
		toc.Origins[old.Tracker] = old
		return nil
	}
	return nil
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
