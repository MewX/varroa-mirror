package varroa

import (
	"encoding/json"
	"html"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/asdine/storm"
	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
)

// FuseEntry is the struct describing a release folder with tracker metadata.
// Only the FolderName is indexed.
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
		}
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
					fe.Artists = append(fe.Artists, SanitizeFolder(html.UnescapeString(el.Name)))
				}
				for _, el := range gt.Response.Group.MusicInfo.With {
					fe.Artists = append(fe.Artists, SanitizeFolder(html.UnescapeString(el.Name)))
				}
				for _, el := range gt.Response.Group.MusicInfo.Composers {
					fe.Artists = append(fe.Artists, SanitizeFolder(html.UnescapeString(el.Name)))
				}
				// record label
				fe.RecordLabel = gt.Response.Group.RecordLabel
				if gt.Response.Torrent.Remastered {
					fe.RecordLabel = gt.Response.Torrent.RemasterRecordLabel
				}
				fe.RecordLabel = SanitizeFolder(html.UnescapeString(fe.RecordLabel))
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

	} else {
		return errors.New("Error, no metadata found")
	}
	return nil
}

type FuseDB struct {
	Database
	Root string
}

func (fdb *FuseDB) Scan(path string) error {
	defer TimeTrack(time.Now(), "Scan FuseDB")

	if fdb.DB == nil {
		return errors.New("Error db not open")
	}
	if err := fdb.DB.Init(&FuseEntry{}); err != nil {
		return errors.New("Could not prepare database for indexing fuse entries")
	}

	if !DirectoryExists(path) {
		return errors.New("Error finding " + path)
	}
	fdb.Root = path

	// don't walk, we only want the top-level directories here
	entries, readErr := ioutil.ReadDir(fdb.Root)
	if readErr != nil {
		return errors.Wrap(readErr, "Error reading target directory")
	}

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = scanningFiles
	s.Start()

	// get old entries
	var previous []FuseEntry
	if err := fdb.DB.All(&previous); err != nil {
		return errors.New("Cannot load previous entries")
	}

	tx, err := fdb.DB.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentFolderNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			// detect if sound files are present, leave otherwise
			if !DirectoryContainsMusic(filepath.Join(fdb.Root, entry.Name())) {
				logThis.Info("Error: no music found in "+entry.Name(), VERBOSEST)
				continue
			}
			// try to find entry
			var fuseEntry FuseEntry
			if dbErr := fdb.DB.One("FolderName", entry.Name(), &fuseEntry); dbErr != nil {
				if dbErr == storm.ErrNotFound {
					// not found, create new entry
					fuseEntry.FolderName = entry.Name()
					// read information from metadata
					if err := fuseEntry.Load(fdb.Root); err != nil {
						logThis.Error(errors.Wrap(err, "Error: could not load metadata for "+entry.Name()), VERBOSEST)
						continue
					}
					if err := tx.Save(&fuseEntry); err != nil {
						logThis.Info("Error: could not save to db "+entry.Name(), VERBOSEST)
						continue
					}
					logThis.Info("New FuseDB entry: "+entry.Name(), VERBOSESTEST)
				} else {
					logThis.Error(dbErr, VERBOSEST)
					continue
				}
			} else {
				// found entry, update it
				// TODO for existing entries, maybe only reload if the metadata has been modified?
				// read information from metadata
				if err := fuseEntry.Load(fdb.Root); err != nil {
					logThis.Info("Error: could not load metadata for "+entry.Name(), VERBOSEST)
					continue
				}
				if err := tx.Update(&fuseEntry); err != nil {
					logThis.Info("Error: could not save to db "+entry.Name(), VERBOSEST)
					continue
				}
				logThis.Info("Updated FuseDB entry: "+entry.Name(), VERBOSESTEST)
			}
			currentFolderNames = append(currentFolderNames, entry.Name())
		}
	}

	// remove entries no longer associated with actual files
	for _, p := range previous {
		if !StringInSlice(p.FolderName, currentFolderNames) {
			if err := tx.DeleteStruct(&p); err != nil {
				logThis.Error(err, VERBOSEST)
			}
			logThis.Info("Removed FuseDB entry: "+p.FolderName, VERBOSESTEST)
		}
	}

	defer TimeTrack(time.Now(), "Committing changes to DB")
	if err := tx.Commit(); err != nil {
		return err
	}

	s.Stop()
	return nil
}
