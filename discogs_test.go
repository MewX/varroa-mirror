package varroa

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/catastrophic/assistance/logthis"
)

var (
	fakeTrackTag1 = TrackTags{Number: "A",
		TotalTracks: "2",
		DiscNumber:  "",
		Artist:      "Rick Astley",
		AlbumArtist: "Rick Astley",
		Title:       "Never Gonna Give You Up",
		Description: "",
		Year:        "1987",
		Genre:       "Electronic",
		Performer:   "",
		Composer:    "",
		Album:       "Never Gonna Give You Up",
		Label:       "RCA"}
	fakeTrackTag2 = TrackTags{Number: "1",
		TotalTracks: "12",
		DiscNumber:  "1",
		Artist:      "Rick Astley",
		AlbumArtist: "Rick Astley!",
		Title:       "Always Gonna Give You Up",
		Description: "CATNUM",
		Year:        "1988",
		Genre:       "Electronic",
		Performer:   "Rick Astley",
		Composer:    "Rick Astley",
		Album:       "Never Gonna Give You Up!",
		Label:       "RCAaaaaah"}
)

func TestDiscogs(t *testing.T) {
	fmt.Println("+ Testing Discogs...")
	check := assert.New(t)
	// setup logger
	logthis.SetLevel(3)

	// get token from env
	// token can be generated in Discogs user settings
	key := os.Getenv("DISCOGS_TOKEN")
	check.NotEqual(0, len(key), "Cannot get Discogs Token from ENV")

	d, err := NewDiscogsRelease(key)
	check.Nil(err)

	r, err := d.GetRelease(249504)
	check.Nil(err)
	check.Equal("Never Gonna Give You Up", r.Title)

	s, err := json.MarshalIndent(r, "", "    ")
	check.Nil(err)
	fmt.Println(string(s))

	tags := r.TrackTags()
	check.Equal(2, len(tags))
	check.Equal("A", tags[0].Number)
	check.Equal("2", tags[0].TotalTracks)
	check.Equal("", tags[0].DiscNumber)
	check.Equal("Rick Astley", tags[0].Artist)
	check.Equal("Rick Astley", tags[0].AlbumArtist)
	check.Equal("Never Gonna Give You Up", tags[0].Title)
	check.Equal("", tags[0].Description)
	check.Equal("1987", tags[0].Year)
	check.Equal("Electronic", tags[0].Genre)
	check.Equal("", tags[0].Performer)
	check.Equal("", tags[0].Composer)
	check.Equal("Electronic", tags[0].Genre)
	check.Equal("Never Gonna Give You Up", tags[0].Album)
	check.Equal("RCA", tags[0].Label)

	// comparing with TrackTags
	check.True(tags[0].diff(fakeTrackTag1))
	check.Nil(tags[0].merge(fakeTrackTag1))

	check.False(tags[0].diff(fakeTrackTag2))
	//check.Nil(tags[0].merge(fakeTrackTag2))

	res, err := d.Search("Alice Coltrane", "Spiritual Eternal: The Complete Warner Bros. Studio Recordings", 2018, "Real Gone Music", "RGM-0692", "CD", "Album")
	check.Nil(err)
	check.Equal(1, res.Pagination.Items) // got 1 result
	check.Equal(12595789, res.Results[0].ID)

	result := res.Results[0]
	s, err = json.MarshalIndent(result, "", "    ")
	check.Nil(err)
	fmt.Println(string(s))

	// compilation
	r, err = d.GetRelease(8364615)
	check.Nil(err)
	check.Equal("Un Printemps 2016 - Volume 2", r.Title)
	/*
		s, err = json.MarshalIndent(r, "", "    ")
		check.Nil(err)
		fmt.Println(string(s))
	*/
}
