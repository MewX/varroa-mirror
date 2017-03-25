package main

import (
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
