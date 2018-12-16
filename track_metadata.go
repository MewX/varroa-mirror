package varroa

import "fmt"

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
)

type TrackMetadata struct {
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
	OtherTags   map[string]string
}

func NewTrackMetadata(tags map[string]string) (*TrackMetadata, error) {
	// TODO parse all tags, return err if something is missing?
	tm := &TrackMetadata{}
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
		} else {
			tm.OtherTags[k] = v
		}
	}
	// TODO detect if we have everything (or at least the required tags)
	// TODO else: trumpable! => return err
	return tm, nil
}

func (tm *TrackMetadata) String() string {
	normalTags := fmt.Sprintf("Disc#: %s| Track#: %s| Artist: %s| Title: %s| AlbumArtist: %s| Album: %s | TotalTracks: %s| Year: %s| Genre: %s| Performer: %s| Composer: %s| Description: %s", tm.DiscNumber, tm.Number, tm.Artist, tm.Title, tm.AlbumArtist, tm.Album, tm.TotalTracks, tm.Year, tm.Genre, tm.Performer, tm.Composer, tm.Description)
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

func (tm *TrackMetadata) diff(o TrackMetadata) bool {
	isSame := true
	logThis.Info("Comparing A & B:", NORMAL)
	isSame = isSame && diffString("Track Number: ", tm.Number, o.Number)

	// TODO tous les champs

	return isSame
}

func (tm *TrackMetadata) merge(o TrackMetadata) error {
	logThis.Info("Merging A & B:", NORMAL)
	if diffString("Track Number: ", tm.Number, o.Number) == false {
		// TODO multiple choice 1. 2. etc + confirm
	}
	return nil
}
