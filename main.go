package main

import (
	"LANChat/chat"
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
			formattedUserID := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(msg.User.Color)).Render(fmt.Sprintf("%s@%s", msg.User.Name, msg.User.Hostname))
			if msg.Type == chat.USERJOIN {
				fmt.Printf(
					"%s has joined the chat\n",
					formattedUserID,
				)
			}
			if msg.Type == chat.USERLEAVE {
				fmt.Printf(
					"%s has left the chat\n",
					formattedUserID,
				)
			}
			if msg.Type == chat.MESSAGESEND {
				fmt.Printf("%s - %s\n%s\n", formattedUserID, msg.Time.Format("3:04 PM"), msg.Content)
			}

			fmt.Print("> ")
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

	chatUser := chat.CreateNewUser(systemUser.Username, hostname)

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
