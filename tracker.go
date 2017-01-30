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

func (t *GazelleTracker) GetStats() error {
	data, err := retrieveGetRequestData(t.client, t.rootURL+"/ajax.php?action=index")
	if err != nil {
		return err
	}
	var index GazelleIndex
	json.Unmarshal(data, &index)
	fmt.Println(index.Stats())
	return nil
}

//--------------------

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


func (i *GazelleIndex) RawBuffer() uint64 {
	return uint64(float64(i.Response.Userstats.Uploaded) / 0.95) - uint64(i.Response.Userstats.Downloaded)
}

func (i *GazelleIndex) Buffer() string {
	return ByteSize(float64(i.RawBuffer())).String()
}

func (i *GazelleIndex) Stats() string {
	return fmt.Sprintf("User: %s (%s) | Up: %s | Down: %s | Buffer: %s | Ratio: %.2f",
		i.Response.Username,
		i.Response.Userstats.Class,
		ByteSize(float64(i.Response.Userstats.Uploaded)).String(),
		ByteSize(float64(i.Response.Userstats.Downloaded)).String(),
		i.Buffer(),
		i.Response.Userstats.Ratio)
}



