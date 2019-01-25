package varroa

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	// since we no longer filter artists with info from the announce, some of these filters have no effect.
	filter1  = &ConfigFilter{Name: "filter1", Artist: []string{"another one"}}
	filter2  = &ConfigFilter{Name: "filter2", Artist: []string{"Another one"}}
	filter3  = &ConfigFilter{Name: "filter3", Artist: []string{"Aníkúlápó"}}
	filter4  = &ConfigFilter{Name: "filter4", Artist: []string{"An artist"}}
	filter5  = &ConfigFilter{Name: "filter5", Artist: []string{"An artist"}, Format: []string{"FLAC"}}
	filter6  = &ConfigFilter{Name: "filter6", Artist: []string{"An artist"}, Format: []string{"FLAC", "MP3"}}
	filter7  = &ConfigFilter{Name: "filter7", Format: []string{"AAC"}}
	filter8  = &ConfigFilter{Name: "filter8", Source: []string{"CD"}, HasLog: true}
	filter9  = &ConfigFilter{Name: "filter9", Year: []int{1999}}
	filter10 = &ConfigFilter{Name: "filter10", Year: []int{1999}, AllowScene: true}
	filter11 = &ConfigFilter{Name: "filter11", Artist: []string{"Another !ONE", "his friend"}}
	filter12 = &ConfigFilter{Name: "filter12", ReleaseType: []string{"Album", "Anthology"}}
	filter13 = &ConfigFilter{Name: "filter13", Quality: []string{"Lossless", "24bit Lossless"}, AllowScene: true}
	filter14 = &ConfigFilter{Name: "filter14", Source: []string{"Vinyl", "Cassette"}, AllowScene: true}
	filter15 = &ConfigFilter{Name: "filter15", Source: []string{"CD"}, HasLog: true, LogScore: 100}
	filter16 = &ConfigFilter{Name: "filter16", Source: []string{"CD"}, LogScore: 80}
	filter17 = &ConfigFilter{Name: "filter17", Quality: []string{"Lossless"}, AllowScene: true}
	filter18 = &ConfigFilter{Name: "filter18", Year: []int{2017}, Format: []string{"FLAC"}, Source: []string{"WEB"}, Quality: []string{"Lossless"}, AllowScene: true, TagsIncluded: []string{"abstract"}, TagsExcluded: []string{"korean"}}

	allFilters = []*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter7, filter8, filter9, filter10, filter11, filter12, filter13, filter14, filter15, filter16, filter17, filter18}
)

type testAnnounce struct {
	announce               string
	expectedHit            bool
	expectedAlternativeHit bool
	expectedRelease        string
	satisfiedFilters       []*ConfigFilter
}

