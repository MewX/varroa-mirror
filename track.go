package varroa

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

type Track struct {
	Filename      string     `json:"filename"`
	MD5           string     `json:"md5"`
	BitDepth      string     `json:"bit_depth"`
	SampleRate    string     `json:"sample_rate"`
	TotalSamples  string     `json:"total_samples"`
	Duration      string     `json:"duration"`
	Fingerprint   string     `json:"fingerprint,omitempty"`
	Tags          *TrackTags `json:"tags"`
	HasCover      bool       `json:"has_cover"`
	PictureSize   string     `json:"picture_size,omitempty"`
	PictureHeight string     `json:"picture_height,omitempty"`
	PictureWidth  string     `json:"picture_width,omitempty"`
	PictureName   string     `json:"picture_name,omitempty"`
}

func (rt *Track) checkExternalBinaries() error {
	_, err := exec.LookPath("flac")
	if err != nil {
		return errors.New("'flac' is not available on this system, not able to deal with flac files")
	}
	_, err = exec.LookPath("metaflac")
	if err != nil {
		return errors.New("'metaflac' is not available on this system, not able to deal with flac files")
	}
	return nil
}

func (rt *Track) String() string {
	var cover string
	if rt.HasCover {
		cover = fmt.Sprintf("Cover: %s (%sx%s, size: %s)", rt.PictureName, rt.PictureWidth, rt.PictureHeight, rt.PictureSize)
	}
	return fmt.Sprintf("%s: FLAC%s %sHz [%ss] (MD5: %s):\n\t%s\n\t%s", rt.Filename, rt.BitDepth, rt.SampleRate, rt.Duration, rt.MD5, rt.Tags.String(), cover)
}

func (rt *Track) parse(filename string) error {
	if err := rt.checkExternalBinaries(); err != nil {
		return err
	}
	if strings.ToLower(filepath.Ext(filename)) != flacExt {
		return errors.New("file is not a FLAC file")
	}

	rt.Filename = filename
	tags := make(map[string]string)

	// getting info & tags
	cmdOut, err := exec.Command("metaflac", "--no-utf8-convert", "--show-bps", "--show-sample-rate", "--show-total-samples", "--show-md5sum", "--export-tags-to=-", filename).Output()
	if err != nil {
		return err
	}
	lines := strings.Split(string(cmdOut), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		if i == 0 {
			rt.BitDepth = line
		} else if i == 1 {
			rt.SampleRate = line
		} else if i == 2 {
			rt.TotalSamples = line
		} else if i == 3 {
			rt.MD5 = line
		} else {
			parts := strings.Split(line, "=")
			tags[parts[0]] = parts[1]
		}
	}
	// parsing tags
	t, err := NewTrackMetadata(tags)
	if err != nil {
		return err
	}
	rt.Tags = t

	// duration = total samples / sample rate
	total, err := strconv.Atoi(rt.TotalSamples)
	if err != nil {
		return err
	}
	rate, err := strconv.Atoi(rt.SampleRate)
	if err != nil {
		return err
	}
	rt.Duration = fmt.Sprintf("%.3f", float32(total)/float32(rate))

	// get embedded picture info
	// TODO what if more than one picture?
	cmdOut, err = exec.Command("metaflac", "--list", "--block-type", "PICTURE", filename).Output()
	if err != nil {
		return err
	}
	output := string(cmdOut)
	if output == "" {
		rt.HasCover = false
	} else {
		rt.HasCover = true
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "length: ") {
				rt.PictureSize = strings.TrimLeft(line, "length: ")
			} else if strings.HasPrefix(line, "width: ") {
				rt.PictureWidth = strings.TrimLeft(line, "width: ")
			} else if strings.HasPrefix(line, "height: ") {
				rt.PictureHeight = strings.TrimLeft(line, "height: ")
			} else if strings.HasPrefix(line, "description: ") {
				rt.PictureName = strings.TrimLeft(line, "description: ")
			}
			if rt.PictureHeight != "" && rt.PictureWidth != "" && rt.PictureSize != "" {
				break
			}
		}

	}

	// TODO image size + padding should be < maxEmbeddedPictureSize
	// TODO if not, warn this could be trumped
	/*sizeInt, err := strconv.Atoi(rt.PictureSize)
	if err != nil {
		logThis.Error(err, VERBOSEST)
	} else {
		if
	}*/

	return nil
}

func (rt *Track) compareEncoding(o Track) bool {
	return rt.SampleRate == o.SampleRate && rt.BitDepth == o.BitDepth
}

func (rt *Track) recompress(dest string) error {
	if err := rt.checkExternalBinaries(); err != nil {
		return err
	}
	// copy file
	if err := CopyFile(rt.Filename, dest, false); err != nil {
		return err
	}
	// recompress
	cmdOut, err := exec.Command("flac", "--no-utf8-convert", "-f", "-8", "-V", dest).CombinedOutput()
	if err != nil {
		return err
	}
	lines := strings.Split(string(cmdOut), "\n")
	status := lines[len(lines)-2]
	logThis.Info("Recompressing "+rt.Filename+": "+status, VERBOSESTEST)

	// TODO save picture somewhere if it exists
	// TODO remove picture + padding

	// remove all padding
	_, err = exec.Command("metaflac", "--no-utf8-convert", "--remove", "--block-type=PADDING", "--dont-use-padding", dest).CombinedOutput()
	if err != nil {
		return err
	}

	// TODO add back the picture or the cover

	// add padding 8k
	_, err = exec.Command("metaflac", "--add-padding=8192", dest).CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}

type trackType int

