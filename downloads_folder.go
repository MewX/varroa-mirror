package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
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

var downloadFolderStates = []string{"unsorted", "accepted", "exported", "rejected"}

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

//-----------------------

type DownloadFolder struct {
	Index              uint64
	Path               string
	Root               string
	Metadata           map[string]TrackerTorrentInfo
	State              DownloadState
	Trackers           []string
	ID                 map[string]int
	GroupID            map[string]int
	HasTrackerMetadata bool
	HasOriginJSON      bool
	HasDescription     bool
	HasInfo            bool
	ReleaseInfo        map[string][]byte
}

func (d *DownloadFolder) ShortState() string {
	return downloadFolderStates[d.State][:1]
}

func (d *DownloadFolder) RawShortString() string {
	return fmt.Sprintf("[#%d]\t[%s]\t%s", d.Index, downloadFolderStates[d.State][:1], d.Path)
}

func (d *DownloadFolder) ShortString() string {
	return d.State.Colorize(d.RawShortString())
}

func (d *DownloadFolder) String() string {
	return d.State.Colorize(fmt.Sprintf("ID #%d: %s [%s]", d.Index, d.Path, downloadFolderStates[d.State]))
}

func (d *DownloadFolder) Description() string {
	txt := d.String()
	if d.HasTrackerMetadata {
		txt += ", Has tracker metadata: "
		if d.HasOriginJSON {
			for _, t := range d.Trackers {
				txt += fmt.Sprintf("%s (ID #%d, GID #%d) ", t, d.ID[t], d.GroupID[t])
			}
		}
		if d.HasDescription {
			for _, t := range d.Trackers {
				txt += fmt.Sprintf("\n%s:\n%s", t, string(d.ReleaseInfo[t]))
			}
		} else if d.HasInfo {
			for _, t := range d.Trackers {
				artists := []string{}
				for k := range d.Metadata[t].artists {
					artists = append(artists, k)
				}
				txt += fmt.Sprintf("| %s metadata: Artist(s): %s / Label: %s ", t, strings.Join(artists, ", "), d.Metadata[t].label)
			}
		}
	} else {
		txt += ", does not have any tracker metadata."
	}
	return d.State.Colorize(txt)
}

func (d *DownloadFolder) init() {
	if d.ID == nil {
		d.ID = make(map[string]int)
	}
	if d.GroupID == nil {
		d.GroupID = make(map[string]int)
	}
	if d.ReleaseInfo == nil {
		d.ReleaseInfo = make(map[string][]byte)
	}
	if d.Metadata == nil {
		d.Metadata = make(map[string]TrackerTorrentInfo)
	}
}

func (d *DownloadFolder) Load() error {
	if d.Path == "" {
		return errors.New("Error, download folder path not set")
	}
	// init if necessary
	d.init()

	// detect if sound files are present, leave otherwise
	if err := filepath.Walk(filepath.Join(d.Root, d.Path), func(path string, f os.FileInfo, err error) error {
		if StringInSlice(strings.ToLower(filepath.Ext(path)), []string{mp3Ext, flacExt}) {
			// stop walking the directory as soon as a track is found
			return errors.New(foundMusic)
		}
		return nil
	}); err == nil || err.Error() != foundMusic {
		return errors.New("Error: no music found in " + d.Path)
	}

	// check if tracker metadata is present
	if DirectoryExists(filepath.Join(d.Root, d.Path, metadataDir)) {
		d.HasTrackerMetadata = true

		// find origin.json
		originFile := filepath.Join(d.Root, d.Path, metadataDir, originJSONFile)
		if FileExists(originFile) {
			origin := TrackerOriginJSON{Path: originFile}
			if err := origin.load(); err != nil {
				logThis.Error(err, NORMAL)
			} else {
				d.HasOriginJSON = true
				for tracker, o := range origin.Origins {
					if !StringInSlice(tracker, d.Trackers) {
						d.Trackers = append(d.Trackers, tracker)
					}
					// updating
					d.ID[tracker] = o.ID
					d.GroupID[tracker] = o.GroupID
					// getting release.md info
					releaseMD := filepath.Join(d.Root, d.Path, metadataDir, tracker+"_"+summaryFile)
					if !FileExists(releaseMD) {
						// if not present, try the old format
						releaseMD = filepath.Join(d.Root, d.Path, metadataDir, "Release.md")
					}
					if FileExists(releaseMD) {
						bytes, err := ioutil.ReadFile(releaseMD)
						if err != nil {
							logThis.Error(err, NORMAL)
						} else {
							d.ReleaseInfo[tracker] = bytes
							d.HasDescription = true
						}
					}
					// getting release info from json
					infoJSON := filepath.Join(d.Root, d.Path, metadataDir, tracker+"_"+trackerMetadataFile)
					if !FileExists(infoJSON) {
						// if not present, try the old format
						infoJSON = filepath.Join(d.Root, d.Path, metadataDir, "Release.json")
					}
					if FileExists(infoJSON) {
						info := TrackerTorrentInfo{}
						if err := info.Load(infoJSON); err != nil {
							logThis.Error(err, NORMAL)
						} else {
							d.Metadata[tracker] = info
							d.HasInfo = true
						}
					}
				}
			}
		}
	}

	// TODO external way to detect d.State? hidden file? ex: find if .rejected/.exported in root?
	return nil
}

