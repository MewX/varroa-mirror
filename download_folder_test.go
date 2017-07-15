package main

import (
	"fmt"
	"testing"

	"encoding/json"

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
	gt1 := &GazelleTorrent{}
	gt1.Response.Group.CatalogueNumber = "CATNUM"
	gt1.Response.Group.MusicInfo.Artists = []struct {
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
	gt1.Response.Group.Name = "RELEASE 1"
	gt1.Response.Group.Year = 1987
	gt1.Response.Group.RecordLabel = "LABEL 1"
	gt1.Response.Group.ReleaseType = 5 // EP
	gt1.Response.Torrent.Format = "FLAC"
	gt1.Response.Torrent.Encoding = "Lossless"
	gt1.Response.Torrent.Media = "WEB"
	gt1.Response.Torrent.Remastered = true
	gt1.Response.Torrent.RemasterTitle = "Deluxe"
	gt1.Response.Torrent.RemasterYear = 2017
	metadataJSONgt1, err := json.MarshalIndent(gt1.Response, "", "    ")
	check.Nil(err)

	// torrent infos
	info1 := TrackerTorrentInfo{
		groupID:  10,
		label:    "label",
		edition:  "edition",
		logScore: 100,
		artists:  map[string]int{"Artist A": 1, "Artist B": 2},
		folder:   "original_torrent_path",
	}
	info1.fullJSON = metadataJSONgt1

	// test DownloadFolders
	d1 := &DownloadFolder{Path: "original_path", HasInfo: false}
	d1.init()
	d2 := &DownloadFolder{Path: "original_path", HasInfo: true}
	d2.init()
	d2.Trackers = append(d2.Trackers, "BLUE")
	d2.Metadata["BLUE"] = info1

	// checking cases where no new name can be generated
	check.Equal("original_path", d1.generatePath("$a"))
	check.Equal("original_path", d1.generatePath(""))
	check.Equal("original_path", d2.generatePath(""))

	// check other cases
	check.Equal("Artist A, Artist B", d2.generatePath("$a"))
	check.Equal("RELEASE 1", d2.generatePath("$t"))
	check.Equal("1987", d2.generatePath("$y"))
	check.Equal("FLAC", d2.generatePath("$f"))
	check.Equal("Lossless", d2.generatePath("$q"))
	check.Equal("WEB", d2.generatePath("$s"))
	check.Equal("LABEL 1", d2.generatePath("$l"))
	check.Equal("CATNUM", d2.generatePath("$n"))
	check.Equal("DLX", d2.generatePath("$e"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC Lossless] [WEB]", d2.generatePath("$a ($y) $t [$f $q] [$s]"))
	check.Equal("Artist A, Artist B (1987) RELEASE 1 [FLAC Lossless] [WEB] {DLX, LABEL 1-CATNUM}", d2.generatePath("$a ($y) $t [$f $q] [$s] {$e, $l-$n}"))
	check.Equal("DLXDLX", d2.generatePath("$e/$e")) // sanitized to remove "/"

}
