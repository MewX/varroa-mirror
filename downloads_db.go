package varroa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

const (
	stateUnsorted DownloadState = iota // has metadata but is unsorted
	stateAccepted                      // has metadata and has been accepted, but not yet exported to library
	stateExported                      // has metadata and has been exported to library
	stateRejected                      // has metadata and is not to be exported to library
)

var DownloadFolderStates = []string{"unsorted", "accepted", "exported", "rejected"}

type DownloadState int

func (ds DownloadState) Colorize(txt string) string {
	switch ds {
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

func (ds DownloadState) Get(txt string) DownloadState {
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
	return DownloadState(-1).Get(txt) != -1

}

//-----------------------

type DownloadEntry struct {
	ID                 int           `storm:"id,increment"`
	FolderName         string        `storm:"unique"`
	State              DownloadState `storm:"index"`
	Tracker            []string      `storm:"index"`
	TrackerID          []int         `storm:"index"`
	Artists            []string      `storm:"index"`
	HasTrackerMetadata bool          `storm:"index"`
}

func (d *DownloadEntry) ShortState() string {
	return DownloadFolderStates[d.State][:1]
}

func (d *DownloadEntry) RawShortString() string {
	return fmt.Sprintf("[#%d]\t[%s]\t%s", d.ID, DownloadFolderStates[d.State][:1], d.FolderName)
}

func (d *DownloadEntry) ShortString() string {
	return d.State.Colorize(d.RawShortString())
}

func (d *DownloadEntry) String() string {
	return d.State.Colorize(fmt.Sprintf("ID #%d: %s [%s]", d.ID, d.FolderName, DownloadFolderStates[d.State]))
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
	return d.State.Colorize(txt)
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
		d.HasTrackerMetadata = false

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
					d.Artists = append(d.Artists, html.UnescapeString(el.Name))
				}
				for _, el := range gt.Response.Group.MusicInfo.With {
					d.Artists = append(d.Artists, html.UnescapeString(el.Name))
				}
				for _, el := range gt.Response.Group.MusicInfo.Composers {
					d.Artists = append(d.Artists, html.UnescapeString(el.Name))
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
		bytes, err := ioutil.ReadFile(releaseMD)
		if err != nil {
			logThis.Error(err, NORMAL)
		} else {
			return bytes
		}
	}
	return []byte{}
}

func (d *DownloadEntry) getMetadata(root, tracker string) (TrackerTorrentInfo, error) {
	// getting release info from json
	if !d.HasTrackerMetadata {
		return TrackerTorrentInfo{}, errors.New("Error, does not have tracker metadata")
	}

	infoJSON := filepath.Join(root, d.FolderName, metadataDir, tracker+"_"+trackerMetadataFile)
	if !FileExists(infoJSON) {
		// if not present, try the old format
		infoJSON = filepath.Join(root, d.FolderName, metadataDir, "Release.json")
	}

	info := TrackerTorrentInfo{}
	if err := info.Load(infoJSON); err != nil {
		logThis.Error(err, NORMAL)
		return TrackerTorrentInfo{}, errors.Wrap(err, "Error, could not load release json")
	}
	return info, nil
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
			candidates = append(candidates, d.generatePath(t, info, defaultFolderTemplate))
			candidates = append(candidates, d.generatePath(t, info, config.Library.FolderTemplate))
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
		fmt.Println("Exporting files to the library root...")
		if err := CopyDir(filepath.Join(root, d.FolderName), filepath.Join(config.Library.Directory, newName), config.Library.UseHardLinks); err != nil {
			return errors.Wrap(err, "Error exporting download "+d.FolderName)
		}
		fmt.Println(Green("This release is now EXPORTED. It will not be removed, but will be ignored in later sorting."))
		d.State = stateExported
	} else {
		fmt.Println("The release was not exported. It can be exported later with the 'downloads export' subcommand.")
	}
	return nil
}

