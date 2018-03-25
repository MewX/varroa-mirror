package varroa

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	// releases
	r1 = &Release{
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
		torrentURL:  "https;//some.thing/id/2",
		TorrentID:   "2",
		GroupID:     "21",
		Size:        123456789,
		Folder:      "a, b (2017) title",
		LogScore:    100,
		Timestamp:   time.Now(),
		Filter:      "",
	}
	r2 = &Release{
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
		torrentURL:  "https;//some.thing/id/2",
		TorrentID:   "22",
		GroupID:     "21",
		Size:        123456789,
		Folder:      "a, b (2017) title",
		LogScore:    100,
		Timestamp:   time.Now(),
		Filter:      "",
	}
	r3 = &Release{
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
		torrentURL:  "https;//some.thing/id/2",
		TorrentID:   "33",
		GroupID:     "22",
		Size:        123456789,
		Folder:      "c (2016) title [WEB] [V0]",
		LogScore:    logScoreNotInAnnounce,
		Timestamp:   time.Now(),
		Filter:      "",
	}
	r4 = &Release{
		Artists:     []string{"a", "j"},
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
		torrentURL:  "https;//some.thing/id/2",
		TorrentID:   "34",
		GroupID:     "22",
		Size:        12345678,
		Folder:      "a (2016) title [WEB] [320]",
		LogScore:    logScoreNotInAnnounce,
		Timestamp:   time.Now(),
		Filter:      "",
	}
	r5 = &Release{
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
		torrentURL:  "https;//some.thing/id/2",
		TorrentID:   "211",
		GroupID:     "21",
		Size:        123456,
		Folder:      "a & b (2017) title",
		LogScore:    95,
		Timestamp:   time.Now(),
		Filter:      "",
	}
	// torrent infos
	art1 = TrackerMetadataArtist{Name: "a"}
	art2 = TrackerMetadataArtist{Name: "b"}
	art3 = TrackerMetadataArtist{Name: "c"}

	i1 = &TrackerMetadata{Size: 1234567, LogScore: 100, Uploader: "that_guy", Artists: []TrackerMetadataArtist{art1}}
	i2 = &TrackerMetadata{Size: 1234567, LogScore: 80, Uploader: "someone else", Artists: []TrackerMetadataArtist{art2}}
	i3 = &TrackerMetadata{Size: 11, LogScore: 80, Artists: []TrackerMetadataArtist{art3}}
	i4 = &TrackerMetadata{Size: 123456789, LogScore: 80}
	i5 = &TrackerMetadata{Size: 1234567, LogScore: 100, RecordLabel: "label1", EditionName: "Remastered"}
	i6 = &TrackerMetadata{Size: 1234567, LogScore: 100, RecordLabel: "label unknown"}
	i7 = &TrackerMetadata{Size: 1234567, LogScore: 100, EditionName: "deluxe edition Clean", EditionYear: 2004}
	i8 = &TrackerMetadata{Size: 1234567, LogScore: 100, EditionName: "anniversary remaster CLEAN", EditionYear: 2017}
)

