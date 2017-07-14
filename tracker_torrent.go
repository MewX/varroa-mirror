package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

type TrackerTorrentInfo struct {
	id       int
	groupID  int
	label    string
	edition  string
	logScore int
	artists  map[string]int // concat artists, composers, etc: artist name: id
	size     uint64
	uploader string
	folder   string
	coverURL string
	fullJSON []byte
}

func (a *TrackerTorrentInfo) String() string {
	artistNames := make([]string, 0, len(a.artists))
	for k := range a.artists {
		artistNames = append(artistNames, k)
	}
	return fmt.Sprintf("Torrent info | ID %d | GroupID %d | Record label: %s | Log Score: %d | Artists: %s | Size %s", a.id, a.groupID, a.label, a.logScore, strings.Join(artistNames, ","), humanize.IBytes(uint64(a.size)))
}

func (a *TrackerTorrentInfo) DownloadCover(targetWithoutExtension string) error {
	if a.coverURL == "" {
		return errors.New("Unknown image url")
	}
	extension := filepath.Ext(a.coverURL)
	if FileExists(targetWithoutExtension + extension) {
		// already downloaded, or exists in folder already: do nothing
		return nil
	}
	response, err := http.Get(a.coverURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	file, err := os.Create(targetWithoutExtension + extension)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	return err
}

func (a *TrackerTorrentInfo) ArtistIDs() []int {
	artistIDs := make([]int, 0, len(a.artists))
	for _, v := range a.artists {
		artistIDs = append(artistIDs, v)
	}
	return artistIDs
}

func (a *TrackerTorrentInfo) ArtistNames() []string {
	artistNames := make([]string, 0, len(a.artists))
	for k := range a.artists {
		artistNames = append(artistNames, k)
	}
	return artistNames
}

func (a *TrackerTorrentInfo) FullInfo() *GazelleTorrent {
	if len(a.fullJSON) == 0 {
		return nil // nothing useful here
	}
	var gt GazelleTorrent
	if unmarshalErr := json.Unmarshal(a.fullJSON, &gt.Response); unmarshalErr != nil {
		logThis.Error(errors.Wrap(unmarshalErr, "Error parsing torrent info JSON"), NORMAL)
		return nil
	}
	return &gt
}

func (a *TrackerTorrentInfo) Release() *Release {
	gt := a.FullInfo()
	if gt == nil {
		return nil // nothing useful here
	}
	r := &Release{Timestamp: time.Now()}
	// for now, using artists, composers, "with" categories
	for _, el := range gt.Response.Group.MusicInfo.Artists {
		r.Artists = append(r.Artists, el.Name)
	}
	for _, el := range gt.Response.Group.MusicInfo.With {
		r.Artists = append(r.Artists, el.Name)
	}
	for _, el := range gt.Response.Group.MusicInfo.Composers {
		r.Artists = append(r.Artists, el.Name)
	}
	r.Title = gt.Response.Group.Name
	if gt.Response.Torrent.Remastered {
		r.Year = gt.Response.Torrent.RemasterYear
	} else {
		r.Year = gt.Response.Group.Year
	}
	r.ReleaseType = getGazelleReleaseType(gt.Response.Group.ReleaseType)
	r.Format = gt.Response.Torrent.Format
	r.Quality = gt.Response.Torrent.Encoding
	r.HasLog = gt.Response.Torrent.HasLog
	r.HasCue = gt.Response.Torrent.HasCue
	r.IsScene = gt.Response.Torrent.Scene
	r.Source = gt.Response.Torrent.Media
	r.Tags = gt.Response.Group.Tags
	// r.url =
	// r.torrentURL =
	r.TorrentID = fmt.Sprintf("%d", gt.Response.Torrent.ID)
	r.GroupID = fmt.Sprintf("%d", gt.Response.Group.ID)
	// r.TorrentFile =
	r.Size = uint64(gt.Response.Torrent.Size)
	r.Folder = gt.Response.Torrent.FilePath
	r.LogScore = gt.Response.Torrent.LogScore
	r.Uploader = gt.Response.Torrent.Username
	r.Metadata = ReleaseMetadata{}
	return r
}

// LoadFromBytes and fill the relevant fields.
func (a *TrackerTorrentInfo) LoadFromBytes(data []byte, fullJSON bool) error {
	var gt GazelleTorrent
	var unmarshalErr error
	if fullJSON {
		unmarshalErr = json.Unmarshal(data, &gt)
	} else {
		unmarshalErr = json.Unmarshal(data, &gt.Response)
	}
	if unmarshalErr != nil {
		logThis.Error(errors.Wrap(unmarshalErr, "Error parsing torrent info JSON"), NORMAL)
		return nil
	}

	a.id = gt.Response.Torrent.ID
	a.groupID = gt.Response.Group.ID
	a.artists = map[string]int{}
	// for now, using artists, composers, "with" categories
	for _, el := range gt.Response.Group.MusicInfo.Artists {
		a.artists[el.Name] = el.ID
	}
	for _, el := range gt.Response.Group.MusicInfo.With {
		a.artists[el.Name] = el.ID
	}
	for _, el := range gt.Response.Group.MusicInfo.Composers {
		a.artists[el.Name] = el.ID
	}
	a.label = gt.Response.Group.RecordLabel
	if gt.Response.Torrent.Remastered {
		a.label = gt.Response.Torrent.RemasterRecordLabel
	}
	a.edition = gt.Response.Torrent.RemasterTitle
	a.logScore = gt.Response.Torrent.LogScore
	a.size = uint64(gt.Response.Torrent.Size)
	a.coverURL = gt.Response.Group.WikiImage
	a.folder = gt.Response.Torrent.FilePath

	// keeping a copy of uploader before anonymizing
	a.uploader = gt.Response.Torrent.Username
	// json for metadata, anonymized
	gt.Response.Torrent.Username = ""
	gt.Response.Torrent.UserID = 0

	// keeping a copy of the full JSON
	metadataJSON, err := json.MarshalIndent(gt.Response, "", "    ")
	if err != nil {
		metadataJSON = data // falling back to complete json
	}
	a.fullJSON = metadataJSON
	return nil
}

// Load from a previously saved JSON file.
func (a *TrackerTorrentInfo) Load(path string) error {
	if !FileExists(path) {
		return errors.New("Error loading file " + path + ", which could not be found")
	}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "Error loading JSON file "+path)
	}
	return a.LoadFromBytes(bytes, false)
}
