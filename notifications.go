package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gregdel/pushover"
	"github.com/pkg/errors"
)

type Notification struct {
	client    *pushover.Pushover
	recipient *pushover.Recipient
}

func (n *Notification) Send(message string, addLink bool, link string) error {
	if n.client == nil || n.recipient == nil {
		return errors.New("Could not send notification: " + message)
	}
	var pushoverMessage *pushover.Message
	if addLink {
		pushoverMessage = &pushover.Message{Message: message, Title: varroa, URL: link, URLTitle: "Graphs"}
	} else {
		pushoverMessage = pushover.NewMessageWithTitle(message, varroa)
	}
	_, err := n.client.SendMessage(pushoverMessage, n.recipient)
	return err
}

//-----------------------------------------------------------------------------

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
	req.Header.Set("X-Varroa-Event", whj.Type)
	req.Header.Set("X-Varroa-Token", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

	return nil
}
