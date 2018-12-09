package varroa

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type Track struct {
	Filename    string            `json:"filename"`
	MD5         string            `json:"md5"`
	BitDepth    string            `json:"bit_depth"`
	SampleRate  string            `json:"sample_rate"`
	Fingerprint string            `json:"fingerprint, omitempty"`
	Tags        map[string]string `json:"tags"`
}

func (rt *Track) String() string {
	var tags string
	for k, v := range rt.Tags {
		tags += fmt.Sprintf("\t%s: %s\n", k, v)
	}
	return fmt.Sprintf("%s: FLAC%s %sHz (MD5: %s):\n%s", rt.Filename, rt.BitDepth, rt.SampleRate, rt.MD5, tags)
}

func (rt *Track) parse(filename string) error {
	// TODO check if flac
	_, err := exec.LookPath("metaflac")
	if err != nil {
		return errors.New("'metaflac' is not available on this system, not able to deal with flac files")
	}
	rt.Tags = make(map[string]string)

	rt.Filename = filepath.Base(filename)
	// TODO find something to see if embedded picture
	cmdOut, err := exec.Command("metaflac", "--no-utf8-convert", "--show-bps", "--show-sample-rate", "--show-md5sum", "--export-tags-to=-", filename).Output()
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
			rt.MD5 = line
		} else {
			parts := strings.Split(line, "=")
			rt.Tags[parts[0]] = parts[1]
		}
	}
	return nil
}

func (rt *Track) compareEncoding(o Track) bool {
	return rt.SampleRate == o.SampleRate && rt.BitDepth == o.BitDepth
}
