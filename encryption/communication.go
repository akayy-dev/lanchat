package encryption

import (
	"LANChat/chat"
	"time"
)

func WaitForGroupKey(messages chat.MessageService, msgChan <-chan chat.Message) (enc_key string, pubKey [32]byte, ok bool) {
	timeout := time.After(3 * time.Second)

	for {
		select {
		case msg := <-msgChan:
			if msg.Type == chat.KEYSEND && !msg.Circular {
				if msg.User.Keys.PublicKey != nil {
					messages.Send(chat.Message{
						Type:    chat.CLIENT,
						Content: "Received public key",
					})

					return msg.Content, *msg.User.Keys.PublicKey, true
				}
			}
		case <-timeout:
			messages.Send(
				chat.Message{
					Type:    chat.CLIENT,
					Content: "Could not find a public key",
				},
			)
			// On a timeout.
			return "", [32]byte{}, false
		}
	}
}
