package main

import (
	"LANChat/chat"
	"LANChat/encryption"
	"LANChat/ui"
	"os"
	"os/user"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	MULTICAST_ADDR = "224.0.0.1:9999"
)

func main() {
	messenger, err := chat.NewMulticastMessenger()
	if err != nil {
		panic(err)
	}
	defer messenger.Close()

	// Get username & hostname
	systemUser, err := user.Current()
	if err != nil {
		panic(err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	chatUser := chat.CreateNewUser(systemUser.Username, hostname)
	messenger.UserUUID = chatUser.UUID

	chatUI := ui.ChatWindow{
		Messenger: messenger,
		User:      chatUser,
	}

	p := tea.NewProgram(&chatUI)
	// Listen for and render messages
	msgChan, _ := messenger.Listen()
	go func() {
		// wait for key
		encGroupKey, peerPubKey, ok := encryption.WaitForGroupKey(messenger, msgChan)
		if ok {
			// TODO: if we got a key, decode it and store it in our keychain
			messenger.Send(chat.Message{
				Type:    chat.CLIENT,
				Content: "Got group key, decrypting",
			})

			// Get the nonce from the group key (first 24 bytes)
			src := []byte(encGroupKey)
			// check to make sure the key is the right length
			if len(src) < 24 {
				messenger.Send(chat.Message{
					Type:    chat.CLIENT,
					Content: "Received group key is too short for nonce",
				})
			}

			// pointer shenanigans
			var nonce [24]byte
			copy(nonce[:], src[:24])
			noncePtr := &nonce
			peerPubKeyPtr := &peerPubKey

			key, err := encryption.DecryptGroupKey([]byte(encGroupKey), noncePtr, peerPubKeyPtr, chatUser.Keys.PrivateKey)
			if err != nil {
				panic(err)
			}
			chatUser.Keys.GroupKey = &key
			messenger.Send(chat.Message{
				Type:    chat.CLIENT,
				Content: "Got group key!",
			})
		} else {
			// Request a group key, send your public key.
			messenger.Send(chat.Message{
				Type: chat.KEYREQUEST,
				User: chatUser,
			})
		}
		for msg := range msgChan {
			// if a new user requests the key and we have it, send it to them
			// encrypted with their public key.
			if msg.Type == chat.KEYREQUEST {
				if chatUser.Keys.GroupKey != nil {
					p.Send(chat.Message{
						Type: chat.KEYSEND,
						Content: string(
							encryption.EncryptGroupKey(
								*chatUser.Keys.GroupKey,
								msg.User.Keys.PublicKey,
								chatUser.Keys.PrivateKey,
							),
						),
						User: chatUser,
					},
					)
				}
			}

			p.Send(ui.NewMessageMsg(msg))
		}
	}()

	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
