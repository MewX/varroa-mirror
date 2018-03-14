package varroa

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
	"github.com/sevlyar/go-daemon"
)

var downloadsDB *DownloadsDB
var onceDownloadsDB sync.Once

type DownloadsDB struct {
	root              string
	additionalSources []string
	db                *Database
}

func NewDownloadsDB(path, root string, additionalSources []string) (*DownloadsDB, error) {
	var returnErr error
	onceDownloadsDB.Do(func() {
		// db should be opened already
		db, err := NewDatabase(path)
		if err != nil {
			returnErr = errors.Wrap(err, "Error opening stats database")
			return
		}
		downloadsDB = &DownloadsDB{db: db, root: root, additionalSources: additionalSources}
		if returnErr = downloadsDB.init(); returnErr != nil {
			logThis.Error(errors.Wrap(returnErr, "Could not prepare database for indexing download entries"), NORMAL)
			return
		}
		if !DirectoryExists(downloadsDB.root) {
			logThis.Info("Error finding "+root, NORMAL)
			return
		}
	})
	return downloadsDB, returnErr
}

func (d *DownloadsDB) init() error {
	return d.db.DB.Init(&DownloadEntry{})
}

func (d *DownloadsDB) Close() error {
	return d.db.Close()
}

func (d *DownloadsDB) String() string {
	txt := "Downloads in database:\n"
	var allEntries []DownloadEntry
	if err := d.db.DB.All(&allEntries); err != nil {
		txt += err.Error()
	} else {
		for _, dl := range allEntries {
			txt += " ▹ " + dl.ShortString() + "\n"
		}
	}
	var stateCounts []string
	for _, s := range DownloadFolderStates {
		states := d.FindByState(s)
		stateCounts = append(stateCounts, fmt.Sprintf("%s: %d (%.02f%%)", s, len(states), 100*float32(len(states))/float32(len(allEntries))))
	}
	txt += "\n" + YellowUnderlined(fmt.Sprintf("Total: %d entries ~~ ", len(allEntries))+strings.Join(stateCounts, ", "))
	return txt
}

func (d *DownloadsDB) Scan() error {
	defer TimeTrack(time.Now(), "Scan Downloads")

	if d.db.DB == nil {
		return errors.New("Error db not open")
	}

	// don't walk, we only want the top-level directories here
	entries, readErr := ioutil.ReadDir(d.root)
	if readErr != nil {
		return errors.Wrap(readErr, "Error reading downloads directory "+d.root)
	}

	// same from additional sources
	for _, s := range d.additionalSources {
		se, err := ioutil.ReadDir(s)
		if err != nil {
			return errors.Wrap(readErr, "Error reading downloads directory "+s)
		}
		entries = append(entries, se...)
	}

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = scanningFiles
	if !daemon.WasReborn() {
		s.Start()
	}

	// get old entries
	var previous []DownloadEntry
	if err := d.db.DB.All(&previous); err != nil {
		return errors.New("Cannot load previous entries")
	}

	tx, err := d.db.DB.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentFolderNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			// detect if sound files are present, leave otherwise
			if !DirectoryContainsMusic(filepath.Join(d.root, entry.Name())) {
				logThis.Info("Error: no music found in "+entry.Name(), VERBOSEST)
				continue
			}
			// try to find entry
			var downloadEntry DownloadEntry
			if dbErr := d.db.DB.One("FolderName", entry.Name(), &downloadEntry); dbErr != nil {
				if dbErr == storm.ErrNotFound {
					// not found, create new entry
					downloadEntry.FolderName = entry.Name()
					// read information from metadata
					if err := downloadEntry.Load(d.root); err != nil {
						logThis.Error(errors.Wrap(err, "Error: could not load metadata for "+entry.Name()), VERBOSEST)
						continue
					}
					if err := tx.Save(&downloadEntry); err != nil {
						logThis.Info("Error: could not save to db "+entry.Name(), VERBOSEST)
						continue
					}
					logThis.Info("New Downloads entry: "+entry.Name(), VERBOSESTEST)
				} else {
					logThis.Error(dbErr, VERBOSEST)
					continue
				}
			} else {
				// found entry, update it
				// TODO for existing entries, maybe only reload if the metadata has been modified?
				// read information from metadata
				if err := downloadEntry.Load(d.root); err != nil {
					logThis.Info("Error: could not load metadata for "+entry.Name(), VERBOSEST)
					continue
				}
				if err := tx.Update(&downloadEntry); err != nil {
					logThis.Info("Error: could not save to db "+entry.Name(), VERBOSEST)
					continue
				}
				logThis.Info("Updated Downloads entry: "+entry.Name(), VERBOSESTEST)
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
			logThis.Info("Removed Download entry: "+p.FolderName, VERBOSESTEST)
		}
	}

	defer TimeTrack(time.Now(), "Committing changes to DB")
	if err := tx.Commit(); err != nil {
		return err
	}

	if !daemon.WasReborn() {
		s.Stop()
	}
	return nil
}

