package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/asdine/storm"
	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
)

type FuseEntry struct {
	ID          int      `storm:"id,increment"`
	FolderName  string   `storm:"unique"`
	Artists     []string `storm:"index"`
	Tags        []string `storm:"index"`
	Title       string   `storm:"index"`
	Year        int      `storm:"index"`
	Tracker     []string `storm:"index"`
	RecordLabel string   `storm:"index"`
}

func (fe *FuseEntry) reset() {
	fe.Artists = []string{}
	fe.Tags = []string{}
	fe.Title = ""
	fe.Year = 0
	fe.Tracker = []string{}
	fe.RecordLabel = ""
}

func (fe *FuseEntry) Load(root string) error {
	if fe.FolderName == "" || !DirectoryExists(filepath.Join(root, fe.FolderName)) {
		return errors.New("Wrong or missing path")
	}

	// find origin.json
	originFile := filepath.Join(root, fe.FolderName, metadataDir, originJSONFile)
	if FileExists(originFile) {
		origin := TrackerOriginJSON{Path: originFile}
		if err := origin.load(); err != nil {
			return errors.Wrap(err, "Error reading origin.json")
		} else {
			// reset fields
			fe.reset()

			// TODO: remove duplicate if there are actually several origins

			// load useful things from JSON
			for tracker := range origin.Origins {
				fe.Tracker = append(fe.Tracker, tracker)

				// getting release info from json
				infoJSON := filepath.Join(root, fe.FolderName, metadataDir, tracker+"_"+trackerMetadataFile)
				if !FileExists(infoJSON) {
					// if not present, try the old format
					infoJSON = filepath.Join(root, fe.FolderName, metadataDir, "Release.json")
				}
				if FileExists(infoJSON) {
					// load JSON, get info
					data, err := ioutil.ReadFile(infoJSON)
					if err != nil {
						return errors.Wrap(err, "Error loading JSON file "+infoJSON)
					}
					var gt GazelleTorrent
					if err := json.Unmarshal(data, &gt.Response); err != nil {
						return errors.Wrap(err, "Error parsing JSON file "+infoJSON)
					}
					// extract relevant information!
					// for now, using artists, composers, "with" categories
					for _, el := range gt.Response.Group.MusicInfo.Artists {
						fe.Artists = append(fe.Artists, el.Name)
					}
					for _, el := range gt.Response.Group.MusicInfo.With {
						fe.Artists = append(fe.Artists, el.Name)
					}
					for _, el := range gt.Response.Group.MusicInfo.Composers {
						fe.Artists = append(fe.Artists, el.Name)
					}
					// record label
					fe.RecordLabel = gt.Response.Group.RecordLabel
					if gt.Response.Torrent.Remastered {
						fe.RecordLabel = gt.Response.Torrent.RemasterRecordLabel
					}
					// year
					fe.Year = gt.Response.Group.Year
					if gt.Response.Torrent.Remastered {
						fe.Year = gt.Response.Torrent.RemasterYear
					}
					// title
					fe.Title = gt.Response.Group.Name
					// tags
					fe.Tags = gt.Response.Group.Tags
				}
			}
		}
	} else {
		return errors.New("Error, no metadata found")
	}
	return nil
}

type FuseDB struct {
	Path string
	DB   *storm.DB
	Root string
}

func (fdb *FuseDB) Open() error {
	// TODO check fdb.Path exists

	var err error
	fdb.DB, err = storm.Open(fdb.Path)
	return err
}

func (fdb *FuseDB) Close() error {
	if fdb.DB != nil {
		return fdb.DB.Close()
	}
	return nil
}

func (fdb *FuseDB) Scan(path string) error {
	if fdb.DB == nil {
		return errors.New("Error db not open")
	}
	if !DirectoryExists(path) {
		return errors.New("Error finding " + path)
	}
	fdb.Root = path

	// don't walk, we only want the top-level directories here
	entries, err := ioutil.ReadDir(fdb.Root)
	if err != nil {
		log.Fatal(err)
	}

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = "Scanning"
	s.Start()

	// TODO FIND THE ENTRIES NO LONGER IN THE FILESYSTEM

	for _, entry := range entries {
		if entry.IsDir() {
			// detect if sound files are present, leave otherwise
			if !DirectoryContainsMusic(filepath.Join(fdb.Root, entry.Name())) {
				logThis.Info("Error: no music found in "+entry.Name(), VERBOSEST)
				continue
			}
			// try to find entry
			var fuseEntry FuseEntry
			if err := fdb.DB.One("FolderName", entry.Name(), &fuseEntry); err != nil {
				if err == storm.ErrNotFound {
					// not found, create new entry
					fuseEntry.FolderName = entry.Name()
				} else {
					logThis.Error(err, VERBOSEST)
					continue
				}
			}

			// TODO for existing entries, maybe only reload if the metadata has been modified?

			// read information from metadata
			if err := fuseEntry.Load(fdb.Root); err != nil {
				logThis.Info("Error: could not load metadata for "+entry.Name(), VERBOSEST)
				continue
			}
			// save to database
			if err := fdb.DB.Save(&fuseEntry); err != nil {
				logThis.Error(err, VERBOSEST)
			}
		}
	}
	s.Stop()

	return nil
}
