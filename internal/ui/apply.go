package ui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hidekingerz/mado/internal/config"
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

	var bgCmd tea.Cmd
	if next.Theme != prev.Theme {
		m.styles = newStyles(next.Theme)
		m.style = resolveStyle(next.Theme.Style, m.darkBG)
		m.sourceStyle = chromaStyleName(m.style, next.Theme.SourceStyle, m.darkBG)
		if needsBackground(next.Theme) && !m.bgKnown {
			// The program is running, so ask through Bubble Tea
			// rather than reading the tty under it; the answer
			// arrives as a BackgroundColorMsg (setBackground).
			bgCmd = tea.RequestBackgroundColor
		}
		if next.Theme.DefaultMode != prev.Theme.DefaultMode {
			m.mode = modeReader
			if next.Theme.DefaultMode == "source" {
				m.mode = modeSource
			}
		}
	}
	// Drawing mermaid blocks happens at render time, so switching it
	// is a re-render, like a theme change.
	if next.Theme != prev.Theme || next.General.Mermaid != prev.General.Mermaid {
		for _, t := range m.tabs {
			t.rendered = 0
		}
		m.ensureRendered(m.activeTab())
	}
	// treeOpts fields are compared and overwritten individually, not
	// wholesale on any Sidebar change: ShowAllFiles and ShowHidden can
	// also be flipped at runtime by the "a" and "." keys without touching
	// m.cfg, and a config change to a sibling field like Width must not
	// revert that.
	treeChanged := false
	if next.Sidebar.ShowAllFiles != prev.Sidebar.ShowAllFiles {
		m.treeOpts.ShowAllFiles = next.Sidebar.ShowAllFiles
		treeChanged = true
	}
	if next.Sidebar.ShowHidden != prev.Sidebar.ShowHidden {
		m.treeOpts.ShowHidden = next.Sidebar.ShowHidden
		treeChanged = true
	}
	if !slices.Equal(next.Search.Exclude, prev.Search.Exclude) {
		m.treeOpts.Exclude = next.Search.Exclude
		treeChanged = true
	}
	if treeChanged {
		m.reloadTree()
	}
	if treeChanged || next.Sidebar.Width != prev.Sidebar.Width {
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
	return tea.Batch(cmd, bgCmd)
}

// setBackground records the terminal's background and, when the style
// depends on it, re-renders the tabs in the style it now resolves to.
func (m *Model) setBackground(dark bool) {
	m.darkBG, m.bgKnown = dark, true
	style := resolveStyle(m.cfg.Theme.Style, dark)
	source := chromaStyleName(style, m.cfg.Theme.SourceStyle, dark)
	if style == m.style && source == m.sourceStyle {
		return
	}
	m.style, m.sourceStyle = style, source
	for _, t := range m.tabs {
		t.rendered = 0
	}
	m.ensureRendered(m.activeTab())
}

// needsBackground reports whether theme resolves differently on a dark
// and a light terminal: the auto style, or a custom style file without
// a source_style, whose highlighting follows the background.
func needsBackground(theme config.Theme) bool {
	switch theme.Style {
	case "", "auto":
		return true
	case "dark", "light", "dracula", "notty", "ascii":
		return false
	}
	return theme.SourceStyle == ""
}
