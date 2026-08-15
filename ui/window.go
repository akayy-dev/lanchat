package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ChatWindowModel struct {
	Users    []User
	Messages []ChatMessage
}

func (c ChatWindowModel) Init() tea.Cmd {
	return tea.ClearScreen
}

func (c ChatWindowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return c, tea.Quit
		}
	case NewUserMsg:
		user := User{
			Name:  msg.Peer.PeerID,
			Color: generateRandomHexString(),
		}
		c.Users = append(c.Users, user)
		//
		// Format the user name with their color using lipgloss
		formattedName := lipgloss.NewStyle().Foreground(lipgloss.Color(user.Color)).Render(user.Name)
		joinMsg := ChatMessage{
			Type:    SYSTEM_MESSAGE,
			Content: formattedName + " joined the chat",
		}
		c.Messages = append(c.Messages, joinMsg)
	}
	return c, nil
}

func (c ChatWindowModel) View() string {
	var output string
	if len(c.Messages) == 0 {
		output = "Waiting for peers to join...\n"
	}
	for _, msg := range c.Messages {
		output += msg.Content + "\n"
	}
	return output
}
