package varroa

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

const (
	discNumberLabel   = "DISCNUMBER"
	discTotalLabel    = "TRACKTOTAL"
	releaseTitleLabel = "ALBUM"
	yearLabel         = "DATE" // TODO check if only contains year
	trackArtistLabel  = "ARTIST"
	albumArtistLabel  = "ALBUMARTIST"
	genreLabel        = "GENRE"
	trackTitleLabel   = "TITLE"
	trackNumberLabel  = "TRACKNUMBER"
	trackCommentLabel = "DESCRIPTION"
	composerLabel     = "COMPOSER"
	performerLabel    = "PERFORMER"
	recordLabelLabel  = "ORGANIZATION"
)

var (
	tagFields = []string{"Number",
		"TotalTracks",
		"DiscNumber",
		"Artist",
		"AlbumArtist",
		"Title",
		"Description",
		"Year",
		"Genre",
		"Performer",
		"Composer",
		"Album",
		"Label"}
	tagDescriptions = map[string]string{
		"Number":      "Track Number: ",
		"TotalTracks": "Total Tracks: ",
		"DiscNumber":  "Disc Number: ",
		"Artist":      "Track Artist: ",
		"AlbumArtist": "Album Artist: ",
		"Title":       "Title: ",
		"Description": "Description: ",
		"Year":        "Year: ",
		"Genre":       "Genre: ",
		"Performer":   "Performer: ",
		"Composer":    "Composer: ",
		"Album":       "Album: ",
		"Label":       "Label: ",
	}
)

type TrackTags struct {
	Number      string
	TotalTracks string
	DiscNumber  string
	Artist      string
	AlbumArtist string
	Title       string
	Description string
	Year        string
	Genre       string
	Performer   string
	Composer    string
	Album       string
	Label       string
	OtherTags   map[string]string
}

func NewTrackMetadata(tags map[string]string) (*TrackTags, error) {
	// parse all tags
	tm := &TrackTags{}
	tm.OtherTags = make(map[string]string)
	for k, v := range tags {
		if k == trackNumberLabel {
			tm.Number = v
		} else if k == discNumberLabel {
			tm.DiscNumber = v
		} else if k == discTotalLabel {
			tm.TotalTracks = v
		} else if k == releaseTitleLabel {
			tm.Album = v
		} else if k == yearLabel {
			tm.Year = v
		} else if k == trackArtistLabel {
			tm.Artist = v
		} else if k == albumArtistLabel {
			tm.AlbumArtist = v
		} else if k == genreLabel {
			tm.Genre = v
		} else if k == trackTitleLabel {
			tm.Title = v
		} else if k == trackCommentLabel {
			tm.Description = v
		} else if k == composerLabel {
			tm.Composer = v
		} else if k == performerLabel {
			tm.Performer = v
		} else if k == recordLabelLabel {
			tm.Label = v
		} else {
			// other less common tags
			tm.OtherTags[k] = v
		}
	}
	// TODO detect if we have everything (or at least the required tags)
	// TODO else: trumpable! => return err
	return tm, nil
}

func (tm *TrackTags) String() string {
	normalTags := fmt.Sprintf("Disc#: %s| Track#: %s| Artist: %s| Title: %s| AlbumArtist: %s| Album: %s | TotalTracks: %s| Year: %s| Genre: %s| Performer: %s| Composer: %s| Description: %s| Label: %s", tm.DiscNumber, tm.Number, tm.Artist, tm.Title, tm.AlbumArtist, tm.Album, tm.TotalTracks, tm.Year, tm.Genre, tm.Performer, tm.Composer, tm.Description, tm.Label)
	var extraTags string
	for k, v := range tm.OtherTags {
		extraTags += fmt.Sprintf("%s: %s| ", k, v)
	}
	return normalTags + "| Extra tags: " + extraTags
}

func diffString(title, a, b string) bool {
	if a == b {
		logThis.Info(title+a, NORMAL)
		return true
	}
	logThis.Info(title+Green(a)+" / "+Red(b), NORMAL)
	return false
}

func diffField(field string, a, b *TrackTags) bool {
	aField := reflect.ValueOf(a).Elem().FieldByName(field).String()
	bField := reflect.ValueOf(b).Elem().FieldByName(field).String()
	return diffString(tagDescriptions[field], aField, bField)
}

func (tm *TrackTags) diff(o TrackTags) bool {
	isSame := true
	logThis.Info("Comparing A & B:", NORMAL)
	for _, f := range tagFields {
		isSame = isSame && diffField(f, tm, &o)
	}

	// TODO otherTags

	return isSame
}

func (tm *TrackTags) merge(o TrackTags) error {
	logThis.Info("Merging Track metadata:", NORMAL)
	for _, f := range tagFields {
		err := tm.mergeFieldByName(f, tagDescriptions[f], o)
		if err != nil {
			return errors.Wrap(err, "error merging "+f)
		}
	}
	// TODO otherTags
	return nil
}

func (tm *TrackTags) mergeFieldByName(field, title string, o TrackTags) error {
	localValue := reflect.ValueOf(tm).Elem().FieldByName(field).String()
	otherValue := reflect.ValueOf(&o).Elem().FieldByName(field).String()
	options := []string{localValue, otherValue}
	if diffString(title, localValue, otherValue) == false {
		newValue, err := SelectOption("Select correct value or enter one\n", "First option comes from the audio file, second option from Discogs.", options)
		if err != nil {
			return err
		}
		reflect.ValueOf(tm).Elem().FieldByName(field).SetString(newValue)
		return nil
	}
	return nil
}
