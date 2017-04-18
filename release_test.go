package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRelease(t *testing.T) {
	fmt.Println("+ Testing Release...")
	check := assert.New(t)

	// releases
	r1 := &Release{
		Artists:     []string{"a", "b"},
		Title:       "title",
		Year:        2017,
		ReleaseType: "Album",
		Format:      "FLAC",
		Quality:     "Lossless",
		HasLog:      true,
		HasCue:      true,
		IsScene:     false,
		Source:      "CD",
		Tags:        []string{"tag1", "tag2"},
		url:         "https;//some.thing",
		torrentURL:  "https;//some.thing/id/2",
		TorrentID:   "2",
		GroupID:     "21",
		TorrentFile: "torrent.torrent",
		Size:        123456789,
		Folder:      "a, b (2017) title",
		LogScore:    100,
		Uploader:    "that_guy",
		Timestamp:   time.Now(),
		Filter:      "",
		Metadata:    ReleaseMetadata{},
	}
	r2 := &Release{
		Artists:     []string{"a", "b"},
		Title:       "title",
		Year:        2017,
		ReleaseType: "Album",
		Format:      "FLAC",
		Quality:     "24bit Lossless",
		HasLog:      false,
		HasCue:      false,
		IsScene:     false,
		Source:      "WEB",
		Tags:        []string{"tag1", "tag2"},
		url:         "https;//some.thing",
		torrentURL:  "https;//some.thing/id/2",
		TorrentID:   "22",
		GroupID:     "21",
		TorrentFile: "torrent.torrent",
		Size:        123456789,
		Folder:      "a, b (2017) title",
		LogScore:    100,
		Uploader:    "that_guy",
		Timestamp:   time.Now(),
		Filter:      "",
		Metadata:    ReleaseMetadata{},
	}

	// filters
	f1 := &ConfigFilter{Name: "f1", PerfectFlac: true}
	f2 := &ConfigFilter{Name: "f2", Format: []string{"FLAC"}, Quality: []string{"Lossless"}}
	f3 := &ConfigFilter{Name: "f3", Format: []string{"FLAC"}, Quality: []string{"24bit Lossless"}}
	f4 := &ConfigFilter{Name: "f4", Format: []string{"FLAC"}, Quality: []string{"Lossless", "24bit Lossless"}}

	// tests
	check.True(r1.Satisfies(f1))
	check.True(r2.Satisfies(f1))

	check.True(r1.Satisfies(f2))
	check.False(r2.Satisfies(f2))

	check.False(r1.Satisfies(f3))
	check.True(r2.Satisfies(f3))

	check.True(r1.Satisfies(f4))
	check.True(r2.Satisfies(f4))

}
