package main

import (
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
