package varroa

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gregdel/pushover"
	"github.com/pkg/errors"
)

// Notify in a goroutine, or directly.
func Notify(msg, tracker, msgType string, e *Environment) error {
	conf, err := NewConfig(DefaultConfigurationFile)
	if err != nil {
		return err
	}
	notify := func() error {
		link := ""
		if conf.gitlabPagesConfigured {
			link = conf.GitlabPages.URL
		} else if conf.webserverConfigured && conf.WebServer.ServeStats && conf.WebServer.PortHTTPS != 0 {
			link = "https://" + conf.WebServer.Hostname + ":" + strconv.Itoa(conf.WebServer.PortHTTPS)
		}
		atLeastOneError := false

		// pushover notifications
		if conf.pushoverConfigured {
			pushOver := &Notification{client: pushover.New(conf.Notifications.Pushover.Token), recipient: pushover.NewRecipient(conf.Notifications.Pushover.User)}
			var pngLink string
			if tracker != FullName && strings.HasPrefix(msg, statsNotificationPrefix) && conf.Notifications.Pushover.IncludeBufferGraph {
				pngLink = filepath.Join(StatsDir, tracker+"_"+lastWeekPrefix+"_"+bufferStatsFile+pngExt)
			}
			if err := pushOver.Send(tracker+": "+msg, conf.gitlabPagesConfigured, link, pngLink); err != nil {
				logThis.Error(errors.Wrap(err, errorNotification), VERBOSE)
				atLeastOneError = true
			}
		}
		// webhooks
		if conf.webhooksConfigured && StringInSlice(tracker, conf.Notifications.WebHooks.Trackers) {
			// create json, POST it
			whJSON := &WebHookJSON{Site: tracker, Message: msg, Link: link, Type: msgType}
			if err := whJSON.Send(conf.Notifications.WebHooks.Address, conf.Notifications.WebHooks.Token); err != nil {
				logThis.Error(errors.Wrap(err, errorWebhook), VERBOSE)
				atLeastOneError = true
			}
		}
		// IRC notifications
		if conf.ircNotifsConfigured && e.ircClient != nil {
			e.ircClient.Privmsg(conf.Notifications.Irc.User, msg)
		}

		if atLeastOneError {
			return errors.New(errorNotifications)
		}
		return nil
	}
	return RunOrGo(notify)
}

type Notification struct {
	client    *pushover.Pushover
	recipient *pushover.Recipient
}

func (n *Notification) Send(message string, addLink bool, link, pngLink string) error {
	if n.client == nil || n.recipient == nil {
		return errors.New("Could not send notification: " + message)
	}
	var pushoverMessage *pushover.Message
	if addLink {
		pushoverMessage = &pushover.Message{Message: message, Title: FullName, URL: link, URLTitle: "Graphs"}
	} else {
		pushoverMessage = pushover.NewMessageWithTitle(message, FullName)
	}
	if pngLink != "" {
		file, err := os.Open(pngLink)
		if err != nil {
			logThis.Error(errors.Wrap(err, "error adding png attachment to pushover notification"), VERBOSE)
		} else {
			defer file.Close()
			if addErr := pushoverMessage.AddAttachment(file); addErr != nil {
				logThis.Error(errors.Wrap(err, "error adding png attachment to pushover notification"), VERBOSE)
			}
		}
	}
	_, err := n.client.SendMessage(pushoverMessage, n.recipient)
	return err
}

// -----------------------------------------------------------------------------

type WebHookJSON struct {
	Site    string
	Message string
	Type    string // "error" "info"
	Link    string
}

func (whj *WebHookJSON) Send(address string, token string) error {
	// TODO check address?

	// create POST request
	hook, err := json.Marshal(whj)
	if err != nil {
		return errors.Wrap(err, "Error creating webhook JSON")
	}

	req, err := http.NewRequest("POST", address, bytes.NewBuffer(hook))
	if err != nil {
		return errors.Wrap(err, "Error preparing webhook request")
	}
	req.Header.Set("X-Varroa-Event", whj.Type)
	req.Header.Set("X-Varroa-Token", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Error sending webhook request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("Webhook remote returned status: " + resp.Status)
	}
	// not doing anything with body, really.
	return nil
}
