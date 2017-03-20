package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
	artist      []string
	title       string
	year        int
	releaseType string
	format      string
	quality     string
	hasLog      bool
	hasCue      bool
	isScene     bool
	source      string
	tags        []string
	url         string
	torrentURL  string
	torrentID   string
	filename    string
	size        uint64
	folder      string
	logScore    int
	uploader    string
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
	// if the raw artist announce contains & or "performed by", split and add to slice
	subArtists := regexp.MustCompile("&|performed by").Split(parts[1], -1)
	if len(subArtists) != 1 {
		for i, a := range subArtists {
			subArtists[i] = strings.TrimSpace(a)
		}
		artist = append(artist, subArtists...)
	}

	r := &Release{artist: artist, title: parts[2], year: year, releaseType: parts[4], format: parts[5], quality: parts[6], source: parts[13], hasLog: hasLog, logScore: logScore, hasCue: hasCue, isScene: isScene, url: parts[16], torrentURL: parts[17], tags: tags, torrentID: torrentID}
	r.filename = fmt.Sprintf(TorrentPath, r.artist[0], r.title, r.year, r.releaseType, r.format, r.quality, r.source, r.torrentID)
	r.filename = strings.Replace(r.filename, "/", "-", -1)
	return r, nil
}

func (r *Release) String() string {
	return fmt.Sprintf(ReleaseString, strings.Join(r.artist, ","), r.title, r.year, r.releaseType, r.format, r.quality, r.hasLog, r.logScore, r.hasCue, r.isScene, r.source, r.tags, r.url, r.torrentURL, r.torrentID)
}

func (r *Release) ShortString() string {
	return fmt.Sprintf(TorrentNotification, r.artist[0], r.title, r.year, r.releaseType, r.format, r.quality, r.source, humanize.IBytes(r.size))
}

func (r *Release) ToSlice() []string {
	// artist;title;year;size;type;quality;haslog;logscore;hascue;isscene;source;format;tags
	// only saving r.artist[0], the raw artist announce
	return []string{r.artist[0], r.title, strconv.Itoa(r.year), strconv.FormatUint(r.size, 10), r.releaseType, r.quality, strconv.FormatBool(r.hasLog), strconv.Itoa(r.logScore), strconv.FormatBool(r.hasCue), strconv.FormatBool(r.isScene), r.source, r.format, strings.Join(r.tags, ","), r.uploader}
}

func (r *Release) FromSlice(slice []string) error {
	// slice contains timestamp + filter, which are ignored
	if len(slice) != 16 {
		return errors.New("Incorrect entry, cannot load release")
	}
	// no need to parse the raw artist announce again, probably
	r.artist = []string{slice[2]}
	r.title = slice[3]
	year, err := strconv.Atoi(slice[4])
	if err != nil {
		return err
	}
	r.year = year
	size, err := strconv.ParseUint(slice[5], 10, 64)
	if err != nil {
		return err
	}
	r.size = size
	r.releaseType = slice[6]
	r.quality = slice[7]
	hasLog, err := strconv.ParseBool(slice[8])
	if err != nil {
		return err
	}
	r.hasLog = hasLog
	logScore, err := strconv.Atoi(slice[9])
	if err != nil {
		return err
	}
	r.logScore = logScore
	hasCue, err := strconv.ParseBool(slice[10])
	if err != nil {
		return err
	}
	r.hasCue = hasCue
	isScene, err := strconv.ParseBool(slice[11])
	if err != nil {
		return err
	}
	r.isScene = isScene
	r.source = slice[12]
	r.format = slice[13]
	r.tags = strings.Split(slice[14], ",")
	r.uploader = slice[15]
	return nil
}

func (r *Release) IsDupe(o *Release) bool {
	// checking if similar
	// size and tags are not taken into account
	if r.artist[0] == o.artist[0] && r.title == o.title && r.year == o.year && r.releaseType == o.releaseType && r.quality == o.quality && r.source == o.source && r.format == o.format && r.hasLog == o.hasLog && r.logScore == o.logScore && r.hasCue == o.hasCue && r.isScene == o.isScene {
		return true
	}
	return false
}

func (r *Release) Satisfies(filter Filter) bool {
	if len(filter.year) != 0 && !IntInSlice(r.year, filter.year) {
		logThis(filter.label+": Wrong year", VERBOSE)
		return false
	}
	if len(filter.format) != 0 && !StringInSlice(r.format, filter.format) {
		logThis(filter.label+": Wrong format", VERBOSE)
		return false
	}
	if r.artist[0] != "Various Artists" && len(filter.artist) != 0 {
		var foundAtLeastOneArtist bool
		for _, artist := range r.artist {
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
	if len(filter.source) != 0 && !StringInSlice(r.source, filter.source) {
		logThis(filter.label+": Wrong source", VERBOSE)
		return false
	}
	if len(filter.quality) != 0 && !StringInSlice(r.quality, filter.quality) {
		logThis(filter.label+": Wrong quality", VERBOSE)
		return false
	}
	if r.source == "CD" && filter.hasLog && !r.hasLog {
		logThis(filter.label+": Release has no log", VERBOSE)
		return false
	}
	// only compare logscores if the announce contained that information
	if r.source == "CD" && filter.logScore != 0 && r.logScore != logScoreNotInAnnounce && filter.logScore > r.logScore {
		logThis(filter.label+": Incorrect log score", VERBOSE)
		return false
	}
	if r.source == "CD" && filter.hasCue && !r.hasCue {
		logThis(filter.label+": Release has no cue", VERBOSE)
		return false
	}
	if !filter.allowScene && r.isScene {
		logThis(filter.label+": Scene release not allowed", VERBOSE)
		return false
	}
	if len(filter.releaseType) != 0 && !StringInSlice(r.releaseType, filter.releaseType) {
		logThis(filter.label+": Wrong release type", VERBOSE)
		return false
	}
	for _, excluded := range filter.excludedTags {
		if StringInSlice(excluded, r.tags) {
			logThis(filter.label+": Has excluded tag", VERBOSE)
			return false
		}
	}
	if len(filter.includedTags) != 0 {
		// if none of r.tags in conf.includedTags, return false
		atLeastOneIncludedTag := false
		for _, t := range r.tags {
			if StringInSlice(t, filter.includedTags) {
				atLeastOneIncludedTag = true
				break
			}
		}
		if !atLeastOneIncludedTag {
			logThis(filter.label+": Does not have any wanted tag", VERBOSE)
			return false
		}
	}
	return true
}

func (r *Release) HasCompatibleTrackerInfo(filter Filter, blacklistedUploaders []string, info *AdditionalInfo) bool {
	r.size = info.size
	r.logScore = info.logScore
	r.uploader = info.uploader
	if filter.maxSize != 0 && filter.maxSize < (info.size/(1024*1024)) {
		logThis(filter.label+": Release too big.", VERBOSE)
		return false
	}
	if filter.minSize > 0 && filter.minSize > (info.size/(1024*1024)) {
		logThis(filter.label+": Release too small.", VERBOSE)
		return false
	}
	if r.source == "CD" && r.hasLog && filter.logScore != 0 && filter.logScore > info.logScore {
		logThis(filter.label+": Incorrect log score", VERBOSE)
		return false
	}
	if len(filter.recordLabel) != 0 && !StringInSlice(info.label, filter.recordLabel) {
		logThis(filter.label+": No match for record label", VERBOSE)
		return false
	}
	if r.artist[0] == "Various Artists" && len(filter.artist) != 0 {
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
	return true
}
