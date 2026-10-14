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
	tab   tab
	args  []string // positional args passed to the subcommand, e.g. a file or URL
	width int

	// Info panel state.
	infoFile string
	probe    *probe
	probeErr error
}

func newModel(t tab, args []string) model {
	m := model{tab: t, args: args}
	if t == tabInfo && len(args) > 0 {
		m.infoFile = args[0]
	}
	return m
}

func (m model) Init() tea.Cmd {
	if m.infoFile != "" {
		return runProbe(m.infoFile)
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case probeMsg:
		m.probe, m.probeErr = msg.probe, msg.err
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "right", "l":
			m.tab = (m.tab + 1) % tab(len(tabNames))
		case "shift+tab", "left", "h":
			m.tab = (m.tab + tab(len(tabNames)) - 1) % tab(len(tabNames))
		case "1", "2", "3":
			m.tab = tab(msg.String()[0] - '1')
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

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, tabs...),
		body.Width(m.width).Render(m.bodyView()),
		help.Render("tab/1-3 switch · q quit"),
	))
	v.AltScreen = true
	return v
}

func (m model) bodyView() string {
	if m.tab != tabInfo {
		content := "(not implemented yet)"
		if len(m.args) > 0 {
			content += "\nargs: " + strings.Join(m.args, " ")
		}
		return content
	}
	switch {
	case m.infoFile == "":
		return "Usage: mx info <file>"
	case m.probeErr != nil:
		return errStyle.Render("ffprobe: " + m.probeErr.Error())
	case m.probe == nil:
		return "Probing " + m.infoFile + " …"
	}
	return renderProbe(m.probe)
}
