package varroa

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	stateUnsorted = iota // has metadata but is unsorted
	stateAccepted        // has metadata and has been accepted, but not yet exported to library
	stateExported        // has metadata and has been exported to library
	stateRejected        // has metadata and is not to be exported to library

	currentDownloadsDBSchemaVersion = 1
)

var DownloadFolderStates = []string{"unsorted", "accepted", "exported", "rejected"}

func ColorizeDownloadState(value int, txt string) string {
	switch value {
	case stateAccepted:
		txt = Green(txt)
	case stateExported:
		txt = GreenBold(txt)
	case stateUnsorted:
		txt = Blue(txt)
	case stateRejected:
		txt = RedBold(txt)
	}
	return txt
}

func DownloadState(txt string) int {
	switch txt {
	case "accepted":
		return stateAccepted
	case "exported":
		return stateExported
	case "unsorted":
		return stateUnsorted
	case "rejected":
		return stateRejected
	}
	return -1
}

func IsValidDownloadState(txt string) bool {
	return DownloadState(txt) != -1
}

// -----------------------

type DownloadEntry struct {
	ID                 int      `storm:"id,increment"`
	FolderName         string   `storm:"unique"`
	State              int      `storm:"index"`
	Tracker            []string `storm:"index"`
	TrackerID          []int    `storm:"index"`
	Artists            []string `storm:"index"`
	HasTrackerMetadata bool     `storm:"index"`
	SchemaVersion      int
}

func (d *DownloadEntry) ShortState() string {
	return DownloadFolderStates[d.State][:1]
}

func (d *DownloadEntry) RawShortString() string {
	return fmt.Sprintf("[#%d]\t[%s]\t%s", d.ID, DownloadFolderStates[d.State][:1], d.FolderName)
}

func (d *DownloadEntry) ShortString() string {
	return ColorizeDownloadState(d.State, d.RawShortString())
}

func (d *DownloadEntry) String() string {
	return ColorizeDownloadState(d.State, fmt.Sprintf("ID #%d: %s [%s]", d.ID, d.FolderName, DownloadFolderStates[d.State]))
}

func (d *DownloadEntry) Description(root string) string {
	txt := d.String()
	if d.HasTrackerMetadata {
		txt += ", Has tracker metadata: "
		for i, t := range d.Tracker {
			txt += fmt.Sprintf("%s (ID #%d) ", t, d.TrackerID[i])
			txt += fmt.Sprintf("\n%s:\n%s", t, string(d.getDescription(root, t)))
		}
	} else {
		txt += ", does not have any tracker metadata."
	}
	return ColorizeDownloadState(d.State, txt)
}

// sorting: tracker name, tracker ID, foldername, description (ie releasemd), state
// generatePath: reads the release.json...

func (d *DownloadEntry) Load(root string) error {
	if d.FolderName == "" || !DirectoryExists(filepath.Join(root, d.FolderName)) {
		return errors.New("Wrong or missing path")
	}

	// find origin.json
	originFile := filepath.Join(root, d.FolderName, metadataDir, originJSONFile)
	if FileExists(originFile) {
		origin := TrackerOriginJSON{Path: originFile}
		if err := origin.load(); err != nil {
			return errors.Wrap(err, "Error reading origin.json")
		}
		// TODO: check last update timestamp, compare with value in db
		// TODO: if was not updated, skip.

		// TODO: remove duplicate if there are actually several origins

		// state: should be set to unsorted by default,
		// if it has already been set, leaving it as it is

		// resetting the other fields
		d.Tracker = []string{}
		d.TrackerID = []int{}
		d.Artists = []string{}
		d.HasTrackerMetadata = false
		if d.SchemaVersion != currentDownloadsDBSchemaVersion {
			//  migration if useful
		}
		d.SchemaVersion = currentDownloadsDBSchemaVersion

		// load useful things from JSON
		for tracker, info := range origin.Origins {
			d.Tracker = append(d.Tracker, tracker)
			d.TrackerID = append(d.TrackerID, info.ID)

			// getting release info from json
			infoJSON := filepath.Join(root, d.FolderName, metadataDir, tracker+"_"+trackerMetadataFile)
			infoJSONOldFormat := filepath.Join(root, d.FolderName, metadataDir, "Release.json")
			if !FileExists(infoJSON) {
				infoJSON = infoJSONOldFormat
			}
			if FileExists(infoJSON) {
				d.HasTrackerMetadata = true

				md := TrackerMetadata{}
				if err := md.LoadFromJSON(tracker, originFile, infoJSON); err != nil {
					return errors.Wrap(err, "Error loading JSON file "+infoJSON)
				}
				// extract relevant information!
				for _, a := range md.Artists {
					d.Artists = append(d.Artists, a.Name)
				}
			}
		}
	} else {
		return errors.New("Error, no metadata found")
	}
	return nil
}

func (d *DownloadEntry) getDescription(root, tracker string) []byte {
	// getting release.md info
	releaseMD := filepath.Join(root, d.FolderName, metadataDir, tracker+"_"+summaryFile)
	if !FileExists(releaseMD) {
		// if not present, try the old format
		releaseMD = filepath.Join(root, d.FolderName, metadataDir, "Release.md")
	}
	if FileExists(releaseMD) {
		fileBytes, err := ioutil.ReadFile(releaseMD)
		if err != nil {
			logThis.Error(err, NORMAL)
		} else {
			return fileBytes
		}
	}
	return []byte{}
}

