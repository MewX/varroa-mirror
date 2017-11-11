package varroa

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/briandowns/spinner"
	"github.com/pkg/errors"
	"github.com/sevlyar/go-daemon"
)

type Downloads struct {
	Root string
	Database
}

func (d *Downloads) String() string {
	txt := "Downloads in database:\n"
	var allEntries []DownloadEntry
	if err := d.DB.All(&allEntries); err != nil {
		txt += err.Error()
	} else {
		for _, dl := range allEntries {
			txt += "\t" + dl.ShortString() + "\n"
		}
	}
	return txt
}

func (d *Downloads) Scan() error {
	defer TimeTrack(time.Now(), "Scan Downloads")

	if d.DB == nil {
		return errors.New("Error db not open")
	}
	if err := d.DB.Init(&DownloadEntry{}); err != nil {
		return errors.New("Could not prepare database for indexing download entries")
	}

	if !DirectoryExists(d.Root) {
		return errors.New("Error finding " + d.Root)
	}

	// don't walk, we only want the top-level directories here
	entries, readErr := ioutil.ReadDir(d.Root)
	if readErr != nil {
		return errors.Wrap(readErr, "Error reading downloads directory")
	}

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = scanningFiles
	if !daemon.WasReborn() {
		s.Start()
	}

	// get old entries
	var previous []DownloadEntry
	if err := d.DB.All(&previous); err != nil {
		return errors.New("Cannot load previous entries")
	}

	tx, err := d.DB.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentFolderNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			// detect if sound files are present, leave otherwise
			if !DirectoryContainsMusic(filepath.Join(d.Root, entry.Name())) {
				logThis.Info("Error: no music found in "+entry.Name(), VERBOSEST)
				continue
			}
			// try to find entry
			var downloadEntry DownloadEntry
			if dbErr := d.DB.One("FolderName", entry.Name(), &downloadEntry); dbErr != nil {
				if dbErr == storm.ErrNotFound {
					// not found, create new entry
					downloadEntry.FolderName = entry.Name()
					// read information from metadata
					if err := downloadEntry.Load(d.Root); err != nil {
						logThis.Error(errors.Wrap(err, "Error: could not load metadata for "+entry.Name()), VERBOSEST)
						continue
					}
					if err := tx.Save(&downloadEntry); err != nil {
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
				if err := downloadEntry.Load(d.Root); err != nil {
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

func (d *Downloads) LoadAndScan(path string) error {
	if err := d.Open(path); err != nil {
		return errors.New(errorLoadingDownloadsDB)
	}
	if err := d.Scan(); err != nil {
		return errors.New("Error scanning downloads")
	}
	return nil
}

func (d *Downloads) FindByID(id int) (DownloadEntry, error) {
	var downloadEntry DownloadEntry
	if err := d.DB.One("ID", id, &downloadEntry); err != nil {
		return DownloadEntry{}, err
	}
	return downloadEntry, nil
}

func (d *Downloads) Sort(e *Environment) error {
	var downloadEntries []DownloadEntry
	query := d.DB.Select(q.Or(q.Eq("State", stateUnsorted), q.Eq("State", stateAccepted))).OrderBy("FolderName")
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
			if err := dl.Sort(e, d.Root); err != nil {
				return errors.Wrap(err, "Error sorting download "+strconv.Itoa(dl.ID))
			}
		} else if dl.State == stateAccepted {
			if Accept(fmt.Sprintf("Do you want to export already accepted release #%d (%s) ", dl.ID, dl.FolderName)) {
				if err := dl.export(d.Root, e.config); err != nil {
					return errors.Wrap(err, "Error exporting download "+strconv.Itoa(dl.ID))
				}
			} else {
				fmt.Println("The release was not exported. It can be exported later by sorting again.")
			}
		}
		if err := d.DB.Update(&dl); err != nil {
			return errors.Wrap(err, "Error saving new state for download "+dl.FolderName)
		}
	}
	return nil
}

func (d *Downloads) SortThisID(e *Environment, id int) error {
	dl, err := d.FindByID(id)
	if err != nil {
		return errors.Wrap(err, "Error finding such an ID in the downloads database")
	}
	if err := dl.Sort(e, d.Root); err != nil {
		return errors.Wrap(err, "Error sorting selected download")
	}
	if err := d.DB.Update(&dl); err != nil {
		return errors.Wrap(err, "Error saving new state for download "+dl.FolderName)
	}
	return nil
}

func (d *Downloads) FindByState(state string) []DownloadEntry {
	if !StringInSlice(state, DownloadFolderStates) {
		logThis.Info("Invalid state", NORMAL)
	}

	dlState := DownloadState(-1).Get(state)
	var hits []DownloadEntry
	if err := d.DB.Find("State", dlState, &hits); err != nil && err != storm.ErrNotFound {
		logThis.Error(errors.Wrap(err, "Could not find downloads by state"), VERBOSEST)
	}
	return hits
}

func (d *Downloads) FindByArtist(artist string) []DownloadEntry {
	var hits []DownloadEntry
	query := d.DB.Select(InSlice("Artists", artist))
	if err := query.Find(&hits); err != nil && err != storm.ErrNotFound {
		logThis.Error(errors.Wrap(err, "Could not find downloads by artist "+artist), VERBOSEST)
	}
	return hits
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
	var toBeMoved []os.FileInfo

	s := spinner.New([]string{"    ", ".   ", "..  ", "... "}, 150*time.Millisecond)
	s.Prefix = scanningFiles
	if !daemon.WasReborn() {
		s.Start()
	}

	// don't walk, we only want the top-level directories here
	entries, err := ioutil.ReadDir(d.Root)
	if err != nil {
		return errors.Wrap(err, "Error readingg directory "+d.Root)
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
		if err := os.Rename(filepath.Join(d.Root, r.Name()), filepath.Join(cleanDir, r.Name())); err != nil {
			return errors.Wrap(err, errorCleaningDownloads+r.Name())
		}
	}
	return nil
}
