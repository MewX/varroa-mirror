package main

import (
	"fmt"
	"io/ioutil"
	//"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTMLIndex(t *testing.T) {
	fmt.Println("\n --- Testing HTML Index. ---")
	check := assert.New(t)

	// setting up
	testFile := "test/generated_index.html"
	expectedFile := "test/test_index.html"
	data := HTMLIndex{
		Title: "Varroa Musica",
		Time:  time.Unix(1492953739, 0).UTC().Format("2006-01-02 15:04:05"),
		CSV:   []HTMLLink{{URL: "1.csv", Name: "trk1"}, {URL: "2.csv", Name: "Trk2"}},
		Stats: []HTMLStats{
			{
				Name:  "BLUE",
				Stats: "Up: something | Down: something else",
				GraphLinks: []HTMLLink{
					{Name: "up", URL: "#blue_up"},
					{Name: "down", URL: "#blue_down"},
				},
				Graphs: []HTMLLink{
					{Title: "Red UP", Name: "blue_up", URL: "up.svg"},
					{Title: "Red DOWN", Name: "blue_down", URL: "down.svg"},
				},
			},
			{
				Name:  "PURPLE",
				Stats: "Up: some amount | Down: another amount",
				GraphLinks: []HTMLLink{
					{Name: "up", URL: "#purple_up"},
					{Name: "down", URL: "#purple_down"},
				},
				Graphs: []HTMLLink{
					{Title: "Purple UP", Name: "purple_up", URL: "nwup.svg"},
					{Title: "Purple DOWN", Name: "purple_down", URL: "nwdown.svg"},
				},
			},
		},
	}
//	defer os.Remove(testFile)

	// generating index
	check.Nil(data.ToHTML(testFile))

	// comparing with expected
	expected, err := ioutil.ReadFile(expectedFile)
	check.Nil(err)
	generated, err := ioutil.ReadFile(testFile)
	check.Nil(err)
	check.Equal(expected, generated)
}
