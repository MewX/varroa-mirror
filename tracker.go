package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/publicsuffix"
)

const (
	// Gazelle usually only allows 5 API calls every 10s
	// Using 2 every 4 to force calls to be more spread in time
	allowedAPICallsByPeriod = 2
	apiCallsPeriodS         = 4

	statusSuccess = "success"

	logScorePattern = `(-?\d*)</span> \(out of 100\)</blockquote>`

	// Notable ratios
	demotionRatio = 0.95
	warningRatio  = 0.6
)

func (t *GazelleTracker) apiCallRateLimiter() {
	// fill the rate limiter the first time
	for i := 0; i < allowedAPICallsByPeriod; i++ {
		t.limiter <- true
	}
	// every apiCallsPeriodS, refill the limiter channel
	for range time.Tick(time.Second * time.Duration(apiCallsPeriodS)) {
	Loop:
		for i := 0; i < allowedAPICallsByPeriod; i++ {
			select {
			case t.limiter <- true:
			default:
				// if channel is full, do nothing and wait for the next tick
				break Loop
			}
		}
	}
}

func (t *GazelleTracker) callJSONAPI(client *http.Client, url string) ([]byte, error) {
	if client == nil {
		return []byte{}, errors.New(errorNotLoggedIn)
	}
	// wait for rate limiter
	<-t.limiter
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
		logThis.Info("BAD JSON, Received: \n"+string(data), VERBOSEST)
		return []byte{}, errors.Wrap(err, errorUnmarshallingJSON)
	}
	if r.Status != statusSuccess {
		if r.Status == "" {
			return data, errors.New(errorInvalidResponse)
		}
		if r.Error == errorGazelleRateLimitExceeded {
			logThis.Info(errorJSONAPI+": "+errorGazelleRateLimitExceeded+", retrying.", NORMAL)
			// calling again, waiting for the rate limiter again should do the trick.
			// that way 2 limiter slots will have passed before the next call is made,
			// the server should allow it.
			<-t.limiter
			return t.callJSONAPI(client, url)
		}
		return data, errors.New(errorAPIResponseStatus + r.Status)
	}
	return data, nil
}

//--------------------

type GazelleTracker struct {
	Name     string
	URL      string
	User     string
	Password string
	client   *http.Client
	userID   int
	limiter  chan bool //  <- 1/tracker
}

func (t *GazelleTracker) Login() error {
	if t.User == "" || t.Password == "" {
		return errors.New("missing login information")
	}
	form := url.Values{}
	form.Add("username", t.User)
	form.Add("password", t.Password)
	form.Add("keeplogged", "1")
	req, err := http.NewRequest("POST", t.URL+"/login.php", strings.NewReader(form.Encode()))
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
		logThis.Error(errors.Wrap(err, errorLogIn), NORMAL)
		return err
	}
	t.client = &http.Client{Jar: jar}
	resp, err := t.client.Do(req)
	if err != nil {
		logThis.Error(errors.Wrap(err, errorLogIn), NORMAL)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(errorLogIn + ": Returned status: " + resp.Status)
	}
	if resp.Request.URL.String() == t.URL+"/login.php" {
		// if after sending the request we're still redirected to the login page, something went wrong.
		return errors.New(errorLogIn + ": Login page returned")
	}
	return nil
}

func (t *GazelleTracker) get(url string) ([]byte, error) {
	data, err := t.callJSONAPI(t.client, url)
	if err != nil {
		logThis.Error(errors.Wrap(err, errorJSONAPI), NORMAL)
		// if error, try once again after logging in again
		if loginErr := t.Login(); loginErr == nil {
			return t.callJSONAPI(t.client, url)
		}
		return nil, errors.New("Could not log in and send get request to " + url)
	}
	return data, err
}

func (t *GazelleTracker) DownloadTorrent(r *Release, destinationFolder string) error {
	if r.torrentURL == "" || r.TorrentFile == "" {
		return errors.New(errorUnknownTorrentURL)
	}
	response, err := t.client.Get(r.torrentURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	file, err := os.Create(r.TorrentFile)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}
	// move to relevant directory
	if err := CopyFile(r.TorrentFile, filepath.Join(destinationFolder, r.TorrentFile)); err != nil {
		return errors.Wrap(err, errorCouldNotMoveTorrent)
	}
	// cleaning up
	if err := os.Remove(r.TorrentFile); err != nil {
		logThis.Info(fmt.Sprintf(errorRemovingTempFile, r.TorrentFile), VERBOSE)
	}
	return nil
}

