package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/filetree"
	"github.com/hidekingerz/mado/internal/watch"
)

// WithConfigPath tells the model where the settings panel saves. An
// empty path keeps changes for the session only.
func (m Model) WithConfigPath(path string) Model {
	m.configPath = path
	return m
}

// newStyles derives every lipgloss style the views use from the theme.
func newStyles(th config.Theme) styles {
	accent := lipgloss.Color(th.AccentColor)
	return styles{
		accent:    lipgloss.NewStyle().Foreground(accent),
		border:    lipgloss.NewStyle().Foreground(lipgloss.Color(th.BorderColor)),
		selection: lipgloss.NewStyle().Foreground(lipgloss.Color(th.SelectionFg)).Background(lipgloss.Color(th.SelectionBg)).Bold(true),
		cursor:    lipgloss.NewStyle().Foreground(lipgloss.Color(th.FileColor)).Bold(true),
		dir:       lipgloss.NewStyle().Foreground(lipgloss.Color(th.DirColor)).Bold(true),
		file:      lipgloss.NewStyle().Foreground(lipgloss.Color(th.FileColor)),
		dimmed:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		status:    lipgloss.NewStyle().Foreground(lipgloss.Color(th.StatusFg)).Background(lipgloss.Color(th.StatusBg)),
		tabActive: lipgloss.NewStyle().Foreground(lipgloss.Color(th.SelectionFg)).Background(accent).Bold(true),
		tabInactive: lipgloss.NewStyle().Foreground(lipgloss.Color(th.StatusFg)).
			Background(lipgloss.Color(th.StatusBg)),
	}
}

// applyConfig makes next the running configuration, re-deriving what
// New derived at startup from whichever parts changed: theme fields
// rebuild the styles and re-render every tab, sidebar fields reload
// the tree and the layout, watch starts or stops the watcher, and the
// key map is always rebuilt. The command, if any, waits on a watcher
// that was just started.
func (m *Model) applyConfig(next config.Config) tea.Cmd {
	prev := m.cfg
	m.cfg = next
	m.keys = newKeyMap(next.Keys)

	if next.Theme != prev.Theme {
		m.styles = newStyles(next.Theme)
		m.style = resolveStyle(next.Theme.Style, m.darkBG)
		m.sourceStyle = chromaStyleName(m.style, next.Theme.SourceStyle, m.darkBG)
		if next.Theme.DefaultMode != prev.Theme.DefaultMode {
			m.mode = modeReader
			if next.Theme.DefaultMode == "source" {
				m.mode = modeSource
			}
		}
		for _, t := range m.tabs {
			t.rendered = 0
		}
		m.ensureRendered(m.activeTab())
	}
	if next.Sidebar != prev.Sidebar {
		m.treeOpts = filetree.Options{
			ShowAllFiles: next.Sidebar.ShowAllFiles,
			ShowHidden:   next.Sidebar.ShowHidden,
		}
		m.reloadTree()
		m.layoutTabs()
	}
	var cmd tea.Cmd
	switch {
	case next.General.Watch && m.watcher == nil:
		w, err := watch.New(watch.DefaultDebounce)
		if err != nil {
			m.statusMsg = "watch disabled: " + err.Error()
		} else {
			m.watcher = w
			m.syncWatch()
			cmd = waitForChange(w)
		}
	case !next.General.Watch && m.watcher != nil:
		m.watcher.Close()
		m.watcher = nil
	}
	return cmd
}