const (
	normalTrack trackType = iota
	multiDiscTrack
	multiArtistsTrack
	multiArtistsAndDiscTrack
)

func (t *Track) trackType(tm TrackerMetadata) trackType {
	// find if multidisc release
	var multiDisc bool
	discNumberInt, err := strconv.Atoi(t.Tags.DiscNumber)
	if err == nil && (discNumberInt > 1) {
		multiDisc = true
	}
	// TODO else use discogs info?

	// find if multi artists based on release type?
	if StringInSlice(tm.ReleaseType, []string{releaseCompilation, releaseDJMix, releaseMixtape, releaseRemix}) {
		if multiDisc {
			return multiArtistsAndDiscTrack
		}
		return multiArtistsTrack
	} else if multiDisc {
		return multiDiscTrack
	}
	return normalTrack
}

func (t *Track) generateName(filenameTemplate string) (string, error) {
	if t.Filename == "" {
		return "", errors.New("a FLAC file must be parsed first")
	}

	// TODO input: TrackerMetadata, if tags not sufficient?

	discNumber := t.Tags.DiscNumber
	if discNumber == "" {
		// TODO do better...
		discNumber = "01"
	}
	totalTracks := t.Tags.TotalTracks
	if totalTracks == "" {
		// TODO do better...
		totalTracks = "01"
	}
	trackNumber := t.Tags.Number
	if trackNumber == "" {
		// TODO mention it's tag trumpable
		return "", errors.New("could not find track number tag for " + t.Filename)
	}
	trackArtist := t.Tags.Artist
	if trackArtist == "" {
		// TODO mention it's tag trumpable
		return "", errors.New("could not find track artist tag for " + t.Filename)
	}
	trackTitle := t.Tags.Title
	if trackTitle == "" {
		// TODO mention it's tag trumpable
		return "", errors.New("could not find track title tag for " + t.Filename)
	}
	albumTitle := t.Tags.Album
	if albumTitle == "" {
		// TODO mention it's tag trumpable
		return "", errors.New("could not find album title tag for " + t.Filename)
	}
	albumArtist := t.Tags.AlbumArtist
	if albumArtist == "" {
		// TODO do better...
		albumArtist = trackArtist
	}
	trackYear := t.Tags.Year
	if trackYear == "" {
		// TODO do better...
		trackYear = "0000"
	}

	r := strings.NewReplacer(
		"$dn", "{{$dn}}",
		"$dt", "{{$dt}}",
		"$tn", "{{$tn}}",
		"$ta", "{{$ta}}",
		"$tt", "{{$tt}}",
		"$aa", "{{$aa}}",
		"$td", "{{$td}}",
		"$t", "{{$t}}",
		"$y", "{{$y}}",
		"{", "ÆÆ", // otherwise golang's template throws a fit if '{' or '}' are in the user pattern
		"}", "¢¢", // assuming these character sequences will probably not cause conflicts.
	)

	// replace with all valid epub parameters
	tmpl := fmt.Sprintf(`{{$dn := %q}}{{$dt := %q}}{{$tn := %q}}{{$ta := %q}}{{$tt := %q}}{{$aa := %q}}{{$td := %q}}{{$t := %q}}{{$y := %q}}%s`,
		SanitizeFolder(discNumber),
		SanitizeFolder(totalTracks),
		SanitizeFolder(trackNumber),
		SanitizeFolder(trackArtist),
		SanitizeFolder(trackTitle),
		SanitizeFolder(albumArtist),
		SanitizeFolder(t.Duration), // TODO min:sec or hh:mm:ss
		SanitizeFolder(albumTitle),
		SanitizeFolder(trackYear),
		r.Replace(filenameTemplate))

	var doc bytes.Buffer
	te := template.Must(template.New("hop").Parse(tmpl))
	if err := te.Execute(&doc, nil); err != nil {
		return t.Filename, err
	}
	newName := strings.TrimSpace(doc.String())
	// trim spaces around all internal folder names
	var trimmedParts = strings.Split(newName, "/")
	for i, part := range trimmedParts {
		trimmedParts[i] = strings.TrimSpace(part)
	}
	// recover brackets
	r2 := strings.NewReplacer(
		"ÆÆ", "{",
		"¢¢", "}",
	)
	return r2.Replace(strings.Join(trimmedParts, "/")) + flacExt, nil
}

func (t *Track) applyMetadata(tm TrackTags) error {
	// TODO dump all tags and/or keep original version in separate json

	// TODO use metaflac to rewrite all tags (more than one --set-tag="" in one call?)

	return nil
}

func (t *Track) generateSpectrals(root string) error {
	// assumes sox is present, checked before

	// filename
	spectralName := filepath.Join(root, strings.Replace(filepath.Base(t.Filename), filepath.Ext(t.Filename), "", -1)+".spectral")
	// running sox commands
	if !FileExists(spectralName + "full.png") {
		_, err := exec.Command("sox", t.Filename, "-n", "remix", "1", "spectrogram", "-x", "3000", "-y", "513", "-z", "120", "-w", "Kaiser", "-o", spectralName+"full.png").CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "error generating full spectrals for "+t.Filename)
		}
	}
	if !FileExists(spectralName + "zoom.png") {
		_, err := exec.Command("sox", t.Filename, "-n", "remix", "1", "spectrogram", "-x", "500", "-y", "1025", "-z", "120", "-w", "Kaiser", "-S", "1:00", "-d", "0:02", "-o", spectralName+"zoom.png").CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "error generating zoom spectrals for "+t.Filename)
		}
	}
	return nil
}
