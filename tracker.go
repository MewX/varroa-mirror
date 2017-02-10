package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"golang.org/x/net/publicsuffix"
)

func retrieveGetRequestData(client *http.Client, url string) ([]byte, error) {
	if client == nil {
		return []byte{}, errors.New("Not logged in")
	}
	resp, err := client.Get(url)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []byte{}, errors.New("Error getting URL, returned status: " + resp.Status)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}
	// check success
	var r GazelleGenericResponse
	json.Unmarshal(data, &r)
	if r.Status != "success" {
		if r.Status == "" {
			// TODO : eventually remove debug
			log.Println(string(data))
			return []byte{}, errors.New("Gazelle API call unsuccessful, invalid response. Maybe log in again?")
		}
		return []byte{}, errors.New("Gazelle API call unsuccessful: " + r.Status)
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
		log.Fatal(err)
		return err
	}
	t.client = &http.Client{Jar: jar}
	resp, err := t.client.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("Returned status: " + resp.Status)
	}
	return nil
}

func (t *GazelleTracker) get(url string) ([]byte, error) {
	data, err := retrieveGetRequestData(t.client, url)
	if err != nil {
		// if error, try once again after logging in again
		if err.Error() == "Gazelle API call unsuccessful, invalid response. Maybe log in again?" {
			if err := t.Login(conf.user, conf.password); err == nil {
				data, err := retrieveGetRequestData(t.client, url)
				if err != nil {
					return nil, err
				}
				return data, err
			}
		}
		return nil, err
	}
	return data, err
}

func (t *GazelleTracker) GetStats() (*Stats, error) {
	if t.userID == 0 {
		data, err := t.get(t.rootURL+"/ajax.php?action=index")
		if err != nil {
			return nil, err
		}
		var i GazelleIndex
		json.Unmarshal(data, &i)
		t.userID = i.Response.ID
	}
	// userStats, more precise and updated faster
	data, err := t.get(t.rootURL+"/ajax.php?action=user&id="+strconv.Itoa(t.userID))
	if err != nil {
		return nil, err
	}
	var s GazelleUserStats
	json.Unmarshal(data, &s)
	ratio, err := strconv.ParseFloat(s.Response.Stats.Ratio, 64)
	if err != nil {
		log.Println("Incorrect ratio: " + s.Response.Stats.Ratio)
		ratio = 0.0
	}
	// GazelleIndex to Stats
	stats := &Stats{
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
	data, err := t.get(t.rootURL+"/ajax.php?action=torrent&id="+id)
	if err != nil {
		return nil, err
	}
	var gt GazelleTorrent
	json.Unmarshal(data, &gt)

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
	info := &AdditionalInfo{id: gt.Response.Torrent.ID, label: label, logScore: gt.Response.Torrent.LogScore, artists: artists, size: uint64(gt.Response.Torrent.Size)}
	return info, nil
}

//--------------------

type AdditionalInfo struct {
	id       int
	label    string
	logScore int
	artists  []string // concat artists, composers, etc
	size     uint64
	// TODO: cover (WikiImage), releaseinfo (WikiBody), catnum (CatalogueNumber), filelist (Torrent.FileList), folder? (Torrent.FilePath)
}

func (a *AdditionalInfo) String() string {
	return fmt.Sprintf("Torrent info | Record label: %s | Log Score: %d | Artists: %s | Size %s", a.label, a.logScore, strings.Join(a.artists, ","), humanize.IBytes(uint64(a.size)))
}

//--------------------

const userStats = "User: %s (%s) | "
const progress = "Up: %s (%s) | Down: %s (%s) | Buffer: %s (%s) | Warning Buffer: %s (%s) | Ratio:  %.3f (%.3f)"
const firstProgress = "Up: %s | Down: %s | Buffer: %s | Warning Buffer: %s | Ratio: %.3f"

type Stats struct {
	Username      string
	Class         string
	Up            uint64
	Down          uint64
	Buffer        uint64
	WarningBuffer uint64
	Ratio         float64
}

func (s *Stats) Diff(previous *Stats) (int64, int64, int64, int64, float64) {
	return int64(s.Up - previous.Up), int64(s.Down - previous.Down), int64(s.Buffer - previous.Buffer), int64(s.WarningBuffer - previous.WarningBuffer), s.Ratio - previous.Ratio
}

func (s *Stats) Progress(previous *Stats) string {
	if previous.Ratio == 0 {
		return s.String()
	}
	dup, ddown, ddbuff, ddwbuff, dratio := s.Diff(previous)
	return fmt.Sprintf(progress, readableUInt64(s.Up), readableInt64(dup), readableUInt64(s.Down), readableInt64(ddown), readableUInt64(s.Buffer), readableInt64(ddbuff), readableUInt64(s.WarningBuffer), readableInt64(ddwbuff), s.Ratio, dratio)
}

func (s *Stats) IsProgressAcceptable(previous *Stats, conf *Config) bool {
	if previous.Ratio == 0 {
		// first pass
		return true
	}
	_, _, bufferChange, _, _ := s.Diff(previous)
	if bufferChange > -int64(conf.maxBufferDecreaseByPeriodMB) {
		return true
	}
	return false
}

func (s *Stats) String() string {
	return fmt.Sprintf(userStats, s.Username, s.Class) + fmt.Sprintf(firstProgress, readableUInt64(s.Up), readableUInt64(s.Down), readableUInt64(s.Buffer), readableUInt64(s.WarningBuffer), s.Ratio)
}
