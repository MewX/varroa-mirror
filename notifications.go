package main

import (
	"errors"

	"github.com/gregdel/pushover"
)

type Notification struct {
	client    *pushover.Pushover
	recipient *pushover.Recipient
}

func (n *Notification) Send(message string) error {
	if n.client == nil || n.recipient == nil {
		return errors.New("Could not send notification: " + message)
	}
	var pushoverMessage *pushover.Message
	if env.config.gitlabPagesConfigured {
		pushoverMessage = &pushover.Message{Message: message, Title: varroa, URL: env.config.GitlabPages.URL, URLTitle: "Graphs"}
	} else {
		pushoverMessage = pushover.NewMessageWithTitle(message, varroa)
	}
	_, err := n.client.SendMessage(pushoverMessage, n.recipient)
	return err
}