func (d *DownloadEntry) getMetadata(root, tracker string) (TrackerMetadata, error) {
	// getting release info from json
	if !d.HasTrackerMetadata {
		return TrackerMetadata{}, errors.New("Error, does not have tracker metadata")
	}

	infoJSON := filepath.Join(root, d.FolderName, metadataDir, tracker+"_"+trackerMetadataFile)
	if !FileExists(infoJSON) {
		// if not present, try the old format
		infoJSON = filepath.Join(root, d.FolderName, metadataDir, "Release.json")
	}
	originJSON := filepath.Join(root, d.FolderName, metadataDir, originJSONFile)

	info := TrackerMetadata{}
	err := info.LoadFromJSON(tracker, originJSON, infoJSON)
	if err != nil {
		logThis.Error(errors.Wrap(err, "Error, could not load release json"), NORMAL)
	}
	return info, err
}

func (d *DownloadEntry) Sort(e *Environment, root string) error {
	// reading metadata
	if err := d.Load(root); err != nil {
		return err
	}
	fmt.Println("Sorting " + d.FolderName)
	// if mpd configured, allow playing the release...
	if e.config.MPD != nil && Accept("Load release into MPD") {
		fmt.Println("Sending to MPD.")
		mpdClient := MPD{}
		if err := mpdClient.Connect(e.config.MPD); err == nil {
			defer mpdClient.DisableAndDisconnect(root, d.FolderName)
			if err := mpdClient.SendAndPlay(root, d.FolderName); err != nil {
				fmt.Println(RedBold("Error sending to MPD: " + err.Error()))
			}
		}
	}
	// try to refresh metadata
	if d.HasTrackerMetadata && Accept("Try to refresh metadata from tracker") {
		for i, t := range d.Tracker {
			tracker, err := e.Tracker(t)
			if err != nil {
				logThis.Error(errors.Wrap(err, "Error getting configuration for tracker "+t), NORMAL)
				continue
			}
			if err := RefreshMetadata(e, tracker, []string{strconv.Itoa(d.TrackerID[i])}); err != nil {
				logThis.Error(errors.Wrap(err, "Error refreshing metadata for tracker "+t), NORMAL)
				continue
			}
		}
	}

	// offer to display metadata
	if d.HasTrackerMetadata && Accept("Display known metadata") {
		fmt.Println(d.Description(root))
	}

	fmt.Println(Green("This is where you decide what to do with this release. In any case, it will keep seeding until you remove it yourself or with your bittorrent client."))
	validChoice := false
	errs := 0
	for !validChoice {
		UserChoice("[A]ccept, [R]eject, or [D]efer decision : ")
		choice, scanErr := GetInput()
		if scanErr != nil {
			return scanErr
		}

		if strings.ToUpper(choice) == "R" {
			fmt.Println(RedBold("This release will be considered REJECTED. It will not be removed, but will be ignored in later sorting."))
			fmt.Println(RedBold("This can be reverted by sorting its specific download ID (" + strconv.Itoa(d.ID) + ")."))
			d.State = stateRejected
			validChoice = true
		} else if strings.ToUpper(choice) == "D" {
			fmt.Println(Green("Decision about this download is POSTPONED."))

			d.State = stateUnsorted
			validChoice = true
		} else if strings.ToUpper(choice) == "A" {
			fmt.Println(Green("This release is ACCEPTED. It will not be removed, but will be ignored in later sorting."))
			fmt.Println(Green("This can be reverted by sorting its specific download ID."))
			d.State = stateAccepted
			if Accept("Do you want to export it now ") {
				if err := d.export(root, e.config); err != nil {
					return err
				}
			} else {
				fmt.Println("The release was not exported. It can be exported later by sorting again.")
			}
			validChoice = true
		}
		if !validChoice {
			fmt.Println(Red("Invalid choice."))
			errs++
			if errs > 10 {
				return errors.New("Error sorting download, too many incorrect choices")
			}
		}
	}
	return nil
}

func (d *DownloadEntry) export(root string, config *Config) error {
	// getting candidates for new folder name
	candidates := []string{d.FolderName}
	if d.HasTrackerMetadata {
		for _, t := range d.Tracker {
			info, err := d.getMetadata(root, t)
			if err != nil {
				logThis.Info("Could not find metadata for tracker "+t, NORMAL)
				continue
			}
			candidates = append(candidates, info.GeneratePath(defaultFolderTemplate))
			candidates = append(candidates, info.GeneratePath(config.Library.FolderTemplate))
		}
	}
	// select or input a new name
	newName, err := SelectOption("Generating new folder name from metadata:\n", "Folder must not already exist.", candidates)
	// sanitizing in case of user input
	newName = SanitizeFolder(newName)
	if err != nil || DirectoryExists(filepath.Join(config.Library.Directory, newName)) {
		return errors.Wrap(err, "Error generating new release folder name")
	}
	// export
	if Accept("Export as " + newName) {
		fmt.Println("Exporting files to the library...")
		if err := CopyDir(filepath.Join(root, d.FolderName), filepath.Join(config.Library.Directory, newName), config.Library.UseHardLinks); err != nil {
			return errors.Wrap(err, "Error exporting download "+d.FolderName)
		}
		fmt.Println(Green("This release is now EXPORTED. It will not be removed, but will be ignored in later sortings."))
		d.State = stateExported
	} else {
		fmt.Println("The release was not exported. It can be exported later by sorting this ID again.")
	}
	return nil
}
