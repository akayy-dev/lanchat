package models

import (
	"strings"

	"LANChat/ui"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func NewChatWindowModel() ChatWindowModel {
	input := textinput.New()
	input.Placeholder = "Type a message..."
	input.Focus()
	model := ChatWindowModel{
		Users:           make(map[string]ui.User),
		Messages:        make([]ui.ChatMessage, 0),
		SentMessageChan: make(chan ui.ChatMessage, 1),
		textinput:       input,
	}
	return model
}

type ChatWindowModel struct {
	Users           map[string]ui.User
	Messages        []ui.ChatMessage
	SentMessageChan chan ui.ChatMessage
	textinput       textinput.Model
}

type InitMsg struct{}

func (c ChatWindowModel) Init() tea.Cmd {
	initCmd := func() tea.Msg {
		return InitMsg{}
	}
	return tea.Batch(tea.ClearScreen, initCmd, textinput.Blink, tea.WindowSize())
}

func (c ChatWindowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case InitMsg:
		// Handle any init logic here.
		return c, nil
	case tea.WindowSizeMsg:
		c.textinput.Width = msg.Width
	case ui.ReceivedChatMessage:
		// FIX: Now uses decoupled ReceivedChatMessage struct instead of chat.TCPMessage.
		// The conversion happens in main.go, keeping UI independent of network layer details.
		c.Messages = append(c.Messages, ui.ChatMessage{
			Type:    ui.USER_MESSAGE,
			Content: msg.Content,
			From:    msg.From,
		})
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return c, tea.Quit
		case "enter":
			input := strings.TrimSpace(c.textinput.Value())
			if input == "/quit" {
				return c, tea.Quit
			}

			if input != "" {
				message := ui.ChatMessage{
					Type:    ui.USER_MESSAGE,
					From:    "You",
					Content: c.textinput.Value(),
				}
				c.Messages = append(c.Messages, message)
				c.textinput.SetValue("")

				// FIX: Use non-blocking send to prevent UI freeze when channel buffer is full.
				// The select with default case allows the UI to remain responsive even if
				// the network layer is slow to consume messages.
				select {
				case c.SentMessageChan <- message:
					// Message sent successfully to network layer
				default:
					// Channel buffer is full - notify user but don't block
					c.Messages = append(c.Messages, ui.ChatMessage{
						Type:    ui.SYSTEM_MESSAGE,
						Content: "Warning: Send buffer full, message may not reach all peers",
					})
				}
			}
			return c, nil
		}
	case ui.NewUserMsg:
		user := ui.User{
			Name:  msg.Peer.PeerID,
			Color: ui.GenerateRandomHexString(),
		}
		c.Users[msg.Peer.PeerID] = user

		// Format the user name with their color using lipgloss
		joinMsg := ui.ChatMessage{
			Type: ui.USER_JOIN,
			From: user.Name,
		}
		c.Messages = append(c.Messages, joinMsg)
	case ui.LeftUserMsg:
		leftMsg := ui.ChatMessage{
			Type: ui.USER_LEFT,
			From: msg.Peer.PeerID,
		}
		c.Messages = append(c.Messages, leftMsg)
		delete(c.Users, msg.Peer.PeerID)
	case ui.MessageUpdate:
		c.Messages = append(c.Messages, ui.ChatMessage{
			Type:    msg.Type,
			Content: msg.Content,
			From:    msg.From,
		})
	}
	c.textinput, cmd = c.textinput.Update(msg)
	return c, cmd
}

func (c ChatWindowModel) View() string {
	var sb strings.Builder
	if len(c.Messages) == 0 {
		sb.WriteString("Waiting for peers to join...\n")
	}
	for _, msg := range c.Messages {
		formattedName := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Users[msg.From].Color)).Render(msg.From)
		switch msg.Type {
		case ui.USER_JOIN:
			sb.WriteString(formattedName + " joined the chat\n")
		case ui.USER_LEFT:
			sb.WriteString(formattedName + " left the chat\n")
		case ui.USER_MESSAGE:
			// HACK: Handle "You" specially since it's not in the Users map.
			// "You" refers to the local user whose messages are displayed locally without
			// going through the Users map (which only tracks remote peers).
			var formattedName string
			if msg.From == "You" {
				// Use bold cyan for local user's messages
				formattedName = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FFFF")).Render(msg.From)
			} else if user, ok := c.Users[msg.From]; ok {
				formattedName = lipgloss.NewStyle().Foreground(lipgloss.Color(user.Color)).Render(msg.From)
			} else {
				formattedName = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(msg.From)
			}
			sb.WriteString(formattedName + ": " + msg.Content + "\n")

		case ui.SYSTEM_MESSAGE:
			formattedContent := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8)).Render(msg.Content)
			sb.WriteString(formattedContent + "\n")
		}
	}
	sb.WriteString(c.textinput.View())
	return sb.String()
}