func (d *DownloadsDB) RescanIDs(IDs []int) error {
	// retrieve the associated DownloadEntries
	var entries []DownloadEntry
	for _, id := range IDs {
		dl, err := d.FindByID(id)
		if err != nil {
			if err == storm.ErrNotFound {
				logThis.Error(errors.Wrap(err, fmt.Sprintf("cannot retrieve entry for ID %d", id)), NORMAL)
			} else {
				return errors.Wrap(err, fmt.Sprintf("error looking for ID %d", id))
			}
		}
		entries = append(entries, dl)
	}
	if len(entries) == 0 {
		return errors.New("none of the IDs could be found in the database")
	}

	// begin transaction
	tx, err := d.db.DB.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// update the entries
	for _, entry := range entries {
		if DirectoryExists(entry.FolderName) {
			// read information from metadata
			if err := entry.Load(d.root); err != nil {
				logThis.Info("Error: could not load metadata for "+entry.FolderName, VERBOSEST)
				continue
			}
			if err := tx.Update(&entry); err != nil {
				logThis.Info("Error: could not save to db "+entry.FolderName, VERBOSEST)
				continue
			}
			logThis.Info("Updated Downloads entry: "+entry.FolderName, VERBOSESTEST)
		} else {
			if err := tx.DeleteStruct(&entry); err != nil {
				logThis.Error(err, VERBOSEST)
			}
			logThis.Info("Removed Download entry: "+entry.FolderName, VERBOSESTEST)
		}
	}
	// commiting transaction
	return tx.Commit()
}

func (d *DownloadsDB) locateFolderName(folderName string) (string, string, error) {
	// getting the absolute path
	absFolderName, err := filepath.Abs(folderName)
	if err != nil {
		return "", "", err
	}

	var found bool
	var basePath string
	c, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		return "", "", err
	}
	if !c.LibraryConfigured || !c.DownloadFolderConfigured {
		return "", "", errors.New("insufficient information from the configuration file: download directory, library section.")
	}

	rel, err := filepath.Rel(c.General.DownloadDir, absFolderName)
	if err != nil {
		return "", "", err
	}
	if filepath.Clean(folderName) == rel {
		basePath = c.General.DownloadDir
		found = true
	}
	if !found {
		for _, s := range c.Library.AdditionalSources {
			rel, err = filepath.Rel(s, absFolderName)
			if err != nil {
				logThis.Error(err, VERBOSESTEST)
				continue
			}
			if filepath.Clean(folderName) == rel {
				basePath = s
				found = true
			}
		}
	}
	if !found {
		return "", "", errors.New("this directory could not be found in the download directory or in other defined sources")
	}
	return absFolderName, basePath, nil
}

func (d *DownloadsDB) RescanPath(folderName string) error {
	if DirectoryExists(folderName) {
		// checking it's inside a known directory (download directory or additional source)
		absFolderName, basePath, err := d.locateFolderName(folderName)
		if err != nil {
			return err
		}

		// begin transaction
		tx, err := d.db.DB.Begin(true)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		var newEntry bool
		dl, err := d.FindByFolderName(folderName)
		if err != nil {
			if err == storm.ErrNotFound {
				logThis.Info("Adding new entry!", NORMAL)
				newEntry = true
				dl.FolderName = folderName
			} else {
				return errors.Wrap(err, fmt.Sprintf("error looking for entry %s", absFolderName))
			}
		}

		// read information from metadata
		if err := dl.Load(basePath); err != nil {
			return errors.Wrap(err, "error: could not load metadata for "+absFolderName)
		}
		if newEntry {
			if err := tx.Save(&dl); err != nil {
				return errors.Wrap(err, "error: could not save to db "+absFolderName)
			}
		} else {
			if err := tx.Update(&dl); err != nil {
				return errors.Wrap(err, "error: could not save to db "+absFolderName)
			}
			logThis.Info("Updated Downloads entry: "+absFolderName, VERBOSESTEST)
		}

		// commiting transaction
		return tx.Commit()
	}
	return errors.New(folderName + " could not be found")
}

func (d *DownloadsDB) FindByID(id int) (DownloadEntry, error) {
	var downloadEntry DownloadEntry
	if err := d.db.DB.One("ID", id, &downloadEntry); err != nil {
		return DownloadEntry{}, err
	}
	return downloadEntry, nil
}

