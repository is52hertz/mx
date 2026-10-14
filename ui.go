package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type tab int

const (
	tabInfo tab = iota
	tabDownload
	tabConvert
)

var tabNames = []string{"Info", "Download", "Convert"}

var (
	activeTab   = lipgloss.NewStyle().Bold(true).Padding(0, 2).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	inactiveTab = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("8"))
	body        = lipgloss.NewStyle().Padding(1, 2)
	help        = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("8"))
)

type model struct {
	tab  tab
	args []string // positional args passed to the subcommand, e.g. a file or URL
}

func newModel(t tab, args []string) model {
	return model{tab: t, args: args}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "right", "l":
			m.tab = (m.tab + 1) % tab(len(tabNames))
		case "shift+tab", "left", "h":
			m.tab = (m.tab + tab(len(tabNames)) - 1) % tab(len(tabNames))
		case "1", "2", "3":
			m.tab = tab(key.String()[0] - '1')
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	tabs := make([]string, len(tabNames))
	for i, name := range tabNames {
		style := inactiveTab
		if tab(i) == m.tab {
			style = activeTab
		}
		tabs[i] = style.Render(name)
	}

	content := "(not implemented yet)"
	if len(m.args) > 0 {
		content += "\nargs: " + strings.Join(m.args, " ")
	}

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, tabs...),
		body.Render(content),
		help.Render("tab/1-3 switch · q quit"),
	))
	v.AltScreen = true
	return v
}
