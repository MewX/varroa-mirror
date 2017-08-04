package main

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
	logThis = LogThis{env: env}

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
	info1 := TrackerTorrentInfo{fullJSON: metadataJSONgt1}
	info2 := TrackerTorrentInfo{fullJSON: metadataJSONgt2}
	info3 := TrackerTorrentInfo{fullJSON: metadataJSONgt3}
	info4 := TrackerTorrentInfo{fullJSON: metadataJSONgt4}
	info5 := TrackerTorrentInfo{fullJSON: metadataJSONgt5}
	info6 := TrackerTorrentInfo{fullJSON: metadataJSONgt6}

	// test DownloadFolders
	d1 := &DownloadFolder{Path: "original_path", HasInfo: false}
	d1.init()
	d2 := &DownloadFolder{Path: "original_path", HasInfo: true}
	d2.init()
	d2.Trackers = append(d2.Trackers, "BLUE")
	d2.Metadata["BLUE"] = info1
	d3 := &DownloadFolder{Path: "original_path", HasInfo: true}
	d3.init()
	d3.Trackers = append(d3.Trackers, "BLUE")
	d3.Metadata["BLUE"] = info2
	d4 := &DownloadFolder{Path: "original_path", HasInfo: true}
	d4.init()
	d4.Trackers = append(d4.Trackers, "BLUE")
	d4.Metadata["BLUE"] = info3
	d5 := &DownloadFolder{Path: "original_path", HasInfo: true}
	d5.init()
	d5.Trackers = append(d5.Trackers, "BLUE")
	d5.Metadata["BLUE"] = info4
	d6 := &DownloadFolder{Path: "original_path", HasInfo: true}
	d6.init()
	d6.Trackers = append(d6.Trackers, "BLUE")
	d6.Metadata["BLUE"] = info5
	d7 := &DownloadFolder{Path: "original_path", HasInfo: true}
	d7.init()
	d7.Trackers = append(d7.Trackers, "BLUE")
	d7.Metadata["BLUE"] = info6

	// checking cases where no new name can be generated
	check.Equal("original_path", d1.generatePath("BLUE", "$a"))
	check.Equal("original_path", d1.generatePath("BLUE", ""))
	check.Equal("original_path", d2.generatePath("BLUE", ""))
	check.Equal("original_path", d2.generatePath("YELLOW", "$a"))

	// check other cases
	check.Equal("Artist A, Artist B", d2.generatePath("BLUE", "$a"))
	check.Equal("RELEASE 1", d2.generatePath("BLUE", "$t"))
	check.Equal("1987", d2.generatePath("BLUE", "$y"))
	check.Equal("FLAC", d2.generatePath("BLUE", "$f"))
	check.Equal("V0", d4.generatePath("BLUE", "$f"))
	check.Equal("FLAC24", d5.generatePath("BLUE", "$f"))
	check.Equal("WEB", d2.generatePath("BLUE", "$s"))
	check.Equal("LABEL 1", d2.generatePath("BLUE", "$l"))
	check.Equal("CATNUM", d2.generatePath("BLUE", "$n"))
	check.Equal("DLX", d2.generatePath("BLUE", "$e"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC] [WEB]", d2.generatePath("BLUE", "$a ($y) $t [$f] [$s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC] [WEB] {DLX, LABEL 1-CATNUM}", d2.generatePath("BLUE", "$a ($y) $t [$f] [$s] {$e, $l-$n}"))
	check.Equal("DLXDLX", d2.generatePath("BLUE", "$e/$e")) // sanitized to remove "/"
	check.Equal("2017, DLX, CATNUM, EP", d2.generatePath("BLUE", "$id"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM, EP} [FLAC WEB]", d2.generatePath("BLUE", "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM, EP} [FLAC CD]", d3.generatePath("BLUE", "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, DLX, CATNUM, EP} [FLAC CD+]", d3.generatePath("BLUE", "$a ($y) $t {$id} [$f $g]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, Bonus, CATNUM, EP} [V0 CD]", d4.generatePath("BLUE", "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM, EP} [FLAC24 Vinyl]", d5.generatePath("BLUE", "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM, EP} [FLAC CD]", d6.generatePath("BLUE", "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {2017, RM, CATNUM, EP} [FLAC CD++]", d6.generatePath("BLUE", "$a ($y) $t {$id} [$f $g]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD]", d7.generatePath("BLUE", "$a ($y) $t {$id} [$f $s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 {PR, CATNUM} [FLAC CD+]", d7.generatePath("BLUE", "$a ($y) $t {$id} [$f $g]"))

}
