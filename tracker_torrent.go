package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"
)

type TrackerTorrentInfo struct {
	id       int
	label    string
	logScore int
	artists  []string // concat artists, composers, etc
	size     uint64
	uploader string
	folder   string
	coverURL string
	fullJSON []byte
}

func (a *TrackerTorrentInfo) String() string {
	return fmt.Sprintf("Torrent info | Record label: %s | Log Score: %d | Artists: %s | Size %s", a.label, a.logScore, strings.Join(a.artists, ","), humanize.IBytes(uint64(a.size)))
}

func (a *TrackerTorrentInfo) DownloadCover(targetWithoutExtension string) error {
	if a.coverURL == "" {
		return errors.New("Unknown image url")
	}
	extension := filepath.Ext(a.coverURL)
	if _, err := FileExists(targetWithoutExtension + extension); err == nil {
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

func (a *TrackerTorrentInfo) Release() *Release {
	if len(a.fullJSON) == 0 {
		return nil // nothing useful here
	}
	var gt GazelleTorrent
	if unmarshalErr := json.Unmarshal(a.fullJSON, &gt.Response); unmarshalErr != nil {
		logThis("Error parsing torrent info JSON", NORMAL)
		return nil
	}
	r := &Release{}
	// for now, using artists, composers, "with" categories
	for _, el := range gt.Response.Group.MusicInfo.Artists {
		r.artist = append(r.artist, el.Name)
	}
	for _, el := range gt.Response.Group.MusicInfo.With {
		r.artist = append(r.artist, el.Name)
	}
	for _, el := range gt.Response.Group.MusicInfo.Composers {
		r.artist = append(r.artist, el.Name)
	}
	r.title = gt.Response.Group.Name

	if gt.Response.Torrent.Remastered {
		r.year = gt.Response.Torrent.RemasterYear
	} else {
		r.year = gt.Response.Group.Year
	}
	switch gt.Response.Group.ReleaseType {
	case 1:
		r.releaseType = "Album"
	case 3:
		r.releaseType = "Soundtrack"
	case 5:
		r.releaseType = "EP"
	case 6:
		r.releaseType = "Anthology"
	case 7:
		r.releaseType = "Compilation"
	case 9:
		r.releaseType = "Single"
	case 11:
		r.releaseType = "Live album"
	case 13:
		r.releaseType = "Remix"
	case 14:
		r.releaseType = "Bootleg"
	case 15:
		r.releaseType = "Interview"
	case 16:
		r.releaseType = "Mixtape"
	case 17:
		r.releaseType = "Demo"
	case 18:
		r.releaseType = "Concert Recording"
	case 19:
		r.releaseType = "DJ Mix"
	case 21:
		r.releaseType = "Unknown"
	}
	r.format = gt.Response.Torrent.Format
	r.quality = gt.Response.Torrent.Encoding
	r.hasLog = gt.Response.Torrent.HasLog
	r.hasCue = gt.Response.Torrent.HasCue
	r.isScene = gt.Response.Torrent.Scene
	r.source = gt.Response.Torrent.Media
	// not found in Gazelle API... r.tags =
	// r.url =
	// r.torrentURL =
	r.torrentID = fmt.Sprintf("%d", gt.Response.Torrent.ID)
	// r.filename =
	r.size = uint64(gt.Response.Torrent.Size)
	r.folder = gt.Response.Torrent.FilePath
	r.logScore = gt.Response.Torrent.LogScore
	r.uploader = gt.Response.Torrent.Username
	return r
}
