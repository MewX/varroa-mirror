package varroa

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/catastrophic/assistance/logthis"
	"gitlab.com/passelecasque/obstruction/tracker"
)

func TestGeneratePath(t *testing.T) {
	fmt.Println("+ Testing TrackerMetadata/generatePath...")
	check := assert.New(t)

	_, configErr := NewConfig("test/test_complete.yaml")
	check.Nil(configErr)

	// setup logger
	logthis.SetLevel(2)

	// test API JSON responses
	gt := tracker.GazelleTorrent{}
	gt.Group.CatalogueNumber = "CATNUM Group"
	gt.Group.MusicInfo.Artists = []tracker.Artist{
		{1,
			"Artist A",
		},
		{2,
			"Artist B",
		},
	}
	gt.Group.Name = "RELEASE 1"
	gt.Group.Year = 1987
	gt.Group.RecordLabel = "LABEL 1 Group"
	gt.Group.ReleaseType = 5 // EP
	gt.Group.Tags = []string{"tag1", "tag2"}
	gt.Group.WikiImage = "http://cover.jpg"
	gt.Torrent.ID = 123
	gt.Torrent.FilePath = "original_path"
	gt.Torrent.Format = "FLAC"
	gt.Torrent.Encoding = "Lossless"
	gt.Torrent.Media = "WEB"
	gt.Torrent.Remastered = true
	gt.Torrent.RemasterCatalogueNumber = "CATNUM"
	gt.Torrent.RemasterRecordLabel = "LABEL 1"
	gt.Torrent.RemasterTitle = "Deluxe"
	gt.Torrent.RemasterYear = 2017
	gt.Torrent.HasLog = true
	gt.Torrent.HasCue = true
	gt.Torrent.LogScore = 100
	gt.Torrent.FileList = "01 - First.flac{{{26538426}}}|||02 - Second.flac{{{32109249}}}"

	gt2 := gt
	gt2.Torrent.Media = "CD"

	gt3 := gt2
	gt3.Torrent.Format = "MP3"
	gt3.Torrent.Encoding = "V0 (VBR)"
	gt3.Torrent.RemasterTitle = "Bonus Tracks"

	gt4 := gt3
	gt4.Torrent.Format = "FLAC"
	gt4.Torrent.Encoding = "24bit Lossless"
	gt4.Torrent.RemasterTitle = "Remaster"
	gt4.Torrent.Media = "Vinyl"

	gt5 := gt4
	gt5.Torrent.Grade = "Gold"
	gt5.Torrent.Media = "CD"
	gt5.Torrent.Encoding = "Lossless"

	gt6 := gt5
	gt6.Torrent.Grade = "Silver"
	gt6.Torrent.RemasterYear = 1987
	gt6.Torrent.RemasterTitle = "Promo"
	gt6.Group.ReleaseType = 1

	gt7 := gt6
	gt7.Group.Name = "RELEASE 1 / RELEASE 2!!&éçà©§Ð‘®¢"

	gt8 := gt7
	gt8.Group.Name = "\"Thing\""

	// tracker
	gzTracker, err := tracker.NewGazelle("BLUE", "http://blue", "user", "password", "", "", userAgent())
	check.Nil(err)

	// torrent infos
	infod2 := &TrackerMetadata{}
	check.Nil(infod2.Load(gzTracker, &gt))
	infod3 := &TrackerMetadata{}
	check.Nil(infod3.Load(gzTracker, &gt2))
	infod4 := &TrackerMetadata{}
	check.Nil(infod4.Load(gzTracker, &gt3))
	infod5 := &TrackerMetadata{}
	check.Nil(infod5.Load(gzTracker, &gt4))
	infod6 := &TrackerMetadata{}
	check.Nil(infod6.Load(gzTracker, &gt5))
	infod7 := &TrackerMetadata{}
	check.Nil(infod7.Load(gzTracker, &gt6))
	infod8 := &TrackerMetadata{}
	check.Nil(infod8.Load(gzTracker, &gt7))
	infod9 := &TrackerMetadata{}
	check.Nil(infod9.Load(gzTracker, &gt8))

	// checking GeneratePath
	check.Equal("original_path", infod2.GeneratePath("", ""))
	check.Equal("Artist A, Artist B", infod2.GeneratePath("$a", ""))
	check.Equal("RELEASE 1", infod2.GeneratePath("$t", ""))
	check.Equal("1987", infod2.GeneratePath("$y", ""))
	check.Equal("FLAC", infod2.GeneratePath("$f", ""))
	check.Equal("V0", infod4.GeneratePath("$f", ""))
	check.Equal("FLAC24", infod5.GeneratePath("$f", ""))
	check.Equal("WEB", infod2.GeneratePath("$s", ""))
	check.Equal("LABEL 1", infod2.GeneratePath("$l", ""))
	check.Equal("CATNUM", infod2.GeneratePath("$n", ""))
	check.Equal("DLX", infod2.GeneratePath("$e", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC] [WEB]", infod2.GeneratePath("$a ($y) $t [$f] [$s]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC] [WEB] {DLX, LABEL 1-CATNUM}", infod2.GeneratePath("$a ($y) $t [$f] [$s] {$e, $l-$n}", ""))
	check.Equal("DLX/DLX", infod2.GeneratePath("$e/$e", "")) // sanitized to remove "/"
	check.Equal("2017, DLX, CATNUM", infod2.GeneratePath("$id", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM} [FLAC WEB]", infod2.GeneratePath("$a ($y) $t {$id} [$f $s]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM} [FLAC CD]", infod3.GeneratePath("$a ($y) $t {$id} [$f $s]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM} [FLAC CD+]", infod3.GeneratePath("$a ($y) $t {$id} [$f $g]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, Bonus, CATNUM} [V0 CD]", infod4.GeneratePath("$a ($y) $t {$id} [$f $s]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM} [FLAC24 Vinyl]", infod5.GeneratePath("$a ($y) $t {$id} [$f $s]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM} [FLAC CD]", infod6.GeneratePath("$a ($y) $t {$id} [$f $s]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM} [FLAC CD++]", infod6.GeneratePath("$a ($y) $t {$id} [$f $g]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD]", infod7.GeneratePath("$a ($y) $t {$id} [$f $s]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD+]", infod7.GeneratePath("$a ($y) $t {$id} [$f $g]", ""))
	check.Equal("[Artist A, Artist B]/Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD+]", infod7.GeneratePath("[$a]/$a ($y) $t {$id} [$f $g]", ""))
	check.Equal("[Artist A, Artist B]/Artist A, Artist B (1987) RELEASE 1 ∕ RELEASE 2!!&éçà©§Ð‘®¢ {PR, CATNUM} [FLAC CD+]", infod8.GeneratePath("[$a]/$a ($y) $t {$id} [$f $g]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM} EP [FLAC WEB]", infod2.GeneratePath("$a ($y) $t {$id} $xar [$f $s]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM} EP [FLAC WEB]", infod2.GeneratePath("$a ($y) $t {$id} $r [$f $s]", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD]", infod7.GeneratePath("$a ($y) $t {$id} [$f $s] $xar", ""))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD] Album", infod7.GeneratePath("$a ($y) $t {$id} [$f $s] $r", ""))
	check.Equal("Artist A, Artist B (1987) \"Thing\" {PR, CATNUM} [FLAC CD] Album", infod9.GeneratePath("$a ($y) $t {$id} [$f $s] $r", ""))

	// checking TextDescription
	fmt.Println(infod2.TextDescription(false))
	fmt.Println(infod2.TextDescription(true))
}

func TestArtistInSlice(t *testing.T) {
	fmt.Println("+ Testing TrackerMetadata/artistInSlice...")
	check := assert.New(t)

	list := []string{"thing", "VA| other thing", "VA|anoother thing", "VA |nope", "noope | VA"}
	check.True(artistInSlice("thing", "useless", list))
	check.False(artistInSlice("Thing", "useless", list))
	check.True(artistInSlice("Various Artists", "other thing", list))
	check.False(artistInSlice("Single Artist", "other thing", list))
	check.True(artistInSlice("Various Artists", "anoother thing", list))
	check.False(artistInSlice("Single Artist", "anoother thing", list))
	check.False(artistInSlice("Various Artists", "nope", list))
	check.False(artistInSlice("Single Artist", "nope", list))
	check.False(artistInSlice("Various Artists", "noope", list))
	check.False(artistInSlice("Various Artists", "VA | other thing", list))
	check.False(artistInSlice("Various Artists", "VA| other thing", list))
}
