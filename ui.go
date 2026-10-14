package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type tab int

const (
	tabFiles tab = iota
	tabInfo
	tabDownload
	tabConvert
)

var tabNames = []string{"Files", "Info", "Download", "Convert"}

var (
	activeTab   = lipgloss.NewStyle().Bold(true).Padding(0, 2).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	inactiveTab = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("8"))
	body        = lipgloss.NewStyle().Padding(1, 2)
	help        = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("8"))
)

type model struct {
	tab           tab
	args          []string // positional args passed to the subcommand, e.g. a file or URL
	width, height int
	lang          lang

	probes map[string]*probeEntry // keyed by absolute path

	files files

	// Info panel state.
	infoFiles  []string
	infoOffset int // first visible line

	cv convert
}

func newModel(t tab, args []string) model {
	m := model{tab: t, args: args, lang: detectLang(), probes: map[string]*probeEntry{}, files: newFiles()}
	switch t {
	case tabInfo:
		m.infoFiles = args
	case tabConvert:
		m.cv = newConvert(args)
	}
	return m
}

func (m model) Init() tea.Cmd {
	switch m.tab {
	case tabFiles:
		return requestProbes(m.probes, m.files.mediaPaths())
	case tabInfo:
		return requestProbes(m.probes, m.infoFiles)
	}
	return m.cv.init()
}

// bodyHeight is the number of lines available to a panel.
func (m model) bodyHeight() int {
	return max(m.height-4, 3) // tabs, help line and the body's vertical padding
}

func (m *model) setTab(t tab) tea.Cmd {
	m.tab = t
	if t == tabFiles { // pick up files created meanwhile, e.g. conversion outputs
		m.files.load("")
		return requestProbes(m.probes, m.files.mediaPaths())
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.files.rows = max(m.bodyHeight()-5, 1) // path, column header, counter, blank, help
		m.files.scroll()
	case fileProbedMsg:
		if e := m.probes[msg.key]; e != nil {
			e.p, e.err, e.done = msg.p, msg.err, true
		}
	case cvProbedMsg, cvProgressMsg, cvJobDoneMsg:
		return m, m.cv.update(msg)
	case tea.KeyPressMsg:
		if handled, cmd := m.panelKey(msg.String()); handled {
			return m, cmd
		}
		switch msg.String() {
		case "L":
			if m.lang == langEN {
				m.lang = langZH
			} else {
				m.lang = langEN
			}
		case "q", "ctrl+c", "esc":
			if m.cv.run != nil {
				m.cv.run.stop() // kill ffmpeg and delete its partial output
			}
			return m, tea.Quit
		case "tab":
			return m, m.setTab((m.tab + 1) % tab(len(tabNames)))
		case "shift+tab":
			return m, m.setTab((m.tab + tab(len(tabNames)) - 1) % tab(len(tabNames)))
		case "1", "2", "3", "4":
			return m, m.setTab(tab(msg.String()[0] - '1'))
		}
	}
	return m, nil
}

// panelKey gives the active panel first refusal on a key.
func (m *model) panelKey(key string) (bool, tea.Cmd) {
	switch m.tab {
	case tabFiles:
		handled, action := m.files.handleKey(key)
		switch action {
		case faDirChanged:
			return true, requestProbes(m.probes, m.files.mediaPaths())
		case faInfo:
			m.infoFiles, m.infoOffset = m.files.targets(), 0
			m.tab = tabInfo
			return true, requestProbes(m.probes, m.infoFiles)
		case faConvert:
			return true, m.openConvert(m.files.targets())
		}
		return handled, nil
	case tabInfo:
		return m.scrollInfo(key), nil
	case tabConvert:
		return m.cv.handleKey(key)
	}
	return false, nil
}

// openConvert loads paths into the Convert panel, reusing cached probes.
func (m *model) openConvert(paths []string) tea.Cmd {
	m.tab = tabConvert
	if m.cv.state == cvRunning {
		return nil // never replace a running batch; the user sees its progress
	}
	m.cv = newConvert(paths)
	probes := make([]*probe, len(paths))
	for i, p := range paths {
		e := m.probes[absPath(p)]
		if e == nil || !e.done || e.err != nil {
			return m.cv.init()
		}
		probes[i] = e.p
	}
	return m.cv.update(cvProbedMsg{probes: probes})
}

func (m *model) scrollInfo(key string) bool {
	page := m.bodyHeight() - 1
	switch key {
	case "up", "k":
		m.infoOffset--
	case "down", "j":
		m.infoOffset++
	case "pgup":
		m.infoOffset -= page
	case "pgdown", "space":
		m.infoOffset += page
	case "home", "g":
		m.infoOffset = 0
	case "end", "G":
		m.infoOffset = 1 << 30
	default:
		return false
	}
	lines := strings.Count(m.infoContent(), "\n") + 1
	m.infoOffset = max(0, min(m.infoOffset, lines-page))
	return true
}

func (m model) View() tea.View {
	tabs := make([]string, len(tabNames))
	for i, name := range tabNames {
		style := inactiveTab
		if tab(i) == m.tab {
			style = activeTab
		}
		tabs[i] = style.Render(m.lang.t(name))
	}

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, tabs...),
		body.Width(m.width).Render(m.bodyView()),
		help.Render(m.lang.t("tab/1-4 switch · L 中文 · q quit")),
	))
	v.AltScreen = true
	return v
}

func (m model) bodyView() string {
	switch m.tab {
	case tabFiles:
		return m.files.view(m.lang, m.probes, m.width)
	case tabInfo:
		return m.infoView()
	case tabConvert:
		return m.cv.view(m.lang)
	}
	content := m.lang.t("(not implemented yet)")
	if len(m.args) > 0 {
		content += "\n" + m.lang.t("args: ") + strings.Join(m.args, " ")
	}
	return content
}

// infoContent renders the full details of every Info file, one after another.
func (m model) infoContent() string {
	if len(m.infoFiles) == 0 {
		return m.lang.t("No files: select some in the Files tab, or run mx info <file>...")
	}
	parts := make([]string, len(m.infoFiles))
	for i, f := range m.infoFiles {
		e := m.probes[absPath(f)]
		switch {
		case e == nil || !e.done:
			parts[i] = fmt.Sprintf(m.lang.t("Probing %s …"), f)
		case e.err != nil:
			parts[i] = errStyle.Render(e.err.Error())
		default:
			parts[i] = renderProbe(e.p, m.lang)
		}
	}
	return strings.Join(parts, "\n\n"+dimStyle.Render(strings.Repeat("─", 40))+"\n\n")
}

// infoView shows the window of infoContent starting at infoOffset.
func (m model) infoView() string {
	lines := strings.Split(m.infoContent(), "\n")
	page := m.bodyHeight() - 1
	if len(lines) <= page+1 {
		return strings.Join(lines, "\n")
	}
	start := min(m.infoOffset, len(lines)-page)
	end := start + page
	return strings.Join(lines[start:end], "\n") + "\n" +
		dimStyle.Render(fmt.Sprintf(m.lang.t("↑↓ scroll · %d–%d / %d lines"), start+1, end, len(lines)))
}
