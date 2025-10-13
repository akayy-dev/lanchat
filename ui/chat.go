package ui

import (
	"LANChat/chat"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Notifies that a new message has been added to the conversation.
type NewMessageMsg chat.Message

type ChatWindow struct {
	Messages  []chat.Message
	Messenger chat.MessageService
	User      chat.User
	textfield textinput.Model
	Width     int
	Height    int
	sb        strings.Builder
}

func (c *ChatWindow) Init() tea.Cmd {
	c.Messenger.Send(chat.Message{
		Type: chat.USERJOIN,
		User: c.User,
	})

	c.textfield = textinput.New()
	c.textfield.Placeholder = "Send a message"
	c.textfield.Focus()
	return tea.Batch(textinput.Blink, tea.WindowSize())
}

func (c *ChatWindow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.QuitMsg:
		c.Messenger.Send(chat.Message{
			User: c.User,
			Type: chat.USERLEAVE,
		})
	case NewMessageMsg:
		c.Messages = append(c.Messages, chat.Message(msg))
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			c.Messenger.Send(chat.Message{
				User: c.User,
				Type: chat.USERLEAVE,
			})
			return c, tea.Quit
		case "enter":
			if c.textfield.Value() == "" {
				break
			}

			if c.textfield.Value() == "/quit" {
				c.Messenger.Send(chat.Message{
					User: c.User,
					Type: chat.USERLEAVE,
				})
				return c, tea.Quit
			}

			c.Messenger.Send(
				chat.Message{
					User:    c.User,
					Type:    chat.MESSAGESEND,
					Content: c.textfield.Value(),
					Time:    time.Now(),
				},
			)
			c.textfield.SetValue("")

		}
	case tea.WindowSizeMsg:
		c.Width = msg.Width
		c.Height = msg.Height
		c.textfield.Width = c.Width
	}
	c.textfield, cmd = c.textfield.Update(msg)
	return c, cmd
}

func (c *ChatWindow) View() string {
	c.sb = strings.Builder{}
	for _, msg := range c.Messages {
		formattedUserID := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(msg.User.Color)).Render(fmt.Sprintf("%s@%s", msg.User.Name, msg.User.Hostname))
		messageFormat := lipgloss.NewStyle().Width(c.Width)
		if msg.Type == chat.USERJOIN {
			c.sb.WriteString(
				fmt.Sprintf(
					"%s has joined the chat\n",
					formattedUserID,
				),
			)
		}
		if msg.Type == chat.USERLEAVE {
			c.sb.WriteString(
				fmt.Sprintf(
					"%s has left the chat\n",
					formattedUserID,
				),
			)
		}
		if msg.Type == chat.MESSAGESEND {
			c.sb.WriteString(
				fmt.Sprintf(
					"(%s) %s: %s\n", msg.Time.Format("3:04 PM"), formattedUserID, messageFormat.Render(msg.Content),
				),
			)
		}
	}
	c.sb.WriteString(c.textfield.View())
	return c.sb.String()
}
