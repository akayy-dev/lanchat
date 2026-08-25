package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func NewChatWindowModel() ChatWindowModel {
	input := textinput.New()
	input.Placeholder = "Type a message..."
	input.Focus()
	model := ChatWindowModel{
		Users:           make(map[string]User),
		Messages:        make([]ChatMessage, 0),
		SentMessageChan: make(chan ChatMessage, 1),
		textinput:       input,
	}
	return model
}

type ChatWindowModel struct {
	Users           map[string]User
	Messages        []ChatMessage
	SentMessageChan chan ChatMessage
	textinput       textinput.Model
}

type InitMsg struct{}

func (c ChatWindowModel) Init() tea.Cmd {
	initCmd := func() tea.Msg {
		return InitMsg{}
	}
	return tea.Batch(initCmd, textinput.Blink, tea.WindowSize())
}

// Writes a SYSTEM_MESSAGE to the screen as if it was a chat message, useful for logging as it implementst he io.Writer interface.
func (c ChatWindowModel) Write(b []byte) (int, error) {
	c.Messages = append(c.Messages, ChatMessage{
		Type:    SYSTEM_MESSAGE,
		Content: string(b),
	})
	return len(b), nil
}

func (c ChatWindowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case InitMsg:
		// Handle any init logic here.
		return c, nil
	case tea.WindowSizeMsg:
		c.textinput.Width = msg.Width
	case ReceivedChatMessage:
		c.Messages = append(c.Messages, ChatMessage{
			Type:    USER_MESSAGE,
			Content: string(msg.Message.Content),
			From:    msg.Message.From,
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
				message := ChatMessage{
					Type:    USER_MESSAGE,
					From:    "You",
					Content: c.textinput.Value(),
				}
				c.Messages = append(c.Messages, message)
				c.textinput.SetValue("")
				// Send the message to the SentMessageChan for broadcasting
				c.SentMessageChan <- message
			}
			return c, nil
		}
	case NewUserMsg:
		user := User{
			Name:  msg.Peer.PeerID,
			Color: generateRandomHexString(),
		}
		c.Users[msg.Peer.PeerID] = user

		// Format the user name with their color using lipgloss
		joinMsg := ChatMessage{
			Type: USER_JOIN,
			From: user.Name,
		}
		c.Messages = append(c.Messages, joinMsg)
	case LeftUserMsg:
		leftMsg := ChatMessage{
			Type: USER_LEFT,
			From: msg.Peer.PeerID,
		}
		c.Messages = append(c.Messages, leftMsg)
		delete(c.Users, msg.Peer.PeerID)
	case MessageUpdate:
		c.Messages = append(c.Messages, ChatMessage{
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
		switch msg.Type {
		case USER_JOIN:
			formattedName := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Users[msg.From].Color)).Render(msg.From)
			sb.WriteString(formattedName + " joined the chat\n")
		case USER_LEFT:
			formattedName := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Users[msg.From].Color)).Render(msg.From)
			sb.WriteString(formattedName + " left the chat\n")
		case USER_MESSAGE:
			formattedName := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Users[msg.From].Color)).Render(msg.From)
			sb.WriteString(formattedName + ": " + msg.Content + "\n")

		case SYSTEM_MESSAGE:
			formattedContent := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8)).Render(msg.Content)
			sb.WriteString(formattedContent + "\n")
		}
	}
	sb.WriteString(c.textinput.View())
	return sb.String()
}
