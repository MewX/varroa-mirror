package varroa

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/catastrophic/assistance/logthis"
)

func TestHTMLIndex(t *testing.T) {
	fmt.Println("\n --- Testing HTML Index. ---")
	check := assert.New(t)

	// setup logger
	logthis.SetLevel(2)

	// setting up
	expectedFile := "test/test_index.html"
	data := HTMLIndex{
		Title: "VARROA MUSICA",
		Time:  time.Unix(1492953739, 0).UTC().Format("2006-01-02 15:04:05"),
		Stats: []HTMLStats{
			{
				Name: "BLUE",
				TrackerStats: [][]string{
					{"Up: something", "Down: something else"},
					{"Up: something", "Down: something else"},
				},
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
				Name: "PURPLE",
				TrackerStats: [][]string{
					{"Up: something!", "Down: something else!"},
					{"Up: something!", "Down: something else!"},
				},
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
		CSS:    knownThemes[darkOrange].CSS(),
		Script: indexJS,
	}

	// generating index
	dataBytes, err := data.IndexStats()
	check.Nil(err)
	check.NotZero(dataBytes)

	check.Nil(data.SetMainContentStats())
	d, err := data.MainPage()
	check.Nil(err)

	// comparing with expected
	expected, err := ioutil.ReadFile(expectedFile)
	check.Nil(err)
	check.Equal(len(expected), len(d))
	check.Equal(expected, d)

	// ioutil.WriteFile("test/generated.html", d, 0666)
}
