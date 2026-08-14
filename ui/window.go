package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type ChatWindowModel struct {
}

func (c ChatWindowModel) Init() tea.Cmd {
}

func (c ChatWindowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return c, tea.Quit

		}
	}

}

func (c ChatWindowModel) View() tea.View {

}
