package varroa

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscogs(t *testing.T) {
	fmt.Println("+ Testing Config...")
	check := assert.New(t)

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
