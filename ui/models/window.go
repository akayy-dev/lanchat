package models

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"LANChat/ui"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	sidebarWidth = 24
	inputHeight  = 3
)

// Styles for the UI components
var (
	accentColor = ui.GenerateRandomHexString()

	// Rounded border style for sidebar
	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(accentColor)).
			Padding(0, 1)

	// Rounded border style for text input
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(accentColor)).
			Padding(0, 1)

	// Dimmed style for unfocused text input
	inputStyleUnfocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.ANSIColor(8)).
				Padding(0, 1)

	// Style for the chat area
	chatAreaStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Style for sidebar title
	sidebarTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(accentColor)).
				MarginBottom(1)

	// Style for online indicator
	onlineIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Render("●")

	// Style for timestamp
	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.ANSIColor(8))

	// Style for highlighted/focused message in scroll mode
	focusedMsgStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(accentColor)).
			Padding(0, 1)

	// Style for scroll mode hint
	scrollHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.ANSIColor(8)).
			Italic(true)
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
		width:           80,
		height:          24,
	}
	return model
}

type ChatWindowModel struct {
	Users           map[string]ui.User
	Messages        []ui.ChatMessage
	SentMessageChan chan ui.ChatMessage
	textinput       textinput.Model
	width           int
	height          int
	scrollMode      bool // true when in scroll mode (textinput unfocused)
	focusedMsgIndex int  // index of currently focused message in scroll mode
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
		c.width = msg.Width
		c.height = msg.Height
		// Account for sidebar width, borders, and padding
		chatWidth := msg.Width - sidebarWidth - 6
		if chatWidth < 20 {
			chatWidth = 20
		}
		c.textinput.Width = chatWidth - 4 // Account for input border padding
	case ui.ReceivedChatMessage:
		// FIX: Now uses decoupled ReceivedChatMessage struct instead of chat.TCPMessage.
		// The conversion happens in main.go, keeping UI independent of network layer details.
		c.Messages = append(c.Messages, ui.ChatMessage{
			Type:      ui.USER_MESSAGE,
			Content:   msg.Content,
			From:      msg.From,
			Timestamp: msg.Timestamp,
		})
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return c, tea.Quit
		case "esc":
			// Enter scroll mode - unfocus textinput
			if !c.scrollMode {
				c.scrollMode = true
				c.textinput.Blur()
				// Start at the most recent message
				if len(c.Messages) > 0 {
					c.focusedMsgIndex = len(c.Messages) - 1
				}
			}
			return c, nil
		case "i":
			// Exit scroll mode - refocus textinput
			if c.scrollMode {
				c.scrollMode = false
				c.textinput.Focus()
				return c, textinput.Blink
			}
		case "j":
			// Move down (to newer messages) in scroll mode
			if c.scrollMode && len(c.Messages) > 0 {
				if c.focusedMsgIndex < len(c.Messages)-1 {
					c.focusedMsgIndex++
				}
				return c, nil
			}
		case "k":
			// Move up (to older messages) in scroll mode
			if c.scrollMode && len(c.Messages) > 0 {
				if c.focusedMsgIndex > 0 {
					c.focusedMsgIndex--
				}
				return c, nil
			}
		case "enter":
			// Only process enter when not in scroll mode
			if c.scrollMode {
				return c, nil
			}
			input := strings.TrimSpace(c.textinput.Value())
			if input == "/quit" {
				return c, tea.Quit
			}

			if input != "" {
				message := ui.ChatMessage{
					Type:      ui.USER_MESSAGE,
					From:      "You",
					Content:   c.textinput.Value(),
					Timestamp: time.Now(),
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
			Type:      ui.USER_JOIN,
			From:      user.Name,
			Timestamp: time.Now(),
		}
		c.Messages = append(c.Messages, joinMsg)
	case ui.LeftUserMsg:
		leftMsg := ui.ChatMessage{
			Type:      ui.USER_LEFT,
			From:      msg.Peer.PeerID,
			Timestamp: time.Now(),
		}
		c.Messages = append(c.Messages, leftMsg)
		delete(c.Users, msg.Peer.PeerID)
	case ui.MessageUpdate:
		c.Messages = append(c.Messages, ui.ChatMessage{
			Type:      msg.Type,
			Content:   msg.Content,
			From:      msg.From,
			Timestamp: time.Now(),
		})
	}
	c.textinput, cmd = c.textinput.Update(msg)
	return c, cmd
}

// formatTimestamp formats a timestamp for display
func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return timestampStyle.Render("[" + t.Format("15:04") + "]")
}

