package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/subosito/norma"
)

const (
	stateUnsorted DownloadState = iota // has metadata but is unsorted
	stateAccepted                      // has metadata and has been accepted, but not yet exported to library
	stateExported                      // has metadata and has been exported to library
	stateRejected                      // has metadata and is not to be exported to library
)

var states = []string{"Unsorted", "Accepted", "Exported", "Rejected"}

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

	// TODO? LogFiles           []string // for check-log
}

func (d *DownloadFolder) ShortString() string {
	return d.State.Colorize(fmt.Sprintf("[#%d]\t[%s]\t%s", d.Index, states[d.State][:1], d.Path))
}

func (d *DownloadFolder) String() string {
	return d.State.Colorize(fmt.Sprintf("ID #%d: %s [%s]", d.Index, d.Path, states[d.State]))
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

	// check if tracker metadata is present
	if DirectoryExists(filepath.Join(d.Root, d.Path, metadataDir)) {
		d.HasTrackerMetadata = true

		// find origin.json
		if FileExists(filepath.Join(d.Root, d.Path, metadataDir, originJSONFile)) {
			d.HasOriginJSON = true

			origin := TrackerOriginJSON{Path: filepath.Join(d.Root, d.Path, metadataDir, originJSONFile)}
			if err := origin.load(); err != nil {
				logThis.Error(err, NORMAL)
			} else {
				for tracker, o := range origin.Origins {
					if !StringInSlice(tracker, d.Trackers) {
						d.Trackers = append(d.Trackers, tracker)
					}
					// updating
					d.ID[tracker] = o.ID
					d.GroupID[tracker] = o.GroupID

					// TODO only update if file has changed!!! or if state == unsorted

					// getting release.md info
					releaseMD := filepath.Join(d.Root, d.Path, metadataDir, tracker+"_"+summaryFile)
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
	// TODO scan for log files (using walk, for multi-disc releases)?

	return nil
}

func (d *DownloadFolder) Sort(libraryPath, folderTemplate string, useHardLinks bool) error {
	fmt.Println("Sorting " + d.Path)
	// TODO if mpd configured...
	if Accept("Load release into MPD") {
		//TODO
		fmt.Println("Sending to MPD.")
	}
	fmt.Println(Green("This is where you decide what to do with this release. In any case, it will keep seeding until you remove it yourself or with your bittorrent client."))

	if d.HasInfo && Accept("Display known metadata") {
		fmt.Println(d.Description())
	}

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
				fmt.Println("Exporting files to the library root...")

				newName := d.generatePath(folderTemplate)
				if !Accept("Export as " + newName) {
					newName = d.Path
					// TODO: allow user to edit manually
				}
				if err := CopyDir(filepath.Join(d.Root, d.Path), filepath.Join(libraryPath, newName), useHardLinks); err != nil {
					return errors.Wrap(err, "Error exporting download "+d.Path)
				}
				fmt.Println(Green("This release is now EXPORTED. It will not be removed, but will be ignored in later sorting."))
				d.State = stateExported
			} else {
				fmt.Println("The release was not exported. It can be exported later with the 'downloads export' subcommand.")
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

func (d *DownloadFolder) generatePath(folderTemplate string) string {
	if folderTemplate == "" || !d.HasInfo {
		return d.Path
	}

	// TODO HOW TO DO IT IF MORE THAN 1?
	info := d.Metadata[d.Trackers[0]]
	release := info.Release()

	r := strings.NewReplacer(
		"$a", "{{$a}}",
		"$t", "{{$t}}",
		"$y", "{{$y}}",
		"$q", "{{$q}}",
		"$f", "{{$f}}",
		"$s", "{{$s}}",
		"$l", "{{$l}}",
		"$n", "{{$n}}",
		"$e", "{{$e}}",
	)
	artists := strings.Join(release.Artists, ", ")
	// TODO do better.
	if len(release.Artists) >= 3 {
		artists = "Various Artists"
	}

	// replace with all valid epub parameters
	tmpl := fmt.Sprintf(`{{$a := "%s"}}{{$y := "%d"}}{{$t := "%s"}}{{$q := "%s"}}{{$f := "%s"}}{{$s := "%s"}}{{$l := "%s"}}{{$n := "%s"}}{{$e := "%s"}}%s`,
		artists,
		release.Year,
		release.Title,
		release.Quality,
		release.Format,
		release.Source,
		info.label,
		"CATALOG_NUMBER",
		info.edition,
		r.Replace(folderTemplate))

	var doc bytes.Buffer
	te := template.Must(template.New("hop").Parse(tmpl))
	if err := te.Execute(&doc, nil); err != nil {
		return d.Path
	}
	newName := strings.TrimSpace(doc.String())
	// making sure the path is relative
	if strings.HasPrefix(newName, "/") {
		newName = newName[1:]
	}

	// TODO check it's not more than 250 characters long!

	// making sure the final filename is valid
	return norma.Sanitize(newName)
}
