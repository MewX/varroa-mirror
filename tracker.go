package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"golang.org/x/net/publicsuffix"
)

const (
	unknownTorrentURL = "Unknown torrent URL"

	errorLogIn             = "Error logging in: "
	errorNotLoggedIn       = "Not logged in"
	errorJSONAPI           = "Error calling JSON API: "
	errorGET               = "Error calling GET on URL, got HTTP status: "
	errorUnmarshallingJSON = "Error reading JSON: "
	errorInvalidResponse   = "Invalid response. Maybe log in again?"
)

func callJSONAPI(client *http.Client, url string) ([]byte, error) {
	if client == nil {
		return []byte{}, errors.New(errorNotLoggedIn)
	}

	// wait for rate limiter
	<-limiter
	// get request
	resp, err := client.Get(url)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []byte{}, errors.New(errorGET + resp.Status)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}
	// check success
	var r GazelleGenericResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return []byte{}, errors.New(errorUnmarshallingJSON + err.Error())
	}
	if r.Status != "success" {
		if r.Status == "" {
			return data, errors.New(errorInvalidResponse)
		}
		return data, errors.New("Got JSON API status: " + r.Status)
	}
	return data, nil
}

//--------------------

type GazelleTracker struct {
	client  *http.Client
	rootURL string
	userID  int
}

func (t *GazelleTracker) Login(user, password string) error {
	form := url.Values{}
	form.Add("username", user)
	form.Add("password", password)
	req, err := http.NewRequest("POST", t.rootURL+"/login.php", strings.NewReader(form.Encode()))
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	options := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	jar, err := cookiejar.New(&options)
	if err != nil {
		logThis(errorLogIn+err.Error(), NORMAL)
		return err
	}
	t.client = &http.Client{Jar: jar}
	resp, err := t.client.Do(req)
	if err != nil {
		logThis(errorLogIn+err.Error(), NORMAL)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(errorLogIn + "Returned status: " + resp.Status)
	}
	if resp.Request.URL.String() == t.rootURL+"/login.php" {
		// if after sending the request we're still redirected to the login page, something went wrong.
		return errors.New(errorLogIn + "login page returned")
	}
	return nil
}

func (t *GazelleTracker) get(url string) ([]byte, error) {
	data, err := callJSONAPI(t.client, url)
	if err != nil {
		logThis(errorJSONAPI+err.Error(), NORMAL)
		// if error, try once again after logging in again
		if err := t.Login(conf.user, conf.password); err == nil {
			data, err := callJSONAPI(t.client, url)
			if err != nil {
				return nil, err
			}
			return data, err
		} else {
			return nil, errors.New("Could not log in and send get request to " + url)
		}
	}
	return data, err
}

func (t *GazelleTracker) Download(r *Release) (string, error) {
	if r.torrentURL == "" {
		return "", errors.New(unknownTorrentURL)
	}
	response, err := t.client.Get(r.torrentURL)
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
	return r.filename, err
}

func (t *GazelleTracker) GetStats() (*TrackerStats, error) {
	if t.userID == 0 {
		data, err := t.get(t.rootURL + "/ajax.php?action=index")
		if err != nil {
			return nil, errors.New(errorJSONAPI + err.Error())
		}
		var i GazelleIndex
		if err := json.Unmarshal(data, &i); err != nil {
			return nil, errors.New(errorUnmarshallingJSON + err.Error())
		}
		t.userID = i.Response.ID
	}
	// userStats, more precise and updated faster
	data, err := t.get(t.rootURL + "/ajax.php?action=user&id=" + strconv.Itoa(t.userID))
	if err != nil {
		return nil, errors.New(errorJSONAPI + err.Error())
	}
	var s GazelleUserStats
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, errors.New(errorUnmarshallingJSON + err.Error())
	}
	ratio, err := strconv.ParseFloat(s.Response.Stats.Ratio, 64)
	if err != nil {
		logThis("Incorrect ratio: "+s.Response.Stats.Ratio, NORMAL)
		ratio = 0.0
	}
	// GazelleIndex to TrackerStats
	stats := &TrackerStats{
		Username:      s.Response.Username,
		Class:         s.Response.Personal.Class,
		Up:            uint64(s.Response.Stats.Uploaded),
		Down:          uint64(s.Response.Stats.Downloaded),
		Buffer:        uint64(float64(s.Response.Stats.Uploaded)/0.95) - uint64(s.Response.Stats.Downloaded),
		WarningBuffer: uint64(float64(s.Response.Stats.Uploaded)/0.6) - uint64(s.Response.Stats.Downloaded),
		Ratio:         ratio,
	}
	return stats, nil
}

