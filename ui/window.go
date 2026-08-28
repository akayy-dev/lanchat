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

func (c ChatWindowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case InitMsg:
		// Handle any init logic here.
		return c, nil
	case tea.WindowSizeMsg:
		c.textinput.Width = msg.Width
	case ReceivedChatMessage:
		// FIX: Now uses decoupled ReceivedChatMessage struct instead of chat.TCPMessage.
		// The conversion happens in main.go, keeping UI independent of network layer details.
		c.Messages = append(c.Messages, ChatMessage{
			Type:    USER_MESSAGE,
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
				message := ChatMessage{
					Type:    USER_MESSAGE,
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
					c.Messages = append(c.Messages, ChatMessage{
						Type:    SYSTEM_MESSAGE,
						Content: "Warning: Send buffer full, message may not reach all peers",
					})
				}
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
			// FIX: Handle "You" specially since it's not in the Users map.
			// "You" refers to the local user whose messages are displayed locally without
			// going through the Users map (which only tracks remote peers).
			var formattedName string
			if msg.From == "You" {
				// Use bold cyan for local user's messages
				formattedName = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FFFF")).Render(msg.From)
			} else if user, ok := c.Users[msg.From]; ok {
				formattedName = lipgloss.NewStyle().Foreground(lipgloss.Color(user.Color)).Render(msg.From)
			} else {
				// Fallback for unknown users (shouldn't happen, but defensive)
				formattedName = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(msg.From)
			}
			sb.WriteString(formattedName + ": " + msg.Content + "\n")

		case SYSTEM_MESSAGE:
			formattedContent := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8)).Render(msg.Content)
			sb.WriteString(formattedContent + "\n")
		}
	}
	sb.WriteString(c.textinput.View())
	return sb.String()
}