func (d *DownloadEntry) generatePath(tracker string, info TrackerTorrentInfo, folderTemplate string) string {
	if folderTemplate == "" || !d.HasTrackerMetadata {
		return d.FolderName
	}

	gt := info.FullInfo()
	if gt == nil {
		return d.FolderName // nothing useful here
	}
	// parsing info that needs to be worked on before use
	artists := []string{}
	// for now, using artists, composers categories
	for _, el := range gt.Response.Group.MusicInfo.Artists {
		artists = append(artists, el.Name)
	}
	for _, el := range gt.Response.Group.MusicInfo.Composers {
		artists = append(artists, el.Name)
	}
	artistsShort := strings.Join(artists, ", ")
	// TODO do better.
	if len(artists) >= 3 {
		artistsShort = "Various Artists"
	}
	originalYear := gt.Response.Group.Year

	// usual edition specifiers, shortened
	editionReplacer := strings.NewReplacer(
		"Reissue", "RE",
		"Repress", "RP",
		"Remaster", "RM",
		"Remastered", "RM",
		"Limited Edition", "LTD",
		"Deluxe", "DLX",
		"Deluxe Edition", "DLX",
		"Special Editon", "SE",
		"Bonus Tracks", "Bonus",
		"Bonus Tracks Edition", "Bonus",
		"Promo", "PR",
		"Test Pressing", "TP",
		"Self Released", "SR",
		"Box Set", "Box set",
		"Compact Disc Recordable", "CDr",
		"Japan Edition", "Japan",
		"Japan Release", "Japan",
	)
	editionName := editionReplacer.Replace(gt.Response.Torrent.RemasterTitle)

	// identifying info
	var idElements []string
	if gt.Response.Torrent.Remastered && gt.Response.Torrent.RemasterYear != originalYear {
		idElements = append(idElements, fmt.Sprintf("%d", gt.Response.Torrent.RemasterYear))
	}
	if editionName != "" {
		idElements = append(idElements, editionName)
	}
	// adding catalog number, or if not specified, the record label
	if gt.Response.Torrent.RemasterCatalogueNumber != "" {
		idElements = append(idElements, gt.Response.Torrent.RemasterCatalogueNumber)
	} else if gt.Response.Group.CatalogueNumber != "" {
		idElements = append(idElements, gt.Response.Group.CatalogueNumber)
	} else {
		if gt.Response.Torrent.RemasterRecordLabel != "" {
			idElements = append(idElements, gt.Response.Torrent.RemasterRecordLabel)
		} else if gt.Response.Group.RecordLabel != "" {
			idElements = append(idElements, gt.Response.Group.RecordLabel)
		}
		// TODO else unkown release!
	}
	if gt.Response.Group.ReleaseType != 1 {
		// adding release type if not album
		idElements = append(idElements, getGazelleReleaseType(gt.Response.Group.ReleaseType))
	}
	id := strings.Join(idElements, ", ")

	// format
	var format string
	switch gt.Response.Torrent.Encoding {
	case qualityLossless:
		format = "FLAC"
	case quality24bitLossless:
		format = "FLAC24"
	case qualityV0:
		format = "V0"
	case qualityV2:
		format = "V2"
	case quality320:
		format = "320"
	default:
		format = "UnF"
	}

	// source
	source := gt.Source()

	r := strings.NewReplacer(
		"$id", "{{$id}}",
		"$a", "{{$a}}",
		"$t", "{{$t}}",
		"$y", "{{$y}}",
		"$f", "{{$f}}",
		"$s", "{{$s}}",
		"$l", "{{$l}}",
		"$n", "{{$n}}",
		"$e", "{{$e}}",
		"$g", "{{$g}}",
		"{", "ÆÆ", // otherwise golang's template throws a fit if '{' or '}' are in the user pattern
		"}", "¢¢", // assuming these character sequences will probably not cause conflicts.
	)

	// replace with all valid epub parameters
	tmpl := fmt.Sprintf(`{{$a := "%s"}}{{$y := "%d"}}{{$t := "%s"}}{{$f := "%s"}}{{$s := "%s"}}{{$g := "%s"}}{{$l := "%s"}}{{$n := "%s"}}{{$e := "%s"}}{{$id := "%s"}}%s`,
		artistsShort,
		originalYear,
		gt.Response.Group.Name, // title
		format,
		gt.Response.Torrent.Media, // original source
		source, // source with indicator if 100%/log/cue or Silver/gold
		gt.Response.Group.RecordLabel,     // label
		gt.Response.Group.CatalogueNumber, // catalog number
		editionName,                       // edition
		id,                                // identifying info
		r.Replace(folderTemplate))

	var doc bytes.Buffer
	te := template.Must(template.New("hop").Parse(tmpl))
	if err := te.Execute(&doc, nil); err != nil {
		return d.FolderName
	}
	newName := strings.TrimSpace(doc.String())

	// recover brackets
	r2 := strings.NewReplacer(
		"ÆÆ", "{",
		"¢¢", "}",
	)
	newName = r2.Replace(newName)

	// making sure the final filename is valid
	return SanitizeFolder(newName)
}
