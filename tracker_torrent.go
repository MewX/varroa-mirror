package main

import (
	"encoding/json"
	"fmt"
	"io"
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

func (a *TrackerTorrentInfo) Release() *Release {
	if len(a.fullJSON) == 0 {
		return nil // nothing useful here
	}
	var gt GazelleTorrent
	if unmarshalErr := json.Unmarshal(a.fullJSON, &gt.Response); unmarshalErr != nil {
		logThis.Error(errors.Wrap(unmarshalErr, "Error parsing torrent info JSON"), NORMAL)
		return nil
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
