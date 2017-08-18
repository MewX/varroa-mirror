package varroa

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDLPath(t *testing.T) {
	fmt.Println("+ Testing download folder/generatePath...")
	check := assert.New(t)
	// setup logger
	c := &Config{General: &ConfigGeneral{LogLevel: 2}}
	env := &Environment{config: c}
	logThis = NewLogThis(env)

	// test API JSON responses
	gt := &GazelleTorrent{}
	gt.Response.Group.CatalogueNumber = "CATNUM"
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
	gt.Response.Group.RecordLabel = "LABEL 1"
	gt.Response.Group.ReleaseType = 5 // EP
	gt.Response.Torrent.Format = "FLAC"
	gt.Response.Torrent.Encoding = "Lossless"
	gt.Response.Torrent.Media = "WEB"
	gt.Response.Torrent.Remastered = true
	gt.Response.Torrent.RemasterTitle = "Deluxe"
	gt.Response.Torrent.RemasterYear = 2017
	gt.Response.Torrent.HasLog = true
	gt.Response.Torrent.HasCue = true
	gt.Response.Torrent.LogScore = 100
	metadataJSONgt1, err := json.MarshalIndent(gt.Response, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Media = "CD"
	metadataJSONgt2, err := json.MarshalIndent(gt.Response, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Format = "MP3"
	gt.Response.Torrent.Encoding = "V0 (VBR)"
	gt.Response.Torrent.RemasterTitle = "Bonus Tracks"
	metadataJSONgt3, err := json.MarshalIndent(gt.Response, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Format = "FLAC"
	gt.Response.Torrent.Encoding = "24bit Lossless"
	gt.Response.Torrent.RemasterTitle = "Remaster"
	gt.Response.Torrent.Media = "Vinyl"
	metadataJSONgt4, err := json.MarshalIndent(gt.Response, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Grade = "Gold"
	gt.Response.Torrent.Media = "CD"
	gt.Response.Torrent.Encoding = "Lossless"
	metadataJSONgt5, err := json.MarshalIndent(gt.Response, "", "    ")
	check.Nil(err)

	gt.Response.Torrent.Grade = "Silver"
	gt.Response.Torrent.RemasterYear = 1987
	gt.Response.Torrent.RemasterTitle = "Promo"
	gt.Response.Group.ReleaseType = 1
	metadataJSONgt6, err := json.MarshalIndent(gt.Response, "", "    ")
	check.Nil(err)

	// torrent infos
	infod2 := TrackerTorrentInfo{fullJSON: metadataJSONgt1}
	infod3 := TrackerTorrentInfo{fullJSON: metadataJSONgt2}
	infod4 := TrackerTorrentInfo{fullJSON: metadataJSONgt3}
	infod5 := TrackerTorrentInfo{fullJSON: metadataJSONgt4}
	infod6 := TrackerTorrentInfo{fullJSON: metadataJSONgt5}
	infod7 := TrackerTorrentInfo{fullJSON: metadataJSONgt6}

	// test DownloadEntrys
	d1 := &DownloadEntry{FolderName: "original_path", HasTrackerMetadata: false}
	d2 := &DownloadEntry{FolderName: "original_path", HasTrackerMetadata: true}
	d2.Tracker = append(d2.Tracker, "BLUE")
	d3 := &DownloadEntry{FolderName: "original_path", HasTrackerMetadata: true}
	d3.Tracker = append(d3.Tracker, "BLUE")
	d4 := &DownloadEntry{FolderName: "original_path", HasTrackerMetadata: true}
	d4.Tracker = append(d4.Tracker, "BLUE")
	d5 := &DownloadEntry{FolderName: "original_path", HasTrackerMetadata: true}
	d5.Tracker = append(d5.Tracker, "BLUE")
	d6 := &DownloadEntry{FolderName: "original_path", HasTrackerMetadata: true}
	d6.Tracker = append(d6.Tracker, "BLUE")
	d7 := &DownloadEntry{FolderName: "original_path", HasTrackerMetadata: true}
	d7.Tracker = append(d7.Tracker, "BLUE")

	// checking cases where no new name can be generated
	check.Equal("original_path", d1.generatePath("BLUE", infod2, "$a"))
	check.Equal("original_path", d1.generatePath("BLUE", infod2, ""))
	check.Equal("original_path", d2.generatePath("BLUE", infod2, ""))

	// check other cases
	check.Equal("Artist A, Artist B", d2.generatePath("BLUE", infod2, "$a"))
	check.Equal("RELEASE 1", d2.generatePath("BLUE", infod2, "$t"))
	check.Equal("1987", d2.generatePath("BLUE", infod2, "$y"))
	check.Equal("FLAC", d2.generatePath("BLUE", infod2, "$f"))
	check.Equal("V0", d4.generatePath("BLUE", infod4, "$f"))
	check.Equal("FLAC24", d5.generatePath("BLUE", infod5, "$f"))
	check.Equal("WEB", d2.generatePath("BLUE", infod2, "$s"))
	check.Equal("LABEL 1", d2.generatePath("BLUE", infod2, "$l"))
	check.Equal("CATNUM", d2.generatePath("BLUE", infod2, "$n"))
	check.Equal("DLX", d2.generatePath("BLUE", infod2, "$e"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC] [WEB]", d2.generatePath("BLUE", infod2, "$a ($y) $t [$f] [$s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC] [WEB] {DLX, LABEL 1-CATNUM}", d2.generatePath("BLUE", infod2, "$a ($y) $t [$f] [$s] {$e, $l-$n}"))
	check.Equal("DLXDLX", d2.generatePath("BLUE", infod2, "$e/$e")) // sanitized to remove "/"
	check.Equal("2017, DLX, CATNUM, EP", d2.generatePath("BLUE", infod2, "$id"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM, EP} [FLAC WEB]", d2.generatePath("BLUE", infod2, "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM, EP} [FLAC CD]", d3.generatePath("BLUE", infod3, "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM, EP} [FLAC CD+]", d3.generatePath("BLUE", infod3, "$a ($y) $t {$id} [$f $g]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, Bonus, CATNUM, EP} [V0 CD]", d4.generatePath("BLUE", infod4, "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM, EP} [FLAC24 Vinyl]", d5.generatePath("BLUE", infod5, "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM, EP} [FLAC CD]", d6.generatePath("BLUE", infod6, "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM, EP} [FLAC CD++]", d6.generatePath("BLUE", infod6, "$a ($y) $t {$id} [$f $g]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD]", d7.generatePath("BLUE", infod7, "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD+]", d7.generatePath("BLUE", infod7, "$a ($y) $t {$id} [$f $g]"))

}
