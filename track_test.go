package varroa

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/catastrophic/assistance/fs"
	"gitlab.com/catastrophic/assistance/logthis"
)

func TestTrackFLAC(t *testing.T) {
	fmt.Println("+ Testing Track...")
	check := assert.New(t)
	// setup logger
	logthis.SetLevel(3)

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
	check.Equal("Composer!", track.Tags.Composer)
	check.Equal("Gangsta", track.Tags.Genre)
	check.Equal("2", track.Tags.DiscNumber)
	check.Equal("05", track.Tags.Number)
	check.Equal("09", track.Tags.TotalTracks)
	check.Equal("Album Artist €«ðøßđŋ", track.Tags.AlbumArtist)
	check.Equal("Original artist.", track.Tags.Performer)
	check.Equal("Mildly interesting comment.", track.Tags.Description)
	check.Equal("Best artist àß€«đ", track.Tags.Artist)
	check.Equal("Album þþ«ł€", track.Tags.Album)
	check.Equal("2018", track.Tags.Year)
	check.Equal("A title ê€$éèç\"&!!", track.Tags.Title)
	check.Equal(3, len(track.Tags.OtherTags))
	check.Equal("FLAC", track.Tags.OtherTags["ENCODED-BY"])
	check.Equal("copyright þæ", track.Tags.OtherTags["COPYRIGHT"])
	check.Equal("http://bestartist.com", track.Tags.OtherTags["CONTACT"])
	check.True(track.HasCover)

	fmt.Println(track.String())

	check.False(fs.FileExists(flacOut))
	check.Nil(track.recompress(flacOut))
	defer os.Remove(flacOut)
	check.True(fs.FileExists(flacOut))

	var track2 Track
	check.Nil(track.parse(flacNoPic))
	check.False(track2.HasCover)

	// testing filename generation
	name, err := track.generateName("$dn | $dt | $tn | $ta | $aa | $tt | $td | $t | $y")
	check.Nil(err)
	check.Equal("02 | 09 | 05 | Best artist àß€«đ | Album Artist €«ðøßđŋ | A title ê€$éèç\"&!! | 1.032 | Album þþ«ł€ | 2018.flac", name)
	name, err = track.generateName("$dn.$tn. $ta - $tt ($td)")
	check.Nil(err)
	check.Equal("02.05. Best artist àß€«đ - A title ê€$éèç\"&!! (1.032).flac", name)
	name, err = track.generateName("$dn.$tn. $tt ($td)")
	check.Nil(err)
	check.Equal("02.05. A title ê€$éèç\"&!! (1.032).flac", name)
	fmt.Println(name)
}
