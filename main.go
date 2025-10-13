package main

import (
	"LANChat/chat"
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

	chatUI := ui.ChatWindow{
		Messenger: messenger,
		User:      chatUser,
	}

	p := tea.NewProgram(&chatUI)
	// Listen for and render messages
	msgChan, _ := messenger.Listen()
	go func() {
		for msg := range msgChan {
			p.Send(ui.NewMessageMsg(msg))
		}
	}()

	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
