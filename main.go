package main

import (
	"LANChat/chat"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/reeflective/readline"
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

	rl := readline.NewShell()
	rl.Prompt.Primary(func() string { return "> " })
	for {
		line, err := rl.Readline()
		if err != nil {
			fmt.Printf("error reading from stdin: %v", err)
			continue
		}
		if line == "" {
			continue
		}

		rl.Printf("Message sent")
		if line == "/quit" {
			messenger.Send(chat.Message{
				User: chatUser,
				Type: chat.USERLEAVE,
			})
			break
		}
		messenger.Send(
			chat.Message{
				User:    chatUser,
				Content: line,
				Time:    time.Now(),
				Type:    chat.MESSAGESEND,
			},
		)
	}
}
