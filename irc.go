package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/gregdel/pushover"
	"github.com/thoj/go-ircevent"
)

const announcePattern = `([\w. ]*) - ([\w,\.:' ]*) \[([\d]{4})\] \[(Album|Soundtrack|Compilation|Anthology|EP|Single|Live album|Remix|Bootleg|Interview|Mixtape|Demo|Concert Recording|DJ Mix|Unknown)\] - (FLAC|MP3) / ([\w/ ()]*) / ([\w]*) - (http[s]?://[\w\./:]*torrents\.php\?id=[\d]*) / (http[s]?://[\w\./:]*torrents\.php\?action=download&id=[\d]*) - ([\w\., ]*)`

func AnalyzeAnnounce(config Config, announced string, hc *http.Client, notification *pushover.Pushover, recipient *pushover.Recipient) (*Release, error) {
	// getting information
	r := regexp.MustCompile(announcePattern)
	hits := r.FindAllStringSubmatch(announced, -1)
	if len(hits) != 0 {
		newTorrent, err := NewTorrent(hits[0])
		if err != nil {
			return nil, err
		}
		fmt.Println(newTorrent)

		// if satisfies a filter, download
		for _, filter := range config.filters {
			if newTorrent.Satisfies(filter) {
				fmt.Println("This is of interest because of filter " + filter.label + ", downloading.")
				if _, err = newTorrent.Download(hc); err != nil {
					return nil, err
				}
				// TODO: compare with max-size from filter
				newTorrent.GetSize()

				// send notification
				// TODO: better description (give size, etc)
				message := pushover.NewMessageWithTitle("AutoDL: "+newTorrent.filename, "dronelister")
				_, err := notification.SendMessage(message, recipient)
				if err != nil {
					log.Println(err.Error())
				}

				return newTorrent, nil
			}
		}
		return nil, errors.New("Not interesting.")

	}
	return nil, errors.New("No hits!")
}

func ircHandler() {
	conf := Config{}
	conf.load("config.yaml")

	tracker := GazelleTracker{rootURL: conf.url}
	if err := tracker.Login(conf.user, conf.password); err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("Logged in tracker.")

	// Create a new pushover app with a token
	notification := pushover.New(conf.pushoverToken)
	recipient := pushover.NewRecipient(conf.pushoverUser)

	irccon := irc.IRC(conf.botName, conf.user)
	irccon.UseTLS = false
	irccon.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	irccon.AddCallback("001", func(e *irc.Event) {
		irccon.Privmsg("NickServ", "IDENTIFY "+conf.nickServPassword)
		irccon.Privmsg(conf.announcer, fmt.Sprintf("enter %s %s %s", conf.announceChannel, conf.user, conf.ircKey))
	})
	irccon.AddCallback("PRIVMSG", func(e *irc.Event) {
		if e.Nick == conf.announcer {
			announced := e.Message()
			_, err := AnalyzeAnnounce(conf, announced, tracker.client, notification, recipient)
			if err != nil {
				fmt.Println("ERR: " + err.Error())
				return
			}
		}
	})

	err := irccon.Connect(conf.ircServer)
	if err != nil {
		fmt.Printf("Err %s", err)
		return
	}
	irccon.Loop()
}
