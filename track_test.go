package varroa

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrackFLAC(t *testing.T) {
	fmt.Println("+ Testing Track...")
	check := assert.New(t)
	// setup logger
	c := &Config{General: &ConfigGeneral{LogLevel: 3}}
	env := &Environment{config: c}
	logThis = NewLogThis(env)

	flac := "test/test.flac"
	flacNoPic := "test/test_no_picture.flac"
	flacOut := "test/test_out.flac"
	_, err := exec.LookPath("metaflac")
	if err != nil {
		fmt.Println("Tests cannot be run without metaflac.")
		return
	}

	var track Track
	check.Nil(track.parse(flac))
	check.Equal("16", track.BitDepth)
	check.Equal("48000", track.SampleRate)
	check.Equal("36b714457db55122404bb83b909bb018", track.MD5)
	check.Equal("Composer!", track.Tags["COMPOSER"])
	check.Equal("Gangsta", track.Tags["GENRE"])
	check.Equal("2", track.Tags["DISCNUMBER"])
	check.Equal("05", track.Tags["TRACKNUMBER"])
	check.Equal("09", track.Tags["TRACKTOTAL"])
	check.Equal("FLAC", track.Tags["ENCODED-BY"])
	check.Equal("Album Artist €«ðøßđŋ", track.Tags["ALBUMARTIST"])
	check.Equal("Original artist.", track.Tags["PERFORMER"])
	check.Equal("Mildly interesting comment.", track.Tags["DESCRIPTION"])
	check.Equal("Best artist àß€«đ", track.Tags["ARTIST"])
	check.Equal("Album þþ«ł€", track.Tags["ALBUM"])
	check.Equal("2018", track.Tags["DATE"])
	check.Equal("copyright þæ", track.Tags["COPYRIGHT"])
	check.Equal("http://bestartist.com", track.Tags["CONTACT"])
	check.Equal("A title ê€$éèç\"&!!", track.Tags["TITLE"])
	check.True(track.HasCover)

	fmt.Println(track.String())

	check.False(FileExists(flacOut))
	check.Nil(track.recompress(flacOut))
	defer os.Remove(flacOut)
	check.True(FileExists(flacOut))

	var track2 Track
	check.Nil(track.parse(flacNoPic))
	check.False(track2.HasCover)
}
