package main

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var announces = []struct {
	announce        string
	expectedHit     bool
	expectedRelease string
}{
	{
		`An artist - Title / \ with utf8 characters éç_?<Ω>§Ð¢<¢<Ð> [2013] [Album] - MP3 / 320 / CD - https://mysterious.address/torrents.php?id=93821 / https://mysterious.address/torrents.php?action=download&id=981243 - tag1.taggy,tag2.mctagface`,
		true,
		"Release info:\n\tArtist: An artist\n\tTitle: Title / \\ with utf8 characters éç_?<Ω>§Ð¢<¢<Ð>\n\tYear: 2013\n\tRelease Type: Album\n\tFormat: MP3\n\tQuality: 320\n\tHasLog: false\n\tHas Cue: false\n\tScene: false\n\tSource: CD\n\tTags: [tag1.taggy tag2.mctagface]\n\tURL: https://mysterious.address/torrents.php?id=93821\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=981243\n\tTorrent ID: 981243",
	},
	{
		`An artist:!, with / another artist! :)ÆΩ¢ - Title / \ with - utf8 characters éç_?<Ω>§Ð¢<¢<Ð> [1999] [EP] - FLAC / 24bit Lossless / Vinyl / Scene - https://mysterious.address/torrents.php?id=93821 / https://mysterious.address/torrents.php?action=download&id=981243 - tag.mctagface`,
		true,
		"Release info:\n\tArtist: An artist:!, with / another artist! :)ÆΩ¢\n\tTitle: Title / \\ with - utf8 characters éç_?<Ω>§Ð¢<¢<Ð>\n\tYear: 1999\n\tRelease Type: EP\n\tFormat: FLAC\n\tQuality: 24bit Lossless\n\tHasLog: false\n\tHas Cue: false\n\tScene: true\n\tSource: Vinyl\n\tTags: [tag.mctagface]\n\tURL: https://mysterious.address/torrents.php?id=93821\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=981243\n\tTorrent ID: 981243",
	},
	{
		`A - B - X [1999] [Live album] - AAC / 256 / WEB - https://mysterious.address/torrents.php?id=93821 / https://mysterious.address/torrents.php?action=download&id=981243 - tag.mctagface`,
		true,
		"Release info:\n\tArtist: A\n\tTitle: B - X\n\tYear: 1999\n\tRelease Type: Live album\n\tFormat: AAC\n\tQuality: 256\n\tHasLog: false\n\tHas Cue: false\n\tScene: false\n\tSource: WEB\n\tTags: [tag.mctagface]\n\tURL: https://mysterious.address/torrents.php?id=93821\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=981243\n\tTorrent ID: 981243",
	},
	{
		"First dude & another one performed by yet another - Title [1992] [Soundtrack] - FLAC / Lossless / Cassette - https://mysterious.address/torrents.php?id=452658 / https://mysterious.address/torrents.php?action=download&id=922578 - classical",
		true,
		"Release info:\n\tArtist: First dude & another one performed by yet another\n\tTitle: Title\n\tYear: 1992\n\tRelease Type: Soundtrack\n\tFormat: FLAC\n\tQuality: Lossless\n\tHasLog: false\n\tHas Cue: false\n\tScene: false\n\tSource: Cassette\n\tTags: [classical]\n\tURL: https://mysterious.address/torrents.php?id=452658\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=922578\n\tTorrent ID: 922578",
	},
	{
		"Various Artists - Something about Blues (Second Edition) [2016] [Compilation] - MP3 / V0 (VBR) / WEB - https://rmysterious.address/torrents.php?id=452491 / https://mysterious.address/torrents.php?action=download&id=922592 - blues",
		true,
		"Release info:\n\tArtist: Various Artists\n\tTitle: Something about Blues (Second Edition)\n\tYear: 2016\n\tRelease Type: Compilation\n\tFormat: MP3\n\tQuality: V0 (VBR)\n\tHasLog: false\n\tHas Cue: false\n\tScene: false\n\tSource: WEB\n\tTags: [blues]\n\tURL: https://rmysterious.address/torrents.php?id=452491\n\tTorrent URL: https://mysterious.address/torrents.php?action=download&id=922592\n\tTorrent ID: 922592",
	},
	{
		"Non-music artist - Ebook Title!  - https://mysterious.address/torrents.php?id=452618 / https://mysterious.address/torrents.php?action=download&id=922495 - science.fiction,medieval.history",
		false,
		"",
	},
}

func TestRegexp(t *testing.T) {
	fmt.Println("+ Testing Announce parsing...")
	verify := assert.New(t)

	for _, announced := range announces {
		r := regexp.MustCompile(announcePattern)
		hits := r.FindAllStringSubmatch(announced.announce, -1)
		if announced.expectedHit {
			verify.NotZero(len(hits))
			release, err := NewRelease(hits[0])
			verify.Nil(err)
			verify.Equal(announced.expectedRelease, release.String())
		} else {
			verify.Zero(len(hits))
		}

	}

}
