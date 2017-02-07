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
	"strings"

	"golang.org/x/net/publicsuffix"
	"math"
)

func login(siteUrl, username, password string) (hc *http.Client, returnData string, err error) {
	form := url.Values{}
	form.Add("username", username)
	form.Add("password", password)
	req, err := http.NewRequest("POST", siteUrl, strings.NewReader(form.Encode()))
	if err != nil {
		fmt.Println(err.Error())
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	options := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	jar, err := cookiejar.New(&options)
	if err != nil {
		log.Fatal(err)
	}
	hc = &http.Client{Jar: jar}

	resp, err := hc.Do(req)
	if err != nil {
		fmt.Println(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = errors.New("Returned status: " + resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	returnData = string(data)
	return
}

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
	return data, nil
}

type ByteSize float64

const (
	_           = iota // ignore first value by assigning to blank identifier
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
)

func (b ByteSize) String() string {
	switch {
	case b >= TB:
		return fmt.Sprintf("%.3fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.3fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.3fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.3fKB", b/KB)
	}
	return fmt.Sprintf("%.3fB", b)
}

func readableUint64(a uint64) string {
	return ByteSize(float64(a)).String()
}
func readableint64(a int64) string {
	if a >= 0 {
		return "+"+ByteSize(math.Abs(float64(a))).String()
	}
	return "-"+ByteSize(math.Abs(float64(a))).String()
}
//--------------------

type GazelleTracker struct {
	client  *http.Client
	rootURL string
}

func (t *GazelleTracker) Login(user, password string) error {
	client, _, err := login(t.rootURL+"/login.php", user, password)
	if err != nil {
		return errors.New("Could not log in")
	}
	t.client = client
	return nil
}

func (t *GazelleTracker) GetStats() (*Stats, error) {
	data, err := retrieveGetRequestData(t.client, t.rootURL+"/ajax.php?action=index")
	if err != nil {
		return nil, err
	}
	var i GazelleIndex
	json.Unmarshal(data, &i)

	// GazelleIndex to Stats
	stats := &Stats{
		Username:      i.Response.Username,
		Class:         i.Response.Userstats.Class,
		Up:            uint64(i.Response.Userstats.Uploaded),
		Down:          uint64(i.Response.Userstats.Downloaded),
		Buffer:        uint64(float64(i.Response.Userstats.Uploaded)/0.95) - uint64(i.Response.Userstats.Downloaded),
		WarningBuffer: uint64(float64(i.Response.Userstats.Uploaded)/0.6) - uint64(i.Response.Userstats.Downloaded),
		Ratio:         i.Response.Userstats.Ratio,
	}

	return stats, nil
}

func (t *GazelleTracker) GetTorrentInfo(id string) (*AdditionalInfo, error) {
	data, err := retrieveGetRequestData(t.client, t.rootURL+"/ajax.php?action=torrent&id="+id)
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
	info := &AdditionalInfo{id: gt.Response.Torrent.ID, label:label, logScore: gt.Response.Torrent.LogScore, artists: artists}

	//fmt.Println(string(data))
	log.Println(info)
	return info, nil
}

//--------------------

type AdditionalInfo struct {
	id int
	label string
	logScore int
	artists []string // concat artists, composers, etc
	// TODO: cover (WikiImage), releaseinfo (WikiBody), catnum (CatalogueNumber), filelist (Torrent.FileList), folder? (Torrent.FilePath)
}

func (a *AdditionalInfo) String() string {
	return fmt.Sprintf("Torrent info | Record label: %s | Log Score: %d | Artists: %s", a.label, a.logScore, strings.Join(a.artists, ","))
}

//--------------------

const userStats = "User: %s (%s) | "
const progress = "Up: %s (%s) | Down: %s (%s) | Buffer: %s (%s) | Warning Buffer: %s (%s) | Ratio:  %.2f (%.2f)"
const firstProgress = "Up: %s | Down: %s | Buffer: %s | Warning Buffer: %s | Ratio: %.2f"

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
	return fmt.Sprintf(progress, readableUint64(s.Up), readableint64(dup), readableUint64(s.Down), readableint64(ddown), readableUint64(s.Buffer), readableint64(ddbuff), readableUint64(s.WarningBuffer), readableint64(ddwbuff), s.Ratio, dratio)
}

func (s *Stats) IsProgressAcceptable(previous *Stats, conf Config) bool {
	if previous.Ratio == 0 {
		// first pass
		return true
	}
	_, _, bufferChange, _, _ := s.Diff(previous)
	if bufferChange > - int64(conf.maxBufferDecreaseByPeriodMB) {
	    return true
	}
	return false
}

func (s *Stats) String() string {
	return fmt.Sprintf(userStats, s.Username, s.Class) + fmt.Sprintf(firstProgress, readableUint64(s.Up), readableUint64(s.Down), readableUint64(s.Buffer), readableUint64(s.WarningBuffer), s.Ratio)
}

//-----------

type GazelleIndex struct {
	Response struct {
		Authkey       string `json:"authkey"`
		ID            int    `json:"id"`
		Notifications struct {
			Messages         int  `json:"messages"`
			NewAnnouncement  bool `json:"newAnnouncement"`
			NewBlog          bool `json:"newBlog"`
			NewSubscriptions bool `json:"newSubscriptions"`
			Notifications    int  `json:"notifications"`
		} `json:"notifications"`
		Passkey   string `json:"passkey"`
		Username  string `json:"username"`
		Userstats struct {
			Class         string  `json:"class"`
			Downloaded    int     `json:"downloaded"`
			Ratio         float64 `json:"ratio"`
			Requiredratio float64 `json:"requiredratio"`
			Uploaded      int     `json:"uploaded"`
		} `json:"userstats"`
	} `json:"response"`
	Status string `json:"status"`
}

//----------------------

type GazelleTorrent struct {
	Response struct {
		Group struct {
			CatalogueNumber string `json:"catalogueNumber"`
			CategoryID      int    `json:"categoryId"`
			CategoryName    string `json:"categoryName"`
			ID              int    `json:"id"`
			MusicInfo       struct {
				Artists []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"artists"`
				Composers []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"composers"`
				Conductor []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"conductor"`
				Dj        []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"dj"`
				Producer  []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"producer"`
				RemixedBy []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"remixedBy"`
				With      []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"with"`
			} `json:"musicInfo"`
			Name        string `json:"name"`
			RecordLabel string `json:"recordLabel"`
			ReleaseType int    `json:"releaseType"`
			Time        string `json:"time"`
			VanityHouse bool   `json:"vanityHouse"`
			WikiBody    string `json:"wikiBody"`
			WikiImage   string `json:"wikiImage"`
			Year        int    `json:"year"`
		} `json:"group"`
		Torrent struct {
			Description             string      `json:"description"`
			Encoding                string      `json:"encoding"`
			FileCount               int         `json:"fileCount"`
			FileList                string      `json:"fileList"`
			FilePath                string      `json:"filePath"`
			Format                  string      `json:"format"`
			FreeTorrent             bool        `json:"freeTorrent"`
			HasCue                  bool        `json:"hasCue"`
			HasLog                  bool        `json:"hasLog"`
			ID                      int         `json:"id"`
			Leechers                int         `json:"leechers"`
			LogScore                int         `json:"logScore"`
			Media                   string      `json:"media"`
			RemasterCatalogueNumber string      `json:"remasterCatalogueNumber"`
			RemasterRecordLabel     string      `json:"remasterRecordLabel"`
			RemasterTitle           string      `json:"remasterTitle"`
			RemasterYear            int         `json:"remasterYear"`
			Remastered              bool        `json:"remastered"`
			Scene                   bool        `json:"scene"`
			Seeders                 int         `json:"seeders"`
			Size                    int         `json:"size"`
			Snatched                int         `json:"snatched"`
			Time                    string      `json:"time"`
			UserID                  int         `json:"userId"`
			Username                interface{} `json:"username"`
		} `json:"torrent"`
	} `json:"response"`
	Status string `json:"status"`
}
