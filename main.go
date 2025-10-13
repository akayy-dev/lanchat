package main

import (
	"LANChat/chat"
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"
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

	// Listen for and render messages
	msgChan, _ := messenger.Listen()
	go func() {
		for msg := range msgChan {
			if msg.Type == chat.USERJOIN {
				fmt.Printf(
					"%s@%s has joined the chat\n",
					msg.User.Name, msg.User.Hostname,
				)
			}
			if msg.Type == chat.USERLEAVE {
				fmt.Printf(
					"%s@%s has left the chat\n",
					msg.User.Name, msg.User.Hostname,
				)
			}
			if msg.Type == chat.MESSAGESEND {
				fmt.Printf("%s@%s - %s\n%s", msg.User.Hostname, msg.User.Hostname, msg.Time.Format("3:04 PM"), msg.Content)
			}
		}
	}()

	// Get username & hostname
	systemUser, err := user.Current()
	if err != nil {
		panic(err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	chatUser := chat.User{
		Name:     systemUser.Username,
		Hostname: hostname,
	}

	messenger.Send(
		chat.Message{
			User: chatUser,
			Type: chat.USERJOIN,
		},
	)
	// Loop to scan for input
	inputReader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		text, err := inputReader.ReadString('\n')
		text = strings.TrimSpace(text)

		if err != nil {
			fmt.Printf("Error reading input, %v", err)
		}

		if text == "" {
			continue
		}

		if text == "/quit" {
			messenger.Send(chat.Message{
				User: chatUser,
				Type: chat.USERLEAVE,
			})
			break
		}
		messenger.Send(
			chat.Message{
				User:    chatUser,
				Content: text,
				Time:    time.Now(),
				Type:    chat.MESSAGESEND,
			},
		)
	}
}
