package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"time"

	"github.com/dustin/go-humanize"
)

const (
	ReleaseString = `Release info:
	Artist: %s
	Title: %s
	Year: %d
	Release Type: %s
	Format: %s
	Quality: %s
	HasLog: %t
	Log Score: %d
	Has Cue: %t
	Scene: %t
	Source: %s
	Tags: %s
	URL: %s
	Torrent URL: %s
	Torrent ID: %s`
	TorrentPath         = `%s - %s (%d) [%s %s %s %s] - %s.torrent`
	TorrentNotification = `%s - %s (%d) [%s/%s/%s/%s] [%s]`

	logScoreNotInAnnounce = -9999
)

type Release struct {
	Artists     []string
	Title       string
	Year        int
	ReleaseType string
	Format      string
	Quality     string
	HasLog      bool
	HasCue      bool
	IsScene     bool
	Source      string
	Tags        []string
	url         string
	torrentURL  string
	TorrentID   string
	TorrentFile string
	Size        uint64
	Folder      string
	LogScore    int
	Uploader    string
	Timestamp   time.Time
	Filter      string
}

func NewRelease(parts []string) (*Release, error) {
	if len(parts) != 19 {
		return nil, errors.New("Incomplete announce information")
	}
	pattern := `http[s]?://[[:alnum:]\./:]*torrents\.php\?action=download&id=([\d]*)`
	rg := regexp.MustCompile(pattern)
	hits := rg.FindAllStringSubmatch(parts[17], -1)
	torrentID := ""
	if len(hits) != 0 {
		torrentID = hits[0][1]
	}
	year, err := strconv.Atoi(parts[3])
	if err != nil {
		year = -1
	}
	tags := strings.Split(parts[18], ",")
	for i, el := range tags {
		tags[i] = strings.TrimSpace(el)
	}
	hasLog := parts[8] != ""
	logScore, err := strconv.Atoi(parts[10])
	if err != nil {
		logScore = logScoreNotInAnnounce
	}
	hasCue := parts[12] != ""
	isScene := parts[15] != ""

	artist := []string{parts[1]}
	// if the raw Artists announce contains & or "performed by", split and add to slice
	subArtists := regexp.MustCompile("&|performed by").Split(parts[1], -1)
	if len(subArtists) != 1 {
		for i, a := range subArtists {
			subArtists[i] = strings.TrimSpace(a)
		}
		artist = append(artist, subArtists...)
	}

	r := &Release{Timestamp: time.Now(), Artists: artist, Title: parts[2], Year: year, ReleaseType: parts[4], Format: parts[5], Quality: parts[6], Source: parts[13], HasLog: hasLog, LogScore: logScore, HasCue: hasCue, IsScene: isScene, url: parts[16], torrentURL: parts[17], Tags: tags, TorrentID: torrentID}
	r.TorrentFile = fmt.Sprintf(TorrentPath, r.Artists[0], r.Title, r.Year, r.ReleaseType, r.Format, r.Quality, r.Source, r.TorrentID)
	r.TorrentFile = strings.Replace(r.TorrentFile, "/", "-", -1)
	return r, nil
}

func (r *Release) String() string {
	return fmt.Sprintf(ReleaseString, strings.Join(r.Artists, ","), r.Title, r.Year, r.ReleaseType, r.Format, r.Quality, r.HasLog, r.LogScore, r.HasCue, r.IsScene, r.Source, r.Tags, r.url, r.torrentURL, r.TorrentID)
}

func (r *Release) ShortString() string {
	return fmt.Sprintf(TorrentNotification, r.Artists[0], r.Title, r.Year, r.ReleaseType, r.Format, r.Quality, r.Source, humanize.IBytes(r.Size))
}

func (r *Release) FromSlice(slice []string) error {
	// Deprecated, only used for migrating old csv files to the new msgpack-based db.

	// slice contains timestamp + filter, which are ignored
	if len(slice) < 16 {
		return errors.New("Incorrect entry, cannot load release")
	}
	// no need to parse the raw Artists announce again, probably
	timestamp, err := strconv.ParseUint(slice[0], 10, 64)
	if err != nil {
		return err
	}
	r.Timestamp = time.Unix(int64(timestamp), 0)
	r.Filter = slice[1]
	r.Artists = []string{slice[2]}
	r.Title = slice[3]
	year, err := strconv.Atoi(slice[4])
	if err != nil {
		return err
	}
	r.Year = year
	size, err := strconv.ParseUint(slice[5], 10, 64)
	if err != nil {
		return err
	}
	r.Size = size
	r.ReleaseType = slice[6]
	r.Quality = slice[7]
	hasLog, err := strconv.ParseBool(slice[8])
	if err != nil {
		return err
	}
	r.HasLog = hasLog
	logScore, err := strconv.Atoi(slice[9])
	if err != nil {
		return err
	}
	r.LogScore = logScore
	hasCue, err := strconv.ParseBool(slice[10])
	if err != nil {
		return err
	}
	r.HasCue = hasCue
	isScene, err := strconv.ParseBool(slice[11])
	if err != nil {
		return err
	}
	r.IsScene = isScene
	r.Source = slice[12]
	r.Format = slice[13]
	r.Tags = strings.Split(slice[14], ",")
	r.Uploader = slice[15]
	return nil
}

