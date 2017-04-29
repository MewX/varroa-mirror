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
	// setup logger
	c := &Config{General: &ConfigGeneral{LogLevel: 2}}
	env := &Environment{config: c}
	logThis = LogThis{env: env}

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
	r3 := &Release{
		Artists:     []string{"c"},
		Title:       "title",
		Year:        2016,
		ReleaseType: "EP",
		Format:      "MP3",
		Quality:     "V0 (VBR)",
		HasLog:      false,
		HasCue:      false,
		IsScene:     true,
		Source:      "WEB",
		Tags:        []string{"tag3", "tag4"},
		url:         "https;//some.thing",
		torrentURL:  "https;//some.thing/id/2",
		TorrentID:   "33",
		GroupID:     "22",
		TorrentFile: "torrent.torrent",
		Size:        123456789,
		Folder:      "c (2016) title [WEB] [V0]",
		LogScore:    logScoreNotInAnnounce,
		Uploader:    "that_other_guy",
		Timestamp:   time.Now(),
		Filter:      "",
		Metadata:    ReleaseMetadata{},
	}
	r4 := &Release{
		Artists:     []string{"a"},
		Title:       "title",
		Year:        2016,
		ReleaseType: "Single",
		Format:      "MP3",
		Quality:     "320",
		HasLog:      false,
		HasCue:      false,
		IsScene:     false,
		Source:      "CD",
		Tags:        []string{"tag3"},
		url:         "https;//some.thing",
		torrentURL:  "https;//some.thing/id/2",
		TorrentID:   "34",
		GroupID:     "22",
		TorrentFile: "torrent.torrent",
		Size:        12345678,
		Folder:      "a (2016) title [WEB] [320]",
		LogScore:    logScoreNotInAnnounce,
		Uploader:    "that_other_guy",
		Timestamp:   time.Now(),
		Filter:      "",
		Metadata:    ReleaseMetadata{},
	}
	r5 := &Release{
		Artists:     []string{"a", "b", "a & b"},
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
		TorrentID:   "211",
		GroupID:     "21",
		TorrentFile: "torrent.torrent",
		Size:        123456,
		Folder:      "a & b (2017) title",
		LogScore:    95,
		Uploader:    "that_guy",
		Timestamp:   time.Now(),
		Filter:      "",
		Metadata:    ReleaseMetadata{},
	}

	// filters
	f0 := &ConfigFilter{Name: "f0"}
	f1 := &ConfigFilter{Name: "f1", PerfectFlac: true}
	f2 := &ConfigFilter{Name: "f2", Format: []string{"FLAC"}, Quality: []string{"Lossless"}}
	f3 := &ConfigFilter{Name: "f3", Format: []string{"FLAC"}, Quality: []string{"24bit Lossless"}}
	f4 := &ConfigFilter{Name: "f4", Format: []string{"FLAC"}, Quality: []string{"Lossless", "24bit Lossless"}}
	f5 := &ConfigFilter{Name: "f5", Format: []string{"MP3"}}
	f6 := &ConfigFilter{Name: "f6", Format: []string{"MP3"}, Quality: []string{"320"}}
	f7 := &ConfigFilter{Name: "f7", Format: []string{"MP3"}, Quality: []string{"320", "V0 (VBR)", "24bit Lossless"}}
	f8 := &ConfigFilter{Name: "f8", Format: []string{"MP3"}, AllowScene: true}
	f9 := &ConfigFilter{Name: "f9", Format: []string{"MP3"}, ReleaseType: []string{"Single"}}
	f10 := &ConfigFilter{Name: "f10", Source: []string{"WEB"}, AllowScene: true}
	f11 := &ConfigFilter{Name: "f11", Source: []string{"WEB"}, AllowScene: true, TagsIncluded: []string{"nope", "tag1"}}
	f12 := &ConfigFilter{Name: "f12", AllowScene: true, TagsIncluded: []string{"tag1", "tag3"}, TagsExcluded: []string{"tag4"}}
	f13 := &ConfigFilter{Name: "f13", LogScore: 100}
	f14 := &ConfigFilter{Name: "f14", LogScore: 80}
	f15 := &ConfigFilter{Name: "f15", LogScore: 80, Source: []string{"WEB"}}
	f16 := &ConfigFilter{Name: "f16", LogScore: 80, Source: []string{"CD"}}
	f17 := &ConfigFilter{Name: "f17", LogScore: 100, Source: []string{"CD"}}
	f18 := &ConfigFilter{Name: "f18", Artist: []string{"a"}}
	f19 := &ConfigFilter{Name: "f19", Artist: []string{"a"}, ExcludedArtist: []string{"b"}}
	f20 := &ConfigFilter{Name: "f20", Year: []int{2016}, AllowScene: true}
	f21 := &ConfigFilter{Name: "f21", MaxSizeMB: 100, MinSizeMB: 1, AllowScene: true}

	// checking filters
	check.NotNil(f0.Check())
	check.Nil(f1.Check())
	check.Nil(f2.Check())
	check.Nil(f3.Check())
	check.Nil(f4.Check())
	check.Nil(f5.Check())
	check.Nil(f6.Check())
	check.Nil(f7.Check())
	check.Nil(f8.Check())
	check.Nil(f9.Check())
	check.Nil(f10.Check())
	check.Nil(f11.Check())
	check.Nil(f12.Check())
	check.NotNil(f13.Check())
	check.NotNil(f14.Check())
	check.NotNil(f15.Check())
	check.Nil(f16.Check())
	check.Nil(f17.Check())
	check.Nil(f18.Check())
	check.Nil(f19.Check())
	check.Nil(f20.Check())
	check.Nil(f21.Check())

	// tests
	check.True(r1.Satisfies(f1))
	check.True(r2.Satisfies(f1))
	check.False(r3.Satisfies(f1))
	check.False(r4.Satisfies(f1))
	check.False(r5.Satisfies(f1))

	check.True(r1.Satisfies(f2))
	check.False(r2.Satisfies(f2))
	check.False(r3.Satisfies(f2))
	check.False(r4.Satisfies(f2))
	check.True(r5.Satisfies(f2))

	check.False(r1.Satisfies(f3))
	check.True(r2.Satisfies(f3))
	check.False(r3.Satisfies(f3))
	check.False(r4.Satisfies(f3))
	check.False(r5.Satisfies(f3))

	check.True(r1.Satisfies(f4))
	check.True(r2.Satisfies(f4))
	check.False(r3.Satisfies(f4))
	check.False(r4.Satisfies(f4))
	check.True(r5.Satisfies(f4))

	check.False(r1.Satisfies(f5))
	check.False(r2.Satisfies(f5))
	check.False(r3.Satisfies(f5))
	check.True(r4.Satisfies(f5))
	check.False(r5.Satisfies(f5))

	check.False(r1.Satisfies(f6))
	check.False(r2.Satisfies(f6))
	check.False(r3.Satisfies(f6))
	check.True(r4.Satisfies(f6))
	check.False(r5.Satisfies(f6))

	check.False(r1.Satisfies(f7))
	check.False(r2.Satisfies(f7))
	check.False(r3.Satisfies(f7))
	check.True(r4.Satisfies(f7))
	check.False(r5.Satisfies(f7))

	check.False(r1.Satisfies(f8))
	check.False(r2.Satisfies(f8))
	check.True(r3.Satisfies(f8))
	check.True(r4.Satisfies(f8))
	check.False(r5.Satisfies(f8))

	check.False(r1.Satisfies(f9))
	check.False(r2.Satisfies(f9))
	check.False(r3.Satisfies(f9))
	check.True(r4.Satisfies(f9))
	check.False(r5.Satisfies(f9))

	check.False(r1.Satisfies(f10))
	check.True(r2.Satisfies(f10))
	check.True(r3.Satisfies(f10))
	check.False(r4.Satisfies(f10))
	check.False(r5.Satisfies(f10))

	check.False(r1.Satisfies(f11))
	check.True(r2.Satisfies(f11))
	check.False(r3.Satisfies(f11))
	check.False(r4.Satisfies(f11))
	check.False(r5.Satisfies(f11))

	check.True(r1.Satisfies(f12))
	check.True(r2.Satisfies(f12))
	check.False(r3.Satisfies(f12))
	check.True(r4.Satisfies(f12))
	check.True(r5.Satisfies(f12))

	check.True(r1.Satisfies(f16))
	check.False(r2.Satisfies(f16))
	check.False(r3.Satisfies(f16))
	check.False(r4.Satisfies(f16))
	check.True(r5.Satisfies(f16))

	check.True(r1.Satisfies(f17))
	check.False(r2.Satisfies(f17))
	check.False(r3.Satisfies(f17))
	check.False(r4.Satisfies(f17))
	check.False(r5.Satisfies(f17))

	check.True(r1.Satisfies(f18))
	check.True(r2.Satisfies(f18))
	check.False(r3.Satisfies(f18))
	check.True(r4.Satisfies(f18))
	check.True(r5.Satisfies(f18))

	check.False(r1.Satisfies(f19))
	check.False(r2.Satisfies(f19))
	check.False(r3.Satisfies(f19))
	check.True(r4.Satisfies(f19))
	check.False(r5.Satisfies(f19))

	check.False(r1.Satisfies(f20))
	check.False(r2.Satisfies(f20))
	check.True(r3.Satisfies(f20))
	check.True(r4.Satisfies(f20))
	check.False(r5.Satisfies(f20))

	check.False(r1.Satisfies(f20))
	check.False(r2.Satisfies(f20))
	check.True(r3.Satisfies(f20))
	check.True(r4.Satisfies(f20))
	check.False(r5.Satisfies(f20))
}
