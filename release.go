package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/dustin/go-humanize"
)

const ReleaseString = `Release info:
	Artist: %s
	Title: %s
	Year: %d
	Release Type: %s
	Format: %s
	Quality: %s
	HasLog: %t
	Has Cue: %t
	Scene: %t
	Source: %s
	Tags: %s
	URL: %s
	Torrent URL: %s
	Torrent ID: %s`
const TorrentPath = `%s - %s (%d) [%s %s %s %s] - %s.torrent`
const TorrentNotification = `%s - %s (%d) [%s/%s/%s/%s] [%s]`

type Release struct {
	artist      string
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
}

func NewTorrent(parts []string) (*Release, error) {
	if len(parts) != 17 {
		return nil, errors.New("Incomplete announce information")
	}
	pattern := `http[s]?://[[:alnum:]\./:]*torrents\.php\?action=download&id=([\d]*)`
	rg := regexp.MustCompile(pattern)
	hits := rg.FindAllStringSubmatch(parts[15], -1)
	torrentID := ""
	if len(hits) != 0 {
		torrentID = hits[0][1]
	}
	year, err := strconv.Atoi(parts[3])
	if err != nil {
		year = -1
	}
	tags := strings.Split(parts[16], ",")
	for i, el := range tags {
		tags[i] = strings.TrimSpace(el)
	}
	hasLog := parts[8] != ""
	hasCue := parts[10] != ""
	isScene := parts[13] != ""

	r := &Release{artist: parts[1], title: parts[2], year: year, releaseType: parts[4], format: parts[5], quality: parts[6], source: parts[11], hasLog: hasLog, hasCue: hasCue, isScene: isScene, url: parts[14], torrentURL: parts[15], tags: tags, torrentID: torrentID}
	r.filename = fmt.Sprintf(TorrentPath, r.artist, r.title, r.year, r.releaseType, r.format, r.quality, r.source, r.torrentID)
	r.filename = strings.Replace(r.filename, "/", "-", -1)
	return r, nil
}

func (r *Release) String() string {
	return fmt.Sprintf(ReleaseString, r.artist, r.title, r.year, r.releaseType, r.format, r.quality, r.hasLog, r.hasCue, r.isScene, r.source, r.tags, r.url, r.torrentURL, r.torrentID)
}

func (r *Release) ShortString() string {
	return fmt.Sprintf(TorrentNotification, r.artist, r.title, r.year, r.releaseType, r.format, r.quality, r.source, humanize.IBytes(r.size))
}

func (r *Release) Download(hc *http.Client) (string, error) {
	if r.torrentURL == "" {
		return "", errors.New("unknown torrent url")
	}
	response, err := hc.Get(r.torrentURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	file, err := os.Create(r.filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	log.Println("++ Downloaded " + r.filename)
	return r.filename, err
}

func (r *Release) Parse() {
	// TODO: remove if not necessary
	mi, err := metainfo.LoadFromFile(r.filename)
	if err != nil {
		log.Println("ERR: " + err.Error())
		return
	}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		log.Println("ERR: " + err.Error())
		return
	}
	r.folder = info.Name
	log.Printf("Torrent folder: %s\n", info.Name)
	totalSize := int64(0)
	for _, f := range info.Files {
		totalSize += f.Length
	}
	log.Printf("Total size: %s\n", humanize.IBytes(uint64(totalSize)))
	r.size = uint64(totalSize)
}

func (r *Release) Satisfies(filter Filter) bool {
	if len(filter.year) != 0 && !IntInSlice(r.year, filter.year) {
		log.Println(filter.label + ": Wrong year")
		return false
	}
	if len(filter.format) != 0 && !StringInSlice(r.format, filter.format) {
		log.Println(filter.label + ": Wrong format")
		return false
	}
	if r.artist != "Various Artists" && len(filter.artist) != 0 && !StringInSlice(r.artist, filter.artist) {
		log.Println(filter.label + ": Wrong artist")
		return false
	}
	if len(filter.source) != 0 && !StringInSlice(r.source, filter.source) {
		log.Println(filter.label + ": Wrong source")
		return false
	}
	if len(filter.quality) != 0 && !StringInSlice(r.quality, filter.quality) {
		log.Println(filter.label + ": Wrong quality")
		return false
	}
	if r.source == "CD" && filter.hasLog && !r.hasLog {
		log.Println(filter.label + ": Release has no log")
		return false
	}
	if r.source == "CD" && filter.hasCue && !r.hasCue {
		log.Println(filter.label + ": Release has no cue")
		return false
	}
	if !filter.allowScene && r.isScene {
		log.Println(filter.label + ": Scene release not allowed")
		return false
	}
	if len(filter.releaseType) != 0 && !StringInSlice(r.releaseType, filter.releaseType) {
		log.Println(filter.label + ": Wrong release type")
		return false
	}
	for _, excluded := range filter.excludedTags {
		if StringInSlice(excluded, r.tags) {
			log.Println(filter.label + ": Has excluded tag")
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
			log.Println(filter.label + ": Does not have any wanted tag")
			return false
		}
	}
	return true
}

func (r *Release) PassesAdditionalChecks(filter Filter, info *AdditionalInfo) bool {
	r.size = info.size
	if filter.maxSize != 0 && filter.maxSize < (info.size/(1024*1024)) {
		log.Println(filter.label + ": Release too big.")
		return false
	}
	if r.source == "CD" && filter.logScore != 0 && filter.logScore != info.logScore {
		log.Println(filter.label + ": Incorrect log score")
		return false
	}
	if len(filter.recordLabel) != 0 && !StringInSlice(info.label, filter.recordLabel) {
		log.Println(filter.label + ": No match for record label")
		return false
	}
	if r.artist == "Various Artists" && len(filter.artist) != 0 {
		var foundAtLeastOneArtist bool
		for _, iArtist := range info.artists {
			if StringInSlice(iArtist, filter.artist) {
				foundAtLeastOneArtist = true
			}
		}
		if !foundAtLeastOneArtist {
			log.Println(filter.label + ": No match for artists")
			return false
		}
	}
	return true
}