func (d *DownloadsDB) FindByFolderName(folderName string) (DownloadEntry, error) {
	var downloadEntry DownloadEntry
	if err := d.db.DB.One("FolderName", folderName, &downloadEntry); err != nil {
		return DownloadEntry{}, err
	}
	return downloadEntry, nil
}

func (d *DownloadsDB) Sort(e *Environment) error {
	var downloadEntries []DownloadEntry
	query := d.db.DB.Select(q.Or(q.Eq("State", stateUnsorted), q.Eq("State", stateAccepted))).OrderBy("FolderName")
	if err := query.Find(&downloadEntries); err != nil {
		if err == storm.ErrNotFound {
			logThis.Info("Everything is sorted. Congratulations!", NORMAL)
			return nil
		}
		return err
	}
	for _, dl := range downloadEntries {
		if dl.State == stateUnsorted {
			if !Accept(fmt.Sprintf("Sorting download #%d (%s), continue ", dl.ID, dl.FolderName)) {
				return nil
			}
			if err := dl.Sort(e, d.root); err != nil {
				return errors.Wrap(err, "Error sorting download "+strconv.Itoa(dl.ID))
			}
		} else if dl.State == stateAccepted {
			if Accept(fmt.Sprintf("Do you want to export already accepted release #%d (%s) ", dl.ID, dl.FolderName)) {
				if err := dl.export(d.root, e.config); err != nil {
					return errors.Wrap(err, "Error exporting download "+strconv.Itoa(dl.ID))
				}
			} else {
				fmt.Println("The release was not exported. It can be exported later by sorting again.")
			}
		}
		if err := d.db.DB.Update(&dl); err != nil {
			return errors.Wrap(err, "Error saving new state for download "+dl.FolderName)
		}
	}
	return nil
}

func (d *DownloadsDB) SortThisID(e *Environment, id int) error {
	dl, err := d.FindByID(id)
	if err != nil {
		return errors.Wrap(err, "Error finding such an ID in the downloads database")
	}
	if err := dl.Sort(e, d.root); err != nil {
		return errors.Wrap(err, "Error sorting selected download")
	}
	if err := d.db.DB.Update(&dl); err != nil {
		return errors.Wrap(err, "Error saving new state for download "+dl.FolderName)
	}
	return nil
}

func (d *DownloadsDB) FindByState(state string) []DownloadEntry {
	if !StringInSlice(state, DownloadFolderStates) {
		logThis.Info("Invalid state", NORMAL)
	}
	var hits []DownloadEntry
	dlState := DownloadState(state)
	if dlState == -1 {
		logThis.Info("Unknown state", VERBOSEST)
	} else {
		if err := d.db.DB.Select(q.Eq("State", dlState)).Find(&hits); err != nil {
			if err == storm.ErrNotFound {
				logThis.Error(errors.Wrap(err, "Could not find downloads by state"), VERBOSEST)
			} else {
				logThis.Error(errors.Wrap(err, "Could not search downloads database"), VERBOSEST)
			}
		}
	}
	return hits
}

func (d *DownloadsDB) FindByArtist(artist string) []DownloadEntry {
	var hits []DownloadEntry
	query := d.db.DB.Select(InSlice("Artists", artist))
	if err := query.Find(&hits); err != nil && err != storm.ErrNotFound {
		logThis.Error(errors.Wrap(err, "Could not find downloads by artist "+artist), VERBOSEST)
	}
	return hits
}

func (d *DownloadsDB) Clean() error {
	// prepare directory for cleaned folders if necessary
	cleanDir := filepath.Join(d.root, downloadsCleanDir)
	if !DirectoryExists(cleanDir) {
		if err := os.MkdirAll(cleanDir, 0777); err != nil {
			return errors.Wrap(err, errorCreatingDownloadsCleanDir)
		}
	}

	// don't walk, we only want the top-level directories here
	var toBeMoved []os.FileInfo

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = scanningFiles
	if !daemon.WasReborn() {
		s.Start()
	}

	// don't walk, we only want the top-level directories here
	entries, err := ioutil.ReadDir(d.root)
	if err != nil {
		return errors.Wrap(err, "Error readingg directory "+d.root)
	}
	for _, entry := range entries {
		if entry.Name() != downloadsCleanDir && entry.IsDir() {
			// read at most 2 entries insinde entry
			f, err := os.Open(filepath.Join(d.root, entry.Name()))
			if err != nil {
				logThis.Error(errors.Wrap(err, "Error opening "+entry.Name()), VERBOSE)
				continue
			}
			contents, err := f.Readdir(2)
			f.Close()
			// move if empty or if the directory only contains tracker metadata
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
		if err := os.Rename(filepath.Join(d.root, r.Name()), filepath.Join(cleanDir, r.Name())); err != nil {
			return errors.Wrap(err, errorCleaningDownloads+r.Name())
		}
	}
	return nil
}