func TestRelease(t *testing.T) {
	fmt.Println("+ Testing Release...")
	check := assert.New(t)
	// setup logger
	c := &Config{General: &ConfigGeneral{LogLevel: 2}}
	env := &Environment{config: c}
	logThis = NewLogThis(env)

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
	f18 := &ConfigFilter{Name: "f18", Artist: []string{"a"}, AllowScene: true}
	f19 := &ConfigFilter{Name: "f19", Artist: []string{"a"}, ExcludedArtist: []string{"b"}, AllowScene: true}
	f20 := &ConfigFilter{Name: "f20", Year: []int{2016}, AllowScene: true}
	f21 := &ConfigFilter{Name: "f21", MaxSizeMB: 100, MinSizeMB: 1, AllowScene: true} // cannot be checked with Satisfies
	f22 := &ConfigFilter{Name: "f22", Source: []string{"CD"}, HasLog: true, HasCue: true, AllowScene: true}
	f23 := &ConfigFilter{Name: "f23", HasLog: true, HasCue: true, AllowScene: true}
	f24 := &ConfigFilter{Name: "f24", ExcludedReleaseType: []string{"Album"}, AllowScene: true}
	f25 := &ConfigFilter{Name: "f25", HasCue: true, HasLog: true, LogScore: 100, Source: []string{"CD"}, ReleaseType: []string{"Album"}, Format: []string{"FLAC"}}
	f26 := &ConfigFilter{Name: "f26", RecordLabel: []string{"label1", "label2"}}
	f27 := &ConfigFilter{Name: "f27", ExcludedArtist: []string{"a", "k"}, AllowScene: true}
	f28 := &ConfigFilter{Name: "f28", PerfectFlac: true, Edition: []string{"r/[dD]eluxe", "Bonus"}}
	f29 := &ConfigFilter{Name: "f29", Uploader: []string{"this_guy", "that_guy"}}
	f30 := &ConfigFilter{Name: "f30", RejectUnknown: true}
	f31 := &ConfigFilter{Name: "f31", EditionYear: []int{2004}, AllowScene: true}
	f32 := &ConfigFilter{Name: "f32", PerfectFlac: true, Edition: []string{"r/[dD]eluxe", "xr/[cC][lL][eE][aA][nN]"}}
	f33 := &ConfigFilter{Name: "f33", BlacklistedUploader: []string{"that_guy"}}
	f34 := &ConfigFilter{Name: "f34", PerfectFlac: true, Edition: []string{"xr/[cC][lL][eE][aA][nN]"}}

	// checking filters
	check.NotNil(f0.check())
	check.Nil(f1.check())
	check.Nil(f2.check())
	check.Nil(f3.check())
	check.Nil(f4.check())
	check.Nil(f5.check())
	check.Nil(f6.check())
	check.Nil(f7.check())
	check.Nil(f8.check())
	check.Nil(f9.check())
	check.Nil(f10.check())
	check.Nil(f11.check())
	check.Nil(f12.check())
	check.NotNil(f13.check())
	check.NotNil(f14.check())
	check.NotNil(f15.check())
	check.Nil(f16.check())
	check.Nil(f17.check())
	check.Nil(f18.check())
	check.Nil(f19.check())
	check.Nil(f20.check())
	check.Nil(f21.check())
	check.Nil(f22.check())
	check.NotNil(f23.check())
	check.Nil(f24.check())
	check.Nil(f25.check())
	check.Nil(f26.check())
	check.Nil(f27.check())
	check.Nil(f28.check())
	check.Nil(f29.check())
	check.Nil(f30.check())
	check.Nil(f31.check())
	check.Nil(f32.check())
	check.Nil(f33.check())
	check.Nil(f34.check())

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
	check.True(r4.Satisfies(f16)) // logscore isn't evaluated since it's not FLAC
	check.True(r5.Satisfies(f16))

	check.True(r1.Satisfies(f17))
	check.False(r2.Satisfies(f17))
	check.False(r3.Satisfies(f17))
	check.True(r4.Satisfies(f17)) // logscore isn't evaluated since it's not FLAC
	check.False(r5.Satisfies(f17))

	check.True(r1.Satisfies(f18))
	check.True(r2.Satisfies(f18))
	check.True(r3.Satisfies(f18)) // false with torrent info only
	check.True(r4.Satisfies(f18))
	check.True(r5.Satisfies(f18))

	check.True(r1.Satisfies(f19)) // artists are checked with torrent info only
	check.True(r2.Satisfies(f19))
	check.True(r3.Satisfies(f19)) // false with torrent info only
	check.True(r4.Satisfies(f19))
	check.True(r5.Satisfies(f19))

	check.False(r1.Satisfies(f20))
	check.False(r2.Satisfies(f20))
	check.True(r3.Satisfies(f20))
	check.True(r4.Satisfies(f20))
	check.False(r5.Satisfies(f20))

	check.True(r1.Satisfies(f21))
	check.True(r2.Satisfies(f21))
	check.True(r3.Satisfies(f21))
	check.True(r4.Satisfies(f21))
	check.True(r5.Satisfies(f21))

	check.True(r1.Satisfies(f22))
	check.False(r2.Satisfies(f22))
	check.False(r3.Satisfies(f22))
	check.True(r4.Satisfies(f22)) // logscore isn't evaluated since it's not FLAC
	check.True(r5.Satisfies(f22))

	check.False(r1.Satisfies(f24))
	check.False(r2.Satisfies(f24))
	check.True(r3.Satisfies(f24))
	check.True(r4.Satisfies(f24))
	check.False(r5.Satisfies(f24))

	check.True(r1.Satisfies(f25))
	check.False(r2.Satisfies(f25))
	check.False(r3.Satisfies(f25))
	check.False(r4.Satisfies(f25))
	check.False(r5.Satisfies(f25))

	check.True(r1.Satisfies(f27)) // artists are checked with torrent info only
	check.True(r2.Satisfies(f27))
	check.True(r3.Satisfies(f27))
	check.True(r4.Satisfies(f27))
	check.True(r5.Satisfies(f27))

	check.True(r1.Satisfies(f30))

	// checking with TorrentInfo

	// artist
	check.True(r1.HasCompatibleTrackerInfo(f18, []string{}, i1))
	check.False(r1.HasCompatibleTrackerInfo(f18, []string{}, i2))
	check.False(r1.HasCompatibleTrackerInfo(f18, []string{}, i3))
	check.True(r1.HasCompatibleTrackerInfo(f19, []string{}, i1))
	check.False(r1.HasCompatibleTrackerInfo(f19, []string{}, i2))
	check.False(r1.HasCompatibleTrackerInfo(f19, []string{}, i3))
	check.False(r1.HasCompatibleTrackerInfo(f27, []string{}, i1))
	check.True(r1.HasCompatibleTrackerInfo(f27, []string{}, i2))
	check.True(r1.HasCompatibleTrackerInfo(f27, []string{}, i3))

	// log score
	check.True(r1.HasCompatibleTrackerInfo(f17, []string{}, i1))
	check.False(r1.HasCompatibleTrackerInfo(f17, []string{}, i2))
	check.True(r1.HasCompatibleTrackerInfo(f14, []string{}, i2))
	check.True(r1.HasCompatibleTrackerInfo(f14, []string{}, i1))
	check.True(r1.HasCompatibleTrackerInfo(f25, []string{}, i1))
	check.False(r1.HasCompatibleTrackerInfo(f25, []string{}, i2))

	// blacklisted users
	check.False(r1.HasCompatibleTrackerInfo(f17, []string{"that_guy", "another_one"}, i1))

	// whitelisted users
	check.False(r1.HasCompatibleTrackerInfo(f29, []string{"that_guy", "another_one"}, i1))
	check.True(r1.HasCompatibleTrackerInfo(f29, []string{"another_one"}, i1))
	check.False(r1.HasCompatibleTrackerInfo(f29, []string{"another_one"}, i2))

	// labels
	check.True(r1.HasCompatibleTrackerInfo(f26, []string{}, i5))
	check.False(r1.HasCompatibleTrackerInfo(f26, []string{}, i6))

	// min/max size out of bounds
	check.False(r5.HasCompatibleTrackerInfo(f21, []string{}, i3))
	check.False(r5.HasCompatibleTrackerInfo(f21, []string{}, i4))

	// edition
	check.False(r1.HasCompatibleTrackerInfo(f28, []string{}, i5))
	check.False(r1.HasCompatibleTrackerInfo(f28, []string{}, i6))
	check.True(r1.HasCompatibleTrackerInfo(f28, []string{}, i7))
	check.False(r1.HasCompatibleTrackerInfo(f28, []string{}, i8))
	check.False(r1.HasCompatibleTrackerInfo(f32, []string{}, i5))
	check.False(r1.HasCompatibleTrackerInfo(f32, []string{}, i6))
	check.False(r1.HasCompatibleTrackerInfo(f32, []string{}, i7))
	check.False(r1.HasCompatibleTrackerInfo(f32, []string{}, i8))
	check.True(r1.HasCompatibleTrackerInfo(f34, []string{}, i5))
	check.True(r1.HasCompatibleTrackerInfo(f34, []string{}, i6))
	check.False(r1.HasCompatibleTrackerInfo(f34, []string{}, i7))
	check.False(r1.HasCompatibleTrackerInfo(f34, []string{}, i8))

	// reject unknown releases
	check.False(r1.HasCompatibleTrackerInfo(f30, []string{}, i1))
	check.True(r1.HasCompatibleTrackerInfo(f30, []string{}, i5))

	// edition year
	check.False(r1.HasCompatibleTrackerInfo(f31, []string{}, i6))
	check.True(r1.HasCompatibleTrackerInfo(f31, []string{}, i7))
	check.False(r1.HasCompatibleTrackerInfo(f31, []string{}, i8))

	// filter-level blacklisted uploaders
	check.False(r1.HasCompatibleTrackerInfo(f33, []string{}, i1))
	check.False(r1.HasCompatibleTrackerInfo(f33, []string{"that_guy"}, i1))
	check.False(r1.HasCompatibleTrackerInfo(f33, []string{"another_one"}, i1))
	check.True(r1.HasCompatibleTrackerInfo(f33, []string{}, i2))

}
