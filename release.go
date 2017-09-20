package varroa

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/subosito/norma"
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
	TorrentNotification = `%s - %s (%d) [%s/%s/%s/%s]`

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
	GroupID     string
	TorrentFile string
	Size        uint64
	Folder      string
	LogScore    int
	Uploader    string
	Timestamp   time.Time
	Filter      string
	Metadata    ReleaseMetadata
}

func NewRelease(parts []string, alternative bool) (*Release, error) {
	if len(parts) != 19 {
		return nil, errors.New("Incomplete announce information")
	}

	var tags []string
	var url, torrentURL, torrentID string
	pattern := `http[s]?://[[:alnum:]\./:]*torrents\.php\?action=download&id=([\d]*)`
	rg := regexp.MustCompile(pattern)

	if alternative {
		tags = strings.Split(parts[16], ",")
		url = parts[17]
		torrentURL = parts[18]

	} else {
		tags = strings.Split(parts[18], ",")
		url = parts[16]
		torrentURL = parts[17]
	}

	// getting torrentID
	hits := rg.FindAllStringSubmatch(torrentURL, -1)
	if len(hits) != 0 {
		torrentID = hits[0][1]
	}
	// cleaning up tags
	for i, el := range tags {
		tags[i] = strings.TrimSpace(el)
	}

	year, err := strconv.Atoi(parts[3])
	if err != nil {
		year = -1
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

	r := &Release{Timestamp: time.Now(), Artists: artist, Title: parts[2], Year: year, ReleaseType: parts[4], Format: parts[5], Quality: parts[6], Source: parts[13], HasLog: hasLog, LogScore: logScore, HasCue: hasCue, IsScene: isScene, url: url, torrentURL: torrentURL, Tags: tags, TorrentID: torrentID, Metadata: ReleaseMetadata{}}
	r.TorrentFile = fmt.Sprintf(TorrentPath, r.Artists[0], r.Title, r.Year, r.ReleaseType, r.Format, r.Quality, r.Source, r.TorrentID)
	r.TorrentFile = norma.Sanitize(r.TorrentFile)
	return r, nil
}

func (r *Release) String() string {
	return fmt.Sprintf(ReleaseString, strings.Join(r.Artists, ","), r.Title, r.Year, r.ReleaseType, r.Format, r.Quality, r.HasLog, r.LogScore, r.HasCue, r.IsScene, r.Source, r.Tags, r.url, r.torrentURL, r.TorrentID)
}

func (r *Release) ShortString() string {
	short := fmt.Sprintf(TorrentNotification, r.Artists[0], r.Title, r.Year, r.ReleaseType, r.Format, r.Quality, r.Source)
	if r.Size != 0 {
		return short + fmt.Sprintf(" [%s]", humanize.IBytes(r.Size))
	}
	return short
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
	if r.Artists[0] == o.Artists[0] && r.Title == o.Title && r.Year == o.Year && r.ReleaseType == o.ReleaseType && r.Quality == o.Quality && r.Source == o.Source && r.Format == o.Format && r.IsScene == o.IsScene {
		if r.Source == "CD" {
			if r.HasLog == o.HasLog && r.LogScore == o.LogScore && r.HasCue == o.HasCue {
				return true
			}
		} else {
			return true
		}
	}
	return false
}

// IsEqual returns true if both release have the same torrentID && groupID.
func (r *Release) IsEqual(o Release) bool {
	return r.TorrentID == o.TorrentID && r.IsInSameGroup(o)
}

// IsInSameGroup returns true if both release are in the same torrentgroup.
func (r *Release) IsInSameGroup(o Release) bool {
	return r.GroupID == o.GroupID
}

func (r *Release) Satisfies(filter *ConfigFilter) bool {
	// no longer filtering on artists. If a filter has artists defined,
	// varroa will now wait until it gets the TorrentInfo and all of the artists
	// to make a call.
	if len(filter.Year) != 0 && !IntInSlice(r.Year, filter.Year) {
		logThis.Info(filter.Name+": Wrong year", VERBOSE)
		return false
	}
	if len(filter.Format) != 0 && !StringInSlice(r.Format, filter.Format) {
		logThis.Info(filter.Name+": Wrong format", VERBOSE)
		return false
	}
	if len(filter.Source) != 0 && !StringInSlice(r.Source, filter.Source) {
		logThis.Info(filter.Name+": Wrong source", VERBOSE)
		return false
	}
	if len(filter.Quality) != 0 && !StringInSlice(r.Quality, filter.Quality) {
		logThis.Info(filter.Name+": Wrong quality", VERBOSE)
		return false
	}
	if r.Source == "CD" && r.Format == "FLAC" && filter.HasLog && !r.HasLog {
		logThis.Info(filter.Name+": Release has no log", VERBOSE)
		return false
	}
	// only compare logscores if the announce contained that information
	if r.Source == "CD" && r.Format == "FLAC" && filter.LogScore != 0 && (!r.HasLog || (r.LogScore != logScoreNotInAnnounce && filter.LogScore > r.LogScore)) {
		logThis.Info(filter.Name+": Incorrect log score", VERBOSE)
		return false
	}
	if r.Source == "CD" && r.Format == "FLAC" && filter.HasCue && !r.HasCue {
		logThis.Info(filter.Name+": Release has no cue", VERBOSE)
		return false
	}
	if !filter.AllowScene && r.IsScene {
		logThis.Info(filter.Name+": Scene release not allowed", VERBOSE)
		return false
	}
	if len(filter.ExcludedReleaseType) != 0 && StringInSlice(r.ReleaseType, filter.ExcludedReleaseType) {
		logThis.Info(filter.Name+": Excluded release type", VERBOSE)
		return false
	}
	if len(filter.ReleaseType) != 0 && !StringInSlice(r.ReleaseType, filter.ReleaseType) {
		logThis.Info(filter.Name+": Wrong release type", VERBOSE)
		return false
	}
	for _, excluded := range filter.TagsExcluded {
		if StringInSlice(excluded, r.Tags) {
			logThis.Info(filter.Name+": Has excluded tag", VERBOSE)
			return false
		}
	}
	if len(filter.TagsIncluded) != 0 {
		// if none of r.tags in conf.includedTags, return false
		atLeastOneIncludedTag := false
		for _, t := range r.Tags {
			if StringInSlice(t, filter.TagsIncluded) {
				atLeastOneIncludedTag = true
				break
			}
		}
		if !atLeastOneIncludedTag {
			logThis.Info(filter.Name+": Does not have any wanted tag", VERBOSE)
			return false
		}
	}
	// taking the opportunity to retrieve and save some info
	r.Filter = filter.Name
	return true
}

func (r *Release) HasCompatibleTrackerInfo(filter *ConfigFilter, blacklistedUploaders []string, info *TrackerTorrentInfo) bool {
	// checks
	if len(filter.EditionYear) != 0 && !IntInSlice(info.editionYear, filter.EditionYear) {
		logThis.Info(filter.Name+": Wrong edition year", VERBOSE)
		return false
	}
	if filter.MaxSizeMB != 0 && uint64(filter.MaxSizeMB) < (info.size/(1024*1024)) {
		logThis.Info(filter.Name+": Release too big.", VERBOSE)
		return false
	}
	if filter.MinSizeMB > 0 && uint64(filter.MinSizeMB) > (info.size/(1024*1024)) {
		logThis.Info(filter.Name+": Release too small.", VERBOSE)
		return false
	}
	if r.Source == "CD" && r.Format == "FLAC" && r.HasLog && filter.LogScore != 0 && filter.LogScore > info.logScore {
		logThis.Info(filter.Name+": Incorrect log score", VERBOSE)
		return false
	}
	if len(filter.RecordLabel) != 0 && !StringInSlice(info.label, filter.RecordLabel) {
		logThis.Info(filter.Name+": No match for record label", VERBOSE)
		return false
	}
	if r.Artists[0] == "Various Artists" && (len(filter.Artist) != 0 || len(filter.ExcludedArtist) != 0) {
		var foundAtLeastOneArtist bool
		for iArtist := range info.artists {
			if StringInSlice(iArtist, filter.Artist) {
				foundAtLeastOneArtist = true
			}
			if StringInSlice(iArtist, filter.ExcludedArtist) {
				logThis.Info(filter.Name+": Found excluded artist "+iArtist, VERBOSE)
				return false
			}
		}
		if !foundAtLeastOneArtist && len(filter.Artist) != 0 {
			logThis.Info(filter.Name+": No match for artists", VERBOSE)
			return false
		}
	}
	if StringInSlice(info.uploader, blacklistedUploaders) {
		logThis.Info(filter.Name+": Uploader "+info.uploader+" is blacklisted.", VERBOSE)
		return false
	}
	if len(filter.Uploader) != 0 && !StringInSlice(info.uploader, filter.Uploader) {
		logThis.Info(filter.Name+": No match for uploader", VERBOSE)
		return false
	}
	if len(filter.Edition) != 0 {
		found := false
		for _, w := range filter.Edition {
			if strings.Contains(strings.ToLower(info.edition), strings.ToLower(w)) {
				found = true
				break
			}
		}
		if !found {
			logThis.Info(filter.Name+": Edition name does not match any criteria.", VERBOSE)
			return false
		}
	}
	if filter.RejectUnknown && info.catnum == "" && info.label == "" {
		logThis.Info(filter.Name+": Release has neither a record label or catalog number, rejected.", VERBOSE)
		return false
	}
	// taking the opportunity to retrieve and save some info
	r.Size = info.size
	r.LogScore = info.logScore
	r.Uploader = info.uploader
	r.Folder = info.folder
	r.GroupID = strconv.Itoa(info.groupID)
	return true
}