func (t *GazelleTracker) GetStats() (*TrackerStats, error) {
	if t.userID == 0 {
		data, err := t.get(t.URL + "/ajax.php?action=index")
		if err != nil {
			return nil, errors.Wrap(err, errorJSONAPI)
		}
		var i GazelleIndex
		if err := json.Unmarshal(data, &i); err != nil {
			return nil, errors.Wrap(err, errorUnmarshallingJSON)
		}
		t.userID = i.Response.ID
	}
	// userStats, more precise and updated faster
	data, err := t.get(t.URL + "/ajax.php?action=user&id=" + strconv.Itoa(t.userID))
	if err != nil {
		return nil, errors.Wrap(err, errorJSONAPI)
	}
	var s GazelleUserStats
	if unmarshalErr := json.Unmarshal(data, &s); unmarshalErr != nil {
		return nil, errors.Wrap(unmarshalErr, errorUnmarshallingJSON)
	}
	ratio, err := strconv.ParseFloat(s.Response.Stats.Ratio, 64)
	if err != nil {
		logThis.Info("Incorrect ratio: "+s.Response.Stats.Ratio, NORMAL)
		ratio = 0.0
	}
	// GazelleIndex to TrackerStats
	stats := &TrackerStats{
		Username:      s.Response.Username,
		Class:         s.Response.Personal.Class,
		Up:            uint64(s.Response.Stats.Uploaded),
		Down:          uint64(s.Response.Stats.Downloaded),
		Buffer:        int64(float64(s.Response.Stats.Uploaded)/demotionRatio) - int64(s.Response.Stats.Downloaded),
		WarningBuffer: int64(float64(s.Response.Stats.Uploaded)/warningRatio) - int64(s.Response.Stats.Downloaded),
		Ratio:         ratio,
	}
	return stats, nil
}

func (t *GazelleTracker) GetTorrentInfo(id string) (*TrackerTorrentInfo, error) {
	data, err := t.get(t.URL + "/ajax.php?action=torrent&id=" + id)
	if err != nil {
		return nil, errors.Wrap(err, errorJSONAPI)
	}
	var gt GazelleTorrent
	if unmarshalErr := json.Unmarshal(data, &gt); unmarshalErr != nil {
		return nil, errors.Wrap(unmarshalErr, errorUnmarshallingJSON)
	}

	artists := map[string]int{}
	// for now, using artists, composers, "with" categories
	for _, el := range gt.Response.Group.MusicInfo.Artists {
		artists[el.Name] = el.ID
	}
	for _, el := range gt.Response.Group.MusicInfo.With {
		artists[el.Name] = el.ID
	}
	for _, el := range gt.Response.Group.MusicInfo.Composers {
		artists[el.Name] = el.ID
	}
	label := gt.Response.Group.RecordLabel
	if gt.Response.Torrent.Remastered {
		label = gt.Response.Torrent.RemasterRecordLabel
	}
	// keeping a copy of uploader before anonymizing
	uploader := gt.Response.Torrent.Username
	// json for metadata, anonymized
	gt.Response.Torrent.Username = ""
	gt.Response.Torrent.UserID = 0
	metadataJSON, err := json.MarshalIndent(gt.Response, "", "    ")
	if err != nil {
		metadataJSON = data // falling back to complete json
	}
	info := &TrackerTorrentInfo{id: gt.Response.Torrent.ID, groupID: gt.Response.Group.ID, edition: gt.Response.Torrent.RemasterTitle, label: label, logScore: gt.Response.Torrent.LogScore, artists: artists, size: uint64(gt.Response.Torrent.Size), uploader: uploader, coverURL: gt.Response.Group.WikiImage, folder: gt.Response.Torrent.FilePath, fullJSON: metadataJSON}
	return info, nil
}