var announces = []testAnnounce{
	{
		`An artist - Title / \ with utf8 characters éç_?<Ω>§Ð¢<¢<Ð> [2013] [Album] - MP3 / 320 / CD - https://mysterious.address/torrents.php?id=93821 / https://mysterious.address/torrents.php?action=download&id=981243 - tag1.taggy,tag2.mctagface`,
		true,
		false,
		"Release info:\n\tArtist: An artist\n\tTitle: Title / \\ with utf8 characters éç_?<Ω>§Ð¢<¢<Ð>\n\tYear: 2013\n\tRelease Type: Album\n\tFormat: MP3\n\tQuality: 320\n\tHasLog: false\n\tLog Score: -9999\n\tHas Cue: false\n\tScene: false\n\tSource: CD\n\tTags: [tag1.taggy tag2.mctagface]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=981243\n\tTorrent ID: 981243",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter6, filter8, filter11, filter12, filter15, filter16},
	},
	{
		`An artist:!, with / another artist! :)ÆΩ¢ - Title / \ with - utf8 characters éç_?<Ω>§Ð¢<¢<Ð> [1999] [EP] - FLAC / 24bit Lossless / Vinyl / Scene - https://mysterious.address/torrents.php?id=93821 / https://mysterious.address/torrents.php?action=download&id=981243 - tag.mctagface`,
		true,
		false,
		"Release info:\n\tArtist: An artist:!, with / another artist! :)ÆΩ¢\n\tTitle: Title / \\ with - utf8 characters éç_?<Ω>§Ð¢<¢<Ð>\n\tYear: 1999\n\tRelease Type: EP\n\tFormat: FLAC\n\tQuality: 24bit Lossless\n\tHasLog: false\n\tLog Score: -9999\n\tHas Cue: false\n\tScene: true\n\tSource: Vinyl\n\tTags: [tag.mctagface]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=981243\n\tTorrent ID: 981243",
		[]*ConfigFilter{filter10, filter13, filter14},
	},
	{
		`A - B - X [1999] [Live album] - AAC / 256 / WEB - https://mysterious.address/torrents.php?id=93821 / https://mysterious.address/torrents.php?action=download&id=981243 - tag.mctagface`,
		true,
		false,
		"Release info:\n\tArtist: A\n\tTitle: B - X\n\tYear: 1999\n\tRelease Type: Live album\n\tFormat: AAC\n\tQuality: 256\n\tHasLog: false\n\tLog Score: -9999\n\tHas Cue: false\n\tScene: false\n\tSource: WEB\n\tTags: [tag.mctagface]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=981243\n\tTorrent ID: 981243",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter7, filter9, filter10, filter11},
	},
	{
		"First dude & another one performed by yet another & his friend - Title [1992] [Soundtrack] - FLAC / Lossless / Cassette - https://mysterious.address/torrents.php?id=452658 / https://mysterious.address/torrents.php?action=download&id=922578 - classical",
		true,
		false,
		"Release info:\n\tArtist: First dude & another one performed by yet another & his friend,First dude,another one,yet another,his friend\n\tTitle: Title\n\tYear: 1992\n\tRelease Type: Soundtrack\n\tFormat: FLAC\n\tQuality: Lossless\n\tHasLog: false\n\tLog Score: -9999\n\tHas Cue: false\n\tScene: false\n\tSource: Cassette\n\tTags: [classical]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=922578\n\tTorrent ID: 922578",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter11, filter13, filter14, filter17},
	},
	{
		"Various Artists - Something about Blues (Second Edition) [2016] [Compilation] - MP3 / V0 (VBR) / WEB - https://mysterious.address/torrents.php?id=452491 / https://mysterious.address/torrents.php?action=download&id=922592 - blues",
		true,
		false,
		"Release info:\n\tArtist: Various Artists\n\tTitle: Something about Blues (Second Edition)\n\tYear: 2016\n\tRelease Type: Compilation\n\tFormat: MP3\n\tQuality: V0 (VBR)\n\tHasLog: false\n\tLog Score: -9999\n\tHas Cue: false\n\tScene: false\n\tSource: WEB\n\tTags: [blues]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=922592\n\tTorrent ID: 922592",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter6, filter11},
	},
	{
		"Some fellow & Aníkúlápó - first / second [1999] [Anthology] - FLAC / Lossless / Log / Cue / CD - https://mysterious.address/torrents.php?id=271487 / https://mysterious.address/torrents.php?action=download&id=923266 - soul, funk, afrobeat, world.music",
		true,
		false,
		"Release info:\n\tArtist: Some fellow & Aníkúlápó,Some fellow,Aníkúlápó\n\tTitle: first / second\n\tYear: 1999\n\tRelease Type: Anthology\n\tFormat: FLAC\n\tQuality: Lossless\n\tHasLog: true\n\tLog Score: -9999\n\tHas Cue: true\n\tScene: false\n\tSource: CD\n\tTags: [soul funk afrobeat world.music]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=923266\n\tTorrent ID: 923266",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter8, filter9, filter10, filter11, filter12, filter13, filter15, filter16, filter17},
	},
	{
		"Non-music artist - Ebook Title!  - https://mysterious.address/torrents.php?id=452618 / https://mysterious.address/torrents.php?action=download&id=922495 - science.fiction,medieval.history",
		false,
		false,
		"",
		[]*ConfigFilter{},
	},
	{
		"Some fellow & Aníkúlápó - first / second [1999] [Anthology] - FLAC / Lossless / Log / 100% / Cue / CD - https://mysterious.address/torrents.php?id=271487 / https://mysterious.address/torrents.php?action=download&id=923266 - soul, funk, afrobeat, world.music",
		true,
		false,
		"Release info:\n\tArtist: Some fellow & Aníkúlápó,Some fellow,Aníkúlápó\n\tTitle: first / second\n\tYear: 1999\n\tRelease Type: Anthology\n\tFormat: FLAC\n\tQuality: Lossless\n\tHasLog: true\n\tLog Score: 100\n\tHas Cue: true\n\tScene: false\n\tSource: CD\n\tTags: [soul funk afrobeat world.music]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=923266\n\tTorrent ID: 923266",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter8, filter9, filter10, filter11, filter12, filter13, filter15, filter16, filter17},
	},
	{
		"Some fellow & Aníkúlápó - first / second [1999] [Anthology] - FLAC / Lossless / Log / 95% / Cue / CD - https://mysterious.address/torrents.php?id=271487 / https://mysterious.address/torrents.php?action=download&id=923266 - soul, funk, afrobeat, world.music",
		true,
		false,
		"Release info:\n\tArtist: Some fellow & Aníkúlápó,Some fellow,Aníkúlápó\n\tTitle: first / second\n\tYear: 1999\n\tRelease Type: Anthology\n\tFormat: FLAC\n\tQuality: Lossless\n\tHasLog: true\n\tLog Score: 95\n\tHas Cue: true\n\tScene: false\n\tSource: CD\n\tTags: [soul funk afrobeat world.music]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=923266\n\tTorrent ID: 923266",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter8, filter9, filter10, filter11, filter12, filter13, filter16, filter17},
	},
	{
		"Some fellow & Aníkúlápó - first / second [1999] [Anthology] - FLAC / Lossless / Log / -75% / Cue / CD - https://mysterious.address/torrents.php?id=271487 / https://mysterious.address/torrents.php?action=download&id=923266 - soul, funk, afrobeat, world.music",
		true,
		false,
		"Release info:\n\tArtist: Some fellow & Aníkúlápó,Some fellow,Aníkúlápó\n\tTitle: first / second\n\tYear: 1999\n\tRelease Type: Anthology\n\tFormat: FLAC\n\tQuality: Lossless\n\tHasLog: true\n\tLog Score: -75\n\tHas Cue: true\n\tScene: false\n\tSource: CD\n\tTags: [soul funk afrobeat world.music]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=923266\n\tTorrent ID: 923266",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter8, filter9, filter10, filter11, filter12, filter13, filter17},
	},
	{
		"Tobias Tobias - That Thing [2017] [Album] - FLAC / 24bit Lossless / WEB - https://mysterious.address/torrents.php?id=493677 / https://mysterious.address/torrents.php?action=download&id=1030280 - abstract,ambient,drone",
		true,
		false,
		"Release info:\n\tArtist: Tobias Tobias\n\tTitle: That Thing\n\tYear: 2017\n\tRelease Type: Album\n\tFormat: FLAC\n\tQuality: 24bit Lossless\n\tHasLog: false\n\tLog Score: -9999\n\tHas Cue: false\n\tScene: false\n\tSource: WEB\n\tTags: [abstract ambient drone]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=1030280\n\tTorrent ID: 1030280",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter11, filter12, filter13},
	},
	{
		"Tobias Tobias - That Thing [2017] [Album] - FLAC / Lossless / WEB - https://mysterious.address/torrents.php?id=493677 / https://mysterious.address/torrents.php?action=download&id=1030280 - abstract,ambient,drone",
		true,
		false,
		"Release info:\n\tArtist: Tobias Tobias\n\tTitle: That Thing\n\tYear: 2017\n\tRelease Type: Album\n\tFormat: FLAC\n\tQuality: Lossless\n\tHasLog: false\n\tLog Score: -9999\n\tHas Cue: false\n\tScene: false\n\tSource: WEB\n\tTags: [abstract ambient drone]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=1030280\n\tTorrent ID: 1030280",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter11, filter12, filter13, filter17, filter18},
	},
	{
		"Tobias Tobias - That Thing [2017] [Album] - FLAC / Lossless / WEB - https://mysterious.address/torrents.php?id=493677 / https://mysterious.address/torrents.php?action=download&id=1030280 - abstract,ambient,drone,korean",
		true,
		false,
		"Release info:\n\tArtist: Tobias Tobias\n\tTitle: That Thing\n\tYear: 2017\n\tRelease Type: Album\n\tFormat: FLAC\n\tQuality: Lossless\n\tHasLog: false\n\tLog Score: -9999\n\tHas Cue: false\n\tScene: false\n\tSource: WEB\n\tTags: [abstract ambient drone korean]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=1030280\n\tTorrent ID: 1030280",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter11, filter12, filter13, filter17},
	},
	{
		"Tobias Tobias - That Thing [2017] [Album] - FLAC / Lossless / WEB - abstract, ambient, drone, korean - https://mysterious.address/torrents.php?id=493677 / https://mysterious.address/torrents.php?action=download&id=1030280",
		false, // alternative pattern
		true,
		"Release info:\n\tArtist: Tobias Tobias\n\tTitle: That Thing\n\tYear: 2017\n\tRelease Type: Album\n\tFormat: FLAC\n\tQuality: Lossless\n\tHasLog: false\n\tLog Score: -9999\n\tHas Cue: false\n\tScene: false\n\tSource: WEB\n\tTags: [abstract ambient drone korean]\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=1030280\n\tTorrent ID: 1030280",
		[]*ConfigFilter{filter1, filter2, filter3, filter4, filter5, filter6, filter11, filter12, filter13, filter17},
	},
}

