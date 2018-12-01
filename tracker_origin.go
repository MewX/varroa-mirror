package varroa

import (
	"encoding/json"
	"io/ioutil"

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
}

func (toc *TrackerOriginJSON) Load() error {
	if toc.Path == "" {
		return errors.New("No path defined")
	}
	if !FileExists(toc.Path) {
		return errors.New("Path does not exist: " + toc.Path)
	}
	b, err := ioutil.ReadFile(toc.Path)
	if err != nil {
		return errors.Wrap(err, "Error loading JSON file "+toc.Path)
	}
	return toc.loadFromBytes(b)
}

func (toc *TrackerOriginJSON) loadFromBytes(data []byte) error {
	err := json.Unmarshal(data, &toc)
	if err != nil || len(toc.Origins) == 0 {
		// if it fails, try loading as the old format
		old := &OriginJSON{}
		if err = json.Unmarshal(data, &old); err != nil || old.Tracker == "" {
			return errors.New("Cannot parse " + OriginJSONFile + " in " + toc.Path)
		}
		// copy into new format
		if toc.Origins == nil {
			toc.Origins = make(map[string]*OriginJSON)
		}
		toc.Origins[old.Tracker] = old
		return nil
	}
	return err
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