func (t *GazelleTracker) GetArtistInfo(artistID int) (*TrackerArtistInfo, error) {
	data, err := t.get(t.URL + "/ajax.php?action=artist&id=" + strconv.Itoa(artistID))
	if err != nil {
		return nil, errors.Wrap(err, errorJSONAPI)
	}
	var gt GazelleArtist
	if unmarshalErr := json.Unmarshal(data, &gt); unmarshalErr != nil {
		return nil, errors.Wrap(unmarshalErr, errorUnmarshallingJSON)
	}
	// TODO get specific info?
	// json for metadata
	metadataJSON, err := json.MarshalIndent(gt.Response, "", "    ")
	if err != nil {
		metadataJSON = data // falling back to complete json
	}
	info := &TrackerArtistInfo{id: gt.Response.ID, name: gt.Response.Name, fullJSON: metadataJSON}
	return info, nil
}

func (t *GazelleTracker) GetTorrentGroupInfo(torrentGroupID int) (*TrackerTorrentGroupInfo, error) {
	data, err := t.get(t.URL + "/ajax.php?action=torrentgroup&id=" + strconv.Itoa(torrentGroupID))
	if err != nil {
		return nil, errors.Wrap(err, errorJSONAPI)
	}
	var gt GazelleTorrentGroup
	if unmarshalErr := json.Unmarshal(data, &gt); unmarshalErr != nil {
		return nil, errors.Wrap(unmarshalErr, errorUnmarshallingJSON)
	}
	// TODO get specific info?
	// json for metadata, anonymized
	for i := range gt.Response.Torrents {
		gt.Response.Torrents[i].UserID = 0
		gt.Response.Torrents[i].Username = ""
	}
	metadataJSON, err := json.MarshalIndent(gt.Response, "", "    ")
	if err != nil {
		metadataJSON = data // falling back to complete json
	}
	info := &TrackerTorrentGroupInfo{id: gt.Response.Group.ID, name: gt.Response.Group.Name, fullJSON: metadataJSON}
	return info, nil
}

func (t *GazelleTracker) prepareLogUpload(uploadURL string, logPath string) (req *http.Request, err error) {
	// setting up the form
	buffer := new(bytes.Buffer)
	w := multipart.NewWriter(buffer)

	// write to "log" input
	f, err := os.Open(logPath)
	if err != nil {
		return nil, errors.Wrap(err, errorCouldNotReadLog)
	}
	defer f.Close()
	fw, err := w.CreateFormFile("log", logPath)
	if err != nil {
		return nil, errors.Wrap(err, errorCouldNotCreateForm)
	}
	if _, err = io.Copy(fw, f); err != nil {
		return nil, errors.Wrap(err, errorCouldNotReadLog)
	}
	// some forms use "logfiles[]", so adding the same data to that input name
	// both will be sent, each tracker will pick up what they want
	f2, err := os.Open(logPath)
	if err != nil {
		return nil, errors.Wrap(err, errorCouldNotReadLog)
	}
	defer f2.Close()
	fw2, err := w.CreateFormFile("logfiles[]", logPath)
	if err != nil {
		return nil, errors.Wrap(err, errorCouldNotCreateForm)
	}
	if _, err = io.Copy(fw2, f2); err != nil {
		return nil, errors.Wrap(err, errorCouldNotReadLog)
	}
	// other inputs
	w.WriteField("submit", "true")
	w.WriteField("action", "takeupload")
	w.Close()

	req, err = http.NewRequest("POST", uploadURL, buffer)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return
}

func (t *GazelleTracker) GetLogScore(logPath string) (string, error) {
	if !FileExists(logPath) {
		return "", errors.New("Log does not exist: " + logPath)
	}
	// prepare upload
	req, err := t.prepareLogUpload(t.URL+"/logchecker.php", logPath)
	if err != nil {
		return "", errors.New("Could not prepare upload form: " + err.Error())
	}
	// submit the request
	resp, err := t.client.Do(req)
	if err != nil {
		return "", errors.New("Could not upload log: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("Returned status: " + resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("Could not read response")
	}

	// getting log score
	returnData := string(data)
	r := regexp.MustCompile(logScorePattern)
	if r.MatchString(returnData) {
		return "Log score " + r.FindStringSubmatch(returnData)[1], nil
	} else {
		if strings.Contains(returnData, "Your log has failed.") {
			return "Log rejected", nil
		} else if strings.Contains(returnData, "This too shall pass.") {
			return "Log checks out, at least Silver", nil
		}
		return "", errors.New("Could not find score")
	}
}
