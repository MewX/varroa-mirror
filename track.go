package varroa

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Track struct {
	Filename      string            `json:"filename"`
	MD5           string            `json:"md5"`
	BitDepth      string            `json:"bit_depth"`
	SampleRate    string            `json:"sample_rate"`
	TotalSamples  string            `json:"total_samples"`
	Duration      string            `json:"duration"`
	Fingerprint   string            `json:"fingerprint,omitempty"`
	Tags          map[string]string `json:"tags"`
	HasCover      bool              `json:"has_cover"`
	PictureSize   string            `json:"picture_size,omitempty"`
	PictureHeight string            `json:"picture_height,omitempty"`
	PictureWidth  string            `json:"picture_width,omitempty"`
	PictureName   string            `json:"picture_name,omitempty"`
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
	var tags string
	for k, v := range rt.Tags {
		tags += fmt.Sprintf("\t%s: %s\n", k, v)
	}
	var cover string
	if rt.HasCover {
		cover = fmt.Sprintf("\tCover: %s (%sx%s, size: %s)", rt.PictureName, rt.PictureWidth, rt.PictureHeight, rt.PictureSize)
	}
	return fmt.Sprintf("%s: FLAC%s %sHz [%ss] (MD5: %s):\n%s%s", rt.Filename, rt.BitDepth, rt.SampleRate, rt.Duration, rt.MD5, tags, cover)
}

func (rt *Track) parse(filename string) error {
	if err := rt.checkExternalBinaries(); err != nil {
		return err
	}
	if strings.ToLower(filepath.Ext(filename)) != flacExt {
		return errors.New("file is not a FLAC file")
	}

	rt.Filename = filename
	rt.Tags = make(map[string]string)

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
			rt.Tags[parts[0]] = parts[1]
		}
	}

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