func (r *Release) IsDupe(o Release) bool {
	// checking if similar
	// size and tags are not taken into account
	if r.Artists[0] == o.Artists[0] && r.Title == o.Title && r.Year == o.Year && r.ReleaseType == o.ReleaseType && r.Quality == o.Quality && r.Source == o.Source && r.Format == o.Format && r.HasLog == o.HasLog && r.LogScore == o.LogScore && r.HasCue == o.HasCue && r.IsScene == o.IsScene {
		return true
	}
	return false
}

func (r *Release) Satisfies(filter Filter) bool {
	if len(filter.year) != 0 && !IntInSlice(r.Year, filter.year) {
		logThis(filter.label+": Wrong year", VERBOSE)
		return false
	}
	if len(filter.format) != 0 && !StringInSlice(r.Format, filter.format) {
		logThis(filter.label+": Wrong format", VERBOSE)
		return false
	}
	if r.Artists[0] != "Various Artists" && len(filter.artist) != 0 {
		var foundAtLeastOneArtist bool
		for _, artist := range r.Artists {
			if StringInSlice(artist, filter.artist) {
				foundAtLeastOneArtist = true
				break
			}
		}
		if !foundAtLeastOneArtist {
			logThis(filter.label+": Wrong artist", VERBOSE)
			return false
		}
	}
	if len(filter.source) != 0 && !StringInSlice(r.Source, filter.source) {
		logThis(filter.label+": Wrong source", VERBOSE)
		return false
	}
	if len(filter.quality) != 0 && !StringInSlice(r.Quality, filter.quality) {
		logThis(filter.label+": Wrong quality", VERBOSE)
		return false
	}
	if r.Source == "CD" && filter.hasLog && !r.HasLog {
		logThis(filter.label+": Release has no log", VERBOSE)
		return false
	}
	// only compare logscores if the announce contained that information
	if r.Source == "CD" && filter.logScore != 0 && r.LogScore != logScoreNotInAnnounce && filter.logScore > r.LogScore {
		logThis(filter.label+": Incorrect log score", VERBOSE)
		return false
	}
	if r.Source == "CD" && filter.hasCue && !r.HasCue {
		logThis(filter.label+": Release has no cue", VERBOSE)
		return false
	}
	if !filter.allowScene && r.IsScene {
		logThis(filter.label+": Scene release not allowed", VERBOSE)
		return false
	}
	if len(filter.releaseType) != 0 && !StringInSlice(r.ReleaseType, filter.releaseType) {
		logThis(filter.label+": Wrong release type", VERBOSE)
		return false
	}
	for _, excluded := range filter.tags.excluded {
		if StringInSlice(excluded, r.Tags) {
			logThis(filter.label+": Has excluded tag", VERBOSE)
			return false
		}
	}
	if len(filter.tags.included) != 0 {
		// if none of r.tags in conf.includedTags, return false
		atLeastOneIncludedTag := false
		for _, t := range r.Tags {
			if StringInSlice(t, filter.tags.included) {
				atLeastOneIncludedTag = true
				break
			}
		}
		if !atLeastOneIncludedTag {
			logThis(filter.label+": Does not have any wanted tag", VERBOSE)
			return false
		}
	}
	// taking the opportunity to retrieve and save some info
	r.Filter = filter.label
	return true
}

func (r *Release) HasCompatibleTrackerInfo(filter Filter, blacklistedUploaders []string, info *TrackerTorrentInfo) bool {
	// checks
	if filter.size.max != 0 && filter.size.max < (info.size/(1024*1024)) {
		logThis(filter.label+": Release too big.", VERBOSE)
		return false
	}
	if filter.size.min > 0 && filter.size.min > (info.size/(1024*1024)) {
		logThis(filter.label+": Release too small.", VERBOSE)
		return false
	}
	if r.Source == "CD" && r.HasLog && filter.logScore != 0 && filter.logScore > info.logScore {
		logThis(filter.label+": Incorrect log score", VERBOSE)
		return false
	}
	if len(filter.recordLabel) != 0 && !StringInSlice(info.label, filter.recordLabel) {
		logThis(filter.label+": No match for record label", VERBOSE)
		return false
	}
	if r.Artists[0] == "Various Artists" && len(filter.artist) != 0 {
		var foundAtLeastOneArtist bool
		for _, iArtist := range info.artists {
			if StringInSlice(iArtist, filter.artist) {
				foundAtLeastOneArtist = true
			}
		}
		if !foundAtLeastOneArtist {
			logThis(filter.label+": No match for artists", VERBOSE)
			return false
		}
	}
	if StringInSlice(info.uploader, blacklistedUploaders) {
		logThis(filter.label+": Uploader "+info.uploader+" is blacklisted.", VERBOSE)
		return false
	}
	// taking the opportunity to retrieve and save some info
	r.Size = info.size
	r.LogScore = info.logScore
	r.Uploader = info.uploader
	r.Folder = info.folder
	return true
}
