package varroa

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
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
	Source      string   `storm:"index"`
	Format      string   `storm:"index"`
}

func (fe *FuseEntry) reset() {
	fe.Artists = []string{}
	fe.Tags = []string{}
	fe.Title = ""
	fe.Year = 0
	fe.Tracker = []string{}
	fe.RecordLabel = ""
	fe.Source = ""
	fe.Format = ""
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
				md := TrackerMetadata{}
				if err := md.LoadFromJSON(tracker, originFile, infoJSON); err != nil {
					return errors.Wrap(err, "Error loading JSON file "+infoJSON)
				}
				// extract relevant information!
				// for now, using artists, composers, "with" categories
				// extract relevant information!
				for _, a := range md.Artists {
					fe.Artists = append(fe.Artists, a.Name)
				}
				fe.RecordLabel = SanitizeFolder(md.RecordLabel)
				fe.Year = md.OriginalYear // only show original year
				fe.Title = md.Title
				fe.Tags = md.Tags
				fe.Source = md.SourceFull
				fe.Format = ShortEncoding(md.Quality)
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

func (fdb *FuseDB) contains(category, value string, inSlice bool) bool {
	var query storm.Query
	if inSlice {
		query = fdb.DB.Select(InSlice(category, value)).Limit(1)
	} else {
		query = fdb.DB.Select(q.Eq(category, value)).Limit(1)
	}
	var entry FuseEntry
	if err := query.First(&entry); err != nil {
		if err == storm.ErrNotFound {
			logThis.Info("Unknown value for "+category+": "+value, VERBOSEST)
			return false
		}
		logThis.Error(err, VERBOSEST)
		return false
	}
	return true
}

func (fdb *FuseDB) uniqueEntries(matcher q.Matcher, field string) ([]string, error) {
	// get all matching entries
	var allEntries []FuseEntry
	query := fdb.DB.Select(matcher)
	if err := query.Find(&allEntries); err != nil {
		logThis.Error(err, VERBOSEST)
		return []string{}, err
	}
	// get all different values
	var allValues []string
	for _, e := range allEntries {
		switch field {
		case "Tags":
			allValues = append(allValues, e.Tags...)
		case "Source":
			allValues = append(allValues, e.Source)
		case "Format":
			allValues = append(allValues, e.Format)
		case "Year":
			allValues = append(allValues, strconv.Itoa(e.Year))
		case "RecordLabel":
			allValues = append(allValues, e.RecordLabel)
		case "Artists":
			allValues = append(allValues, e.Artists...)
		case "FolderName":
			allValues = append(allValues, e.FolderName)
		}
	}
	return RemoveStringSliceDuplicates(allValues), nil
}