func (d *DownloadFolder) Sort(e *Environment) error {
	// reading metadata
	if err := d.Load(); err != nil {
		return err
	}
	fmt.Println("Sorting " + d.Path)
	// if mpd configured, allow playing the release...
	if e.config.MPD != nil && Accept("Load release into MPD") {
		fmt.Println("Sending to MPD.")
		mpdClient := MPD{}
		if err := mpdClient.Connect(e.config.MPD); err == nil {
			defer mpdClient.DisableAndDisconnect(d.Root, d.Path)
			if err := mpdClient.SendAndPlay(d.Root, d.Path); err != nil {
				fmt.Println(RedBold("Error sending to MPD: " + err.Error()))
			}
		}
	}
	// try to refresh metadata
	if d.HasOriginJSON && Accept("Try to refresh metadata from tracker") {
		for _, t := range d.Trackers {
			tracker, err := e.Tracker(t)
			if err != nil {
				logThis.Error(errors.Wrap(err, "Error getting configuration for tracker "+t), NORMAL)
				continue
			}
			if err := refreshMetadata(e, tracker, []string{strconv.Itoa(d.ID[t])}); err != nil {
				logThis.Error(errors.Wrap(err, "Error refreshing metadata for tracker "+t), NORMAL)
				continue
			}
			// reading metadata again
			if err := d.Load(); err != nil {
				return err
			}
		}
	}

	// offer to display metadata
	if d.HasDescription && Accept("Display known metadata") {
		fmt.Println(d.Description())
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
			fmt.Println(RedBold("This can be reverted by sorting its specific download ID (" + strconv.FormatUint(d.Index, 10) + ")."))
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
				if err := d.export(e.config); err != nil {
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

func (d *DownloadFolder) export(config *Config) error {
	// getting candidates for new folder name
	candidates := []string{d.Path}
	for _, t := range d.Trackers {
		candidates = append(candidates, d.generatePath(t, defaultFolderTemplate))
		candidates = append(candidates, d.generatePath(t, config.Library.FolderTemplate))
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
		if err := CopyDir(filepath.Join(d.Root, d.Path), filepath.Join(config.Library.Directory, newName), config.Library.UseHardLinks); err != nil {
			return errors.Wrap(err, "Error exporting download "+d.Path)
		}
		fmt.Println(Green("This release is now EXPORTED. It will not be removed, but will be ignored in later sorting."))
		d.State = stateExported
	} else {
		fmt.Println("The release was not exported. It can be exported later with the 'downloads export' subcommand.")
	}
	return nil
}

func (d *DownloadFolder) generatePath(tracker, folderTemplate string) string {
	if folderTemplate == "" || !d.HasInfo {
		return d.Path
	}
	info, ok := d.Metadata[tracker]
	if !ok {
		logThis.Info("Could not find metadata for tracker "+tracker, NORMAL)
		return d.Path
	}
	gt := info.FullInfo()
	if gt == nil {
		return d.Path // nothing useful here
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
	idElements := []string{}
	if gt.Response.Torrent.Remastered && gt.Response.Torrent.RemasterYear != originalYear {
		idElements = append(idElements, fmt.Sprintf("%d", gt.Response.Torrent.RemasterYear))
	}
	if editionName != "" {
		idElements = append(idElements, editionName)
	}
	// adding catalog number, or if not specified, the record label
	if gt.Response.Group.CatalogueNumber != "" {
		idElements = append(idElements, gt.Response.Group.CatalogueNumber)
	} else {
		if gt.Response.Torrent.RemasterRecordLabel != "" {
			idElements = append(idElements, gt.Response.Torrent.RemasterRecordLabel)
		} else if gt.Response.Group.RecordLabel != "" {
			idElements = append(idElements, gt.Response.Group.RecordLabel)
		}
	}
	if gt.Response.Group.ReleaseType != 1 {
		// adding release type if not album
		idElements = append(idElements, getGazelleReleaseType(gt.Response.Group.ReleaseType))
	}
	id := strings.Join(idElements, ", ")

	// format
	var format string
	switch gt.Response.Torrent.Encoding {
	case "Lossless":
		format = "FLAC"
	case "24bit Lossless":
		format = "FLAC24"
	case "V0 (VBR)":
		format = "V0"
	case "V2 (VBR)":
		format = "V2"
	case "320":
		format = "320"
	default:
		format = "UnF"
	}

	// source
	source := gt.Response.Torrent.Media
	if source == "CD" && format == "FLAC" {
		if gt.Response.Torrent.HasLog && gt.Response.Torrent.HasCue && (gt.Response.Torrent.LogScore == 100 || gt.Response.Torrent.Grade == "Silver") {
			source += "+"
		}
		if gt.Response.Torrent.Grade == "Gold" {
			source += "+"
		}
	}

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
		"{", "ÆÆ", // otherwise golang's template throws a fit
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
		return d.Path
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
