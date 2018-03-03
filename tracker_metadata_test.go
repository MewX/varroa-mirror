package varroa

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratePath(t *testing.T) {
	fmt.Println("+ Testing TrackerMetadata/generatePath...")
	check := assert.New(t)
	// setup logger
	c := &Config{General: &ConfigGeneral{LogLevel: 2}}
	env := &Environment{config: c}
	logThis = NewLogThis(env)

	// test API JSON responses
	gt := &GazelleTorrent{}
	gt.Response.Group.CatalogueNumber = "CATNUM Group"
	gt.Response.Group.MusicInfo.Artists = []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{
		{1,
			"Artist A",
		},
		{2,
			"Artist B",
		},
	}
	gt.Response.Group.Name = "RELEASE 1"
	gt.Response.Group.Year = 1987
	gt.Response.Group.RecordLabel = "LABEL 1 Group"
	gt.Response.Group.ReleaseType = 5 // EP
	gt.Response.Torrent.FilePath = "original_path"
	gt.Response.Torrent.Format = "FLAC"
	gt.Response.Torrent.Encoding = "Lossless"
	gt.Response.Torrent.Media = "WEB"
	gt.Response.Torrent.Remastered = true
	gt.Response.Torrent.RemasterCatalogueNumber = "CATNUM"
	gt.Response.Torrent.RemasterRecordLabel = "LABEL 1"
	gt.Response.Torrent.RemasterTitle = "Deluxe"
	gt.Response.Torrent.RemasterYear = 2017
	gt.Response.Torrent.HasLog = true
	gt.Response.Torrent.HasCue = true
	gt.Response.Torrent.LogScore = 100
	gt.Response.Torrent.FileList = "01 - First.flac{{{26538426}}}|||02 - Second.flac{{{32109249}}}"

	metadataJSONgt1, err := json.MarshalIndent(gt, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Media = "CD"
	metadataJSONgt2, err := json.MarshalIndent(gt, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Format = "MP3"
	gt.Response.Torrent.Encoding = "V0 (VBR)"
	gt.Response.Torrent.RemasterTitle = "Bonus Tracks"
	metadataJSONgt3, err := json.MarshalIndent(gt, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Format = "FLAC"
	gt.Response.Torrent.Encoding = "24bit Lossless"
	gt.Response.Torrent.RemasterTitle = "Remaster"
	gt.Response.Torrent.Media = "Vinyl"
	metadataJSONgt4, err := json.MarshalIndent(gt, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Grade = "Gold"
	gt.Response.Torrent.Media = "CD"
	gt.Response.Torrent.Encoding = "Lossless"
	metadataJSONgt5, err := json.MarshalIndent(gt, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Grade = "Silver"
	gt.Response.Torrent.RemasterYear = 1987
	gt.Response.Torrent.RemasterTitle = "Promo"
	gt.Response.Group.ReleaseType = 1
	metadataJSONgt6, err := json.MarshalIndent(gt, "", "    ")
	check.Nil(err)

	// tracker
	tracker := &GazelleTracker{Name: "BLUE", URL: "http://blue"}

	// torrent infos
	infod2 := &TrackerMetadata{}
	check.Nil(infod2.LoadFromTracker(tracker, metadataJSONgt1))
	infod3 := &TrackerMetadata{}
	check.Nil(infod3.LoadFromTracker(tracker, metadataJSONgt2))
	infod4 := &TrackerMetadata{}
	check.Nil(infod4.LoadFromTracker(tracker, metadataJSONgt3))
	infod5 := &TrackerMetadata{}
	check.Nil(infod5.LoadFromTracker(tracker, metadataJSONgt4))
	infod6 := &TrackerMetadata{}
	check.Nil(infod6.LoadFromTracker(tracker, metadataJSONgt5))
	infod7 := &TrackerMetadata{}
	check.Nil(infod7.LoadFromTracker(tracker, metadataJSONgt6))

	check.Equal("original_path", infod2.GeneratePath(""))
	check.Equal("Artist A, Artist B", infod2.GeneratePath("$a"))
	check.Equal("RELEASE 1", infod2.GeneratePath("$t"))
	check.Equal("1987", infod2.GeneratePath("$y"))
	check.Equal("FLAC", infod2.GeneratePath("$f"))
	check.Equal("V0", infod4.GeneratePath("$f"))
	check.Equal("FLAC24", infod5.GeneratePath("$f"))
	check.Equal("WEB", infod2.GeneratePath("$s"))
	check.Equal("LABEL 1", infod2.GeneratePath("$l"))
	check.Equal("CATNUM", infod2.GeneratePath("$n"))
	check.Equal("DLX", infod2.GeneratePath("$e"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC] [WEB]", infod2.GeneratePath("$a ($y) $t [$f] [$s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC] [WEB] {DLX, LABEL 1-CATNUM}", infod2.GeneratePath("$a ($y) $t [$f] [$s] {$e, $l-$n}"))
	check.Equal("DLXDLX", infod2.GeneratePath("$e/$e")) // sanitized to remove "/"
	check.Equal("2017, DLX, CATNUM, EP", infod2.GeneratePath("$id"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM, EP} [FLAC WEB]", infod2.GeneratePath("$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM, EP} [FLAC CD]", infod3.GeneratePath("$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM, EP} [FLAC CD+]", infod3.GeneratePath("$a ($y) $t {$id} [$f $g]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, Bonus, CATNUM, EP} [V0 CD]", infod4.GeneratePath("$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM, EP} [FLAC24 Vinyl]", infod5.GeneratePath("$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM, EP} [FLAC CD]", infod6.GeneratePath("$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM, EP} [FLAC CD++]", infod6.GeneratePath("$a ($y) $t {$id} [$f $g]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD]", infod7.GeneratePath("$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD+]", infod7.GeneratePath("$a ($y) $t {$id} [$f $g]"))

}