// formatMessage formats a single message for display, optionally highlighted
func (c ChatWindowModel) formatMessage(msg ui.ChatMessage, index int, maxWidth int) string {
	timestamp := formatTimestamp(msg.Timestamp)
	var line string

	switch msg.Type {
	case ui.USER_JOIN:
		formattedName := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Users[msg.From].Color)).Render(msg.From)
		line = timestamp + " " + formattedName + " joined the chat"
	case ui.USER_LEFT:
		formattedName := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Users[msg.From].Color)).Render(msg.From)
		line = timestamp + " " + formattedName + " left the chat"
	case ui.USER_MESSAGE:
		var formattedName string
		if msg.From == "You" {
			formattedName = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accentColor)).Render(msg.From)
		} else if user, ok := c.Users[msg.From]; ok {
			formattedName = lipgloss.NewStyle().Foreground(lipgloss.Color(user.Color)).Render(msg.From)
		} else {
			formattedName = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(msg.From)
		}
		line = timestamp + " " + formattedName + ": " + msg.Content
	case ui.SYSTEM_MESSAGE:
		formattedContent := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8)).Render(msg.Content)
		line = timestamp + " " + formattedContent
	}

	// If in scroll mode and this is the focused message, highlight it
	if c.scrollMode && index == c.focusedMsgIndex {
		return focusedMsgStyle.Width(maxWidth - 4).Render(line)
	}

	return line
}

func (c ChatWindowModel) View() string {
	// Calculate dimensions
	// Sidebar takes sidebarWidth + 2 (for border)
	// Left panel gets the rest
	chatWidth := c.width - sidebarWidth - 2
	if chatWidth < 20 {
		chatWidth = 20
	}

	// Input box: 1 line content + 2 lines border (top/bottom) = 3 lines total
	inputBoxHeight := 3
	// Chat area gets remaining height
	chatHeight := c.height - inputBoxHeight

	// Build chat messages area
	var chatContent strings.Builder
	if len(c.Messages) == 0 {
		chatContent.WriteString("Waiting for peers to join...\n")
	}
	for i, msg := range c.Messages {
		chatContent.WriteString(c.formatMessage(msg, i, chatWidth-2) + "\n")
	}

	// Style the chat area - no border, just padding
	// Height is content height (total - input)
	chatArea := chatAreaStyle.
		Width(chatWidth).
		Height(chatHeight).
		Render(chatContent.String())

	// Style the text input with rounded border
	// Use different style based on scroll mode
	var styledInput string
	if c.scrollMode {
		// Show hint and dimmed input when in scroll mode
		hint := scrollHintStyle.Render("-- SCROLL MODE -- press <i> to type, j/k to navigate")
		styledInput = inputStyleUnfocused.
			Width(chatWidth - 4).
			Render(hint)
	} else {
		styledInput = inputStyle.
			Width(chatWidth - 4).
			Render(c.textinput.View())
	}

	// Combine chat area and input vertically
	leftPanel := lipgloss.JoinVertical(lipgloss.Left, chatArea, styledInput)

	// Build sidebar with users list - full terminal height
	sidebar := c.renderSidebar(c.height)

	// Join left panel and sidebar horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, sidebar)
}

// renderSidebar renders the users sidebar with rounded corners
func (c ChatWindowModel) renderSidebar(height int) string {
	var sb strings.Builder

	// Title
	sb.WriteString(sidebarTitleStyle.Render(fmt.Sprintf("Online Users (%d)", len(c.Users))))
	sb.WriteString("\n")

	// Get sorted list of users for consistent display
	userNames := make([]string, 0, len(c.Users))
	for name := range c.Users {
		userNames = append(userNames, name)
	}
	sort.Strings(userNames)

	// Render each user
	for _, name := range userNames {
		user := c.Users[name]
		userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(user.Color))
		// Truncate long names to fit sidebar
		displayName := name
		maxLen := sidebarWidth - 6
		if len(displayName) > maxLen {
			displayName = displayName[:maxLen-3] + "..."
		}
		sb.WriteString(onlineIndicator + " " + userStyle.Render(displayName) + "\n")
	}

	// If no users, show placeholder
	if len(c.Users) == 0 {
		noUsersStyle := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8)).Italic(true)
		sb.WriteString(noUsersStyle.Render("No users online"))
	}

	// Apply sidebar style with rounded border
	// Border adds 2 lines (top + bottom), so content height is height - 2
	// But we also have padding (0, 1) which doesn't add vertical space
	return sidebarStyle.
		Width(sidebarWidth).
		Height(height - 2).
		Render(sb.String())
}