func (t *GazelleTracker) GetTorrentInfo(id string) (*AdditionalInfo, error) {
	data, err := t.get(t.rootURL + "/ajax.php?action=torrent&id=" + id)
	if err != nil {
		return nil, errors.New(errorJSONAPI + err.Error())
	}
	var gt GazelleTorrent
	if err := json.Unmarshal(data, &gt); err != nil {
		return nil, errors.New(errorUnmarshallingJSON + err.Error())
	}

	artists := []string{}
	// for now, using artists, composers, "with" categories
	for _, el := range gt.Response.Group.MusicInfo.Artists {
		artists = append(artists, el.Name)
	}
	for _, el := range gt.Response.Group.MusicInfo.With {
		artists = append(artists, el.Name)
	}
	for _, el := range gt.Response.Group.MusicInfo.Composers {
		artists = append(artists, el.Name)
	}
	label := gt.Response.Group.RecordLabel
	if gt.Response.Torrent.Remastered {
		label = gt.Response.Torrent.RemasterRecordLabel
	}
	// json for metadata
	metadataJson, err := json.MarshalIndent(gt.Response, "", "    ")
	if err != nil {
		metadataJson = data // falling back to complete json
	}
	info := &AdditionalInfo{id: gt.Response.Torrent.ID, label: label, logScore: gt.Response.Torrent.LogScore, artists: artists, size: uint64(gt.Response.Torrent.Size), uploader: gt.Response.Torrent.Username, coverURL: gt.Response.Group.WikiImage, folder: gt.Response.Torrent.FilePath, fullJSON: metadataJson}
	return info, nil
}

//--------------------

type AdditionalInfo struct {
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

func (a *AdditionalInfo) String() string {
	return fmt.Sprintf("Torrent info | Record label: %s | Log Score: %d | Artists: %s | Size %s", a.label, a.logScore, strings.Join(a.artists, ","), humanize.IBytes(uint64(a.size)))
}

func (a *AdditionalInfo) DownloadCover(targetWithoutExtension string) error {
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

//--------------------

const userStats = "User: %s (%s) | "
const progress = "Up: %s (%s) | Down: %s (%s) | Buffer: %s (%s) | Warning Buffer: %s (%s) | Ratio:  %.3f (%.3f)"
const firstProgress = "Up: %s | Down: %s | Buffer: %s | Warning Buffer: %s | Ratio: %.3f"

type TrackerStats struct {
	Username      string
	Class         string
	Up            uint64
	Down          uint64
	Buffer        uint64
	WarningBuffer uint64
	Ratio         float64
}

func (s *TrackerStats) Diff(previous *TrackerStats) (int64, int64, int64, int64, float64) {
	return int64(s.Up - previous.Up), int64(s.Down - previous.Down), int64(s.Buffer - previous.Buffer), int64(s.WarningBuffer - previous.WarningBuffer), s.Ratio - previous.Ratio
}

func (s *TrackerStats) Progress(previous *TrackerStats) string {
	if previous.Ratio == 0 {
		return s.String()
	}
	dup, ddown, dbuff, dwbuff, dratio := s.Diff(previous)
	return fmt.Sprintf(progress, readableUInt64(s.Up), readableInt64(dup), readableUInt64(s.Down), readableInt64(ddown), readableUInt64(s.Buffer), readableInt64(dbuff), readableUInt64(s.WarningBuffer), readableInt64(dwbuff), s.Ratio, dratio)
}

func (s *TrackerStats) IsProgressAcceptable(previous *TrackerStats, maxDecrease int) bool {
	if previous.Ratio == 0 {
		// first pass
		return true
	}
	_, _, bufferChange, _, _ := s.Diff(previous)
	if bufferChange > -int64(maxDecrease*1024*1024) {
		return true
	}
	logThis(fmt.Sprintf("Decrease: %d bytes, only %d allowed. Unacceptable.\n", bufferChange, maxDecrease*1024*1024), VERBOSE)
	return false
}

func (s *TrackerStats) String() string {
	return fmt.Sprintf(userStats, s.Username, s.Class) + fmt.Sprintf(firstProgress, readableUInt64(s.Up), readableUInt64(s.Down), readableUInt64(s.Buffer), readableUInt64(s.WarningBuffer), s.Ratio)
}

func (s *TrackerStats) ToSlice() []string {
	// up;down;ratio;buffer;warningBuffer
	return []string{strconv.FormatUint(s.Up, 10), strconv.FormatUint(s.Down, 10), strconv.FormatFloat(s.Ratio, 'f', -1, 64), strconv.FormatUint(s.Buffer, 10), strconv.FormatUint(s.WarningBuffer, 10)}
}

func (s *TrackerStats) FromSlice(slice []string) error {
	// slice contains timestamp, which is ignored
	if len(slice) != 6 {
		return errors.New("Incorrect entry, cannot load stats")
	}
	up, err := strconv.ParseUint(slice[1], 10, 64)
	if err != nil {
		return err
	}
	s.Up = up
	down, err := strconv.ParseUint(slice[2], 10, 64)
	if err != nil {
		return err
	}
	s.Down = down
	ratio, err := strconv.ParseFloat(slice[3], 64)
	if err != nil {
		return err
	}
	s.Ratio = ratio
	buffer, err := strconv.ParseUint(slice[4], 10, 64)
	if err != nil {
		return err
	}
	s.Buffer = buffer
	warningBuffer, err := strconv.ParseUint(slice[5], 10, 64)
	if err != nil {
		return err
	}
	s.WarningBuffer = warningBuffer
	return nil
}