func testFilters(announced testAnnounce, hits [][]string, alternate bool, verify *assert.Assertions) {
	verify.NotZero(len(hits))
	release, err := NewRelease("tracker", hits[0], alternate)
	verify.Nil(err)
	verify.Equal(announced.expectedRelease, release.String())
	fmt.Println(release)
	satisfied := 0
	for _, f := range allFilters {
		if release.Satisfies(f) {
			found := false
			for _, ef := range announced.satisfiedFilters {
				if f == ef {
					found = true
					fmt.Println("=> triggers " + f.Name + " (expected)")
					break
				}
			}
			if !found {
				fmt.Println("=> triggers " + f.Name + " (UNexpected!)")
			}
			verify.True(found)
			satisfied++
		}
	}
	verify.Equal(len(announced.satisfiedFilters), satisfied, "Unexpected number of hits for "+announced.announce)
}

func TestRegexp(t *testing.T) {
	fmt.Println("+ Testing Announce parsing & filtering...")
	verify := assert.New(t)

	// testing parser
	for _, announced := range announces {
		r := regexp.MustCompile(announcePattern)
		r2 := regexp.MustCompile(alternativeAnnouncePattern)

		hits := r.FindAllStringSubmatch(announced.announce, -1)
		if announced.expectedHit {
			testFilters(announced, hits, false, verify)
		} else {
			verify.Zero(len(hits))
		}

		hits = r2.FindAllStringSubmatch(announced.announce, -1)
		if announced.expectedAlternativeHit {
			testFilters(announced, hits, true, verify)
		} else {
			verify.Zero(len(hits))
		}
	}

}
