package varroa

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

	logScorePattern           = `(-?\d*)</span> \(out of 100\)</blockquote>`
	deletedFromTrackerPattern = `<span class="log_deleted"> Torrent <a href="torrents\.php\?torrentid=\d+">(\d+)</a>(.*)</span>`
)

func userAgent() string {
	return FullNameAlt + "/" + Version[1:]
}

// GazelleTracker allows querying the Gazelle JSON API.
type GazelleTracker struct {
	Name     string
	URL      string
	User     string
	Password string
	client   *http.Client
	userID   int
	limiter  chan bool //  <- 1/tracker
}

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

func (t *GazelleTracker) getRequest(client *http.Client, url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", userAgent())
	return client.Do(req)
}

func (t *GazelleTracker) rateLimitedGetRequest(url string, expectingJSON bool) ([]byte, error) {
	if t.client == nil {
		return []byte{}, errors.New(errorNotLoggedIn)
	}
	// wait for rate limiter
	<-t.limiter
	// get request
	resp, err := t.getRequest(t.client, url)
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
	if expectingJSON {
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
				return t.rateLimitedGetRequest(url, expectingJSON)
			}
			return data, errors.New(errorAPIResponseStatus + r.Status)
		}
		return data, nil
	}
	return data, nil
}

func (t *GazelleTracker) apiCall(endpoint string) ([]byte, error) {
	data, err := t.rateLimitedGetRequest(t.URL+"/ajax.php?"+endpoint, true)
	if err != nil {
		logThis.Error(errors.Wrap(err, errorJSONAPI), VERBOSEST)
		// if error, try once again after logging in again
		if loginErr := t.Login(); loginErr == nil {
			return t.rateLimitedGetRequest(endpoint, true)
		}
		return nil, errors.New("Could not log in and send get request to " + endpoint)
	}
	return data, err
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
	req.Header.Add("User-Agent", userAgent())

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

// DownloadTorrent using its download URL.
func (t *GazelleTracker) DownloadTorrent(torrentURL, torrentFile, destinationFolder string) error {
	if torrentURL == "" || torrentFile == "" {
		return errors.New(errorUnknownTorrentURL)
	}
	response, err := t.getRequest(t.client, torrentURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	file, err := os.Create(torrentFile)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}
	// move to relevant directory
	if err := CopyFile(torrentFile, filepath.Join(destinationFolder, torrentFile), false); err != nil {
		return errors.Wrap(err, errorCouldNotMoveTorrent)
	}
	// cleaning up
	if err := os.Remove(torrentFile); err != nil {
		logThis.Info(fmt.Sprintf(errorRemovingTempFile, torrentFile), VERBOSE)
	}
	return nil
}

// DownloadTorrentFromID using its ID instead.
func (t *GazelleTracker) DownloadTorrentFromID(torrentID string, destinationFolder string, useFLToken bool) error {
	torrentFile := SanitizeFolder(t.Name) + "_id" + torrentID + torrentExt
	torrentURL := t.URL + "/torrents.php?action=download&id=" + torrentID
	if useFLToken {
		torrentURL += "&usetoken=1"
	}
	return t.DownloadTorrent(torrentURL, torrentFile, destinationFolder)
}

func (t *GazelleTracker) GetStats() (*StatsEntry, error) {
	if t.userID == 0 {
		data, err := t.apiCall("action=index")
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
	data, err := t.apiCall("action=user&id=" + strconv.Itoa(t.userID))
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
	// return StatsEntry
	stats := &StatsEntry{
		Tracker:       t.Name,
		Up:            s.Response.Stats.Uploaded,
		Down:          s.Response.Stats.Downloaded,
		Ratio:         ratio,
		Timestamp:     time.Now(),
		TimestampUnix: time.Now().Unix(),
		Collected:     true,
		SchemaVersion: currentStatsDBSchemaVersion,
	}
	return stats, nil
}

func (t *GazelleTracker) GetTorrentMetadata(id string) (*TrackerMetadata, error) {
	data, err := t.apiCall("action=torrent&id=" + id)
	if err != nil {
		isDeleted, deletedText, errCheck := t.CheckIfDeleted(id)
		if errCheck != nil {
			logThis.Error(errors.Wrap(errCheck, "could not get information from site log"), VERBOSE)
		} else if isDeleted {
			logThis.Info(Red(deletedText), NORMAL)
		}
		return nil, errors.Wrap(err, errorJSONAPI)
	}
	// json bytes will be re-saved by info after anonymizing
	info := &TrackerMetadata{ReleaseJSON: data}
	if unmarshalErr := info.LoadFromTracker(t, data); err != nil {
		return nil, errors.Wrap(unmarshalErr, errorUnmarshallingJSON)
	}
	return info, nil
}

func (t *GazelleTracker) GetArtistInfo(artistID int) (*TrackerMetadataArtist, error) {
	data, err := t.apiCall("action=artist&id=" + strconv.Itoa(artistID))
	if err != nil {
		return nil, errors.Wrap(err, errorJSONAPI)
	}
	var gt GazelleArtist
	if unmarshalErr := json.Unmarshal(data, &gt); unmarshalErr != nil {
		return nil, errors.Wrap(unmarshalErr, errorUnmarshallingJSON)
	}
	// json for metadata
	metadataJSON, err := json.MarshalIndent(gt.Response, "", "    ")
	if err != nil {
		metadataJSON = data // falling back to complete json
	}
	info := &TrackerMetadataArtist{ID: gt.Response.ID, Name: gt.Response.Name, JSON: metadataJSON}
	return info, nil
}

func (t *GazelleTracker) GetTorrentGroupInfo(torrentGroupID int) (*TrackerMetadataTorrentGroup, error) {
	data, err := t.apiCall("action=torrentgroup&id=" + strconv.Itoa(torrentGroupID))
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
	info := &TrackerMetadataTorrentGroup{id: gt.Response.Group.ID, name: gt.Response.Group.Name, fullJSON: metadataJSON}
	return info, nil
}

func (t *GazelleTracker) prepareLogUpload(uploadURL string, logPath string) (*http.Request, error) {
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

	req, err := http.NewRequest("POST", uploadURL, buffer)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Add("User-Agent", userAgent())
	return req, err
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
	}
	if strings.Contains(returnData, "Your log has failed.") {
		return "Log rejected", nil
	} else if strings.Contains(returnData, "This too shall pass.") {
		return "Log checks out, at least Silver", nil
	}
	return "", errors.New("Could not find score")
}

func (t *GazelleTracker) CheckIfDeleted(torrentID string) (bool, string, error) {
	data, err := t.rateLimitedGetRequest(t.URL+"/log.php?search=Torrent+"+torrentID, false)
	if err != nil {
		return false, "", err
	}
	// getting deletion reason
	returnData := string(data)
	r := regexp.MustCompile(deletedFromTrackerPattern)
	if r.MatchString(returnData) {
		if r.FindStringSubmatch(returnData)[1] == torrentID {
			return true, "Site log: Torrent " + r.FindStringSubmatch(returnData)[1] + " " + r.FindStringSubmatch(returnData)[2], nil
		}
	}
	return false, "", nil
}
