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
	pushoverMessage := pushover.NewMessageWithTitle(message, "varroa musica")
	_, err := n.client.SendMessage(pushoverMessage, n.recipient)
	return err
}
