package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/hidekingerz/mado/internal/termsafe"
)

const maxTabTitleWidth = 20

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	var b strings.Builder
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	var main string
	content := m.renderContent()
	if m.sidebar {
		main = lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(), content)
	} else {
		main = content
	}
	b.WriteString(main)
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	return b.String()
}

// ── tab bar ─────────────────────────────────────────────────────────

// tabLabel returns the printable label for tab i. Regions computed in
// computeTabRegions must match this layout exactly.
func (m Model) tabLabel(i int) string {
	return " " + truncate(m.tabs[i].title, maxTabTitleWidth) + " ✕ "
}

// computeTabRegions records the column span of every tab for mouse
// hit-testing. Called whenever tabs or geometry change.
func (m *Model) computeTabRegions() {
	m.tabRegions = m.tabRegions[:0]
	x := 0
	for i := range m.tabs {
		label := m.tabLabel(i)
		w := lipgloss.Width(label)
		m.tabRegions = append(m.tabRegions, tabRegion{
			start:  x,
			end:    x + w,
			closeX: x + w - 2,
		})
		x += w
	}
}

func (m Model) renderTabBar() string {
	if len(m.tabs) == 0 {
		title := m.styles.accent.Bold(true).Render(" mado ") +
			m.styles.dimmed.Render(" tui markdown viewer")
		return padLine(title, m.width)
	}
	var parts []string
	for i := range m.tabs {
		label := m.tabLabel(i)
		if i == m.active {
			parts = append(parts, m.styles.tabActive.Render(label))
		} else {
			parts = append(parts, m.styles.tabInactive.Render(label))
		}
	}
	return padLine(strings.Join(parts, ""), m.width)
}

// ── sidebar ─────────────────────────────────────────────────────────

func (m Model) renderSidebar() string {
	innerW := m.sidebarWidth() - 2
	innerH := m.treeInnerHeight()

	var rows []string
	for row := 0; row < innerH; row++ {
		idx := m.treeOff + row
		if idx >= len(m.items) {
			rows = append(rows, strings.Repeat(" ", innerW))
			continue
		}
		it := m.items[idx]
		icon := "•"
		if it.Node.IsDir {
			if it.Node.Expanded {
				icon = "▾"
			} else {
				icon = "▸"
			}
		}
		// A file name is chosen by whoever created the file, so it
		// gets the same treatment as file contents.
		name := termsafe.String(it.Node.Name)
		if it.Node.IsDir {
			name += "/"
		}
		text := strings.Repeat("  ", it.Depth) + icon + " " + name
		text = truncate(text, innerW)
		text += strings.Repeat(" ", innerW-lipgloss.Width(text))

		switch {
		case idx == m.cursor && m.focus == focusSidebar:
			text = m.styles.selection.Render(text)
		case idx == m.cursor:
			text = m.styles.cursor.Render(text)
		case it.Node.IsDir:
			text = m.styles.dir.Render(text)
		case hasBinaryExt(it.Node.Path):
			text = m.styles.dimmed.Render(text)
		default:
			text = m.styles.file.Render(text)
		}
		rows = append(rows, text)
	}

	borderColor := m.cfg.Theme.BorderColor
	if m.focus == focusSidebar {
		borderColor = m.cfg.Theme.AccentColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(innerW).
		Height(innerH).
		Render(strings.Join(rows, "\n"))
}

// ── content ─────────────────────────────────────────────────────────

func (m Model) renderContent() string {
	innerW := m.contentInnerWidth()
	innerH := m.contentInnerHeight()

	var body string
	switch {
	case m.showHelp:
		body = m.renderHelp(innerW, innerH)
	case len(m.tabs) == 0:
		body = m.renderWelcome(innerW, innerH)
	default:
		body = m.tabs[m.active].vp.View()
	}

	borderColor := m.cfg.Theme.BorderColor
	if m.focus == focusContent {
		borderColor = m.cfg.Theme.AccentColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(innerW).
		Height(innerH).
		Render(body)
}

func (m Model) renderWelcome(w, h int) string {
	msg := m.styles.accent.Bold(true).Render("mado") + "\n\n" +
		m.styles.dimmed.Render("Select a markdown file in the sidebar,\nor click one with the mouse.\n\nPress "+m.keys.Help.Help().Key+" for help.")
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg)
}

func (m Model) renderHelp(w, h int) string {
	k := m.keys
	rows := [][2]string{
		{k.Up.Help().Key, k.Up.Help().Desc},
		{k.Down.Help().Key, k.Down.Help().Desc},
		{k.Open.Help().Key, k.Open.Help().Desc},
		{k.Back.Help().Key, k.Back.Help().Desc},
		{k.NextTab.Help().Key, k.NextTab.Help().Desc},
		{k.PrevTab.Help().Key, k.PrevTab.Help().Desc},
		{k.CloseTab.Help().Key, k.CloseTab.Help().Desc},
		{k.ToggleSidebar.Help().Key, k.ToggleSidebar.Help().Desc},
		{k.HalfPageDown.Help().Key, k.HalfPageDown.Help().Desc},
		{k.HalfPageUp.Help().Key, k.HalfPageUp.Help().Desc},
		{k.Top.Help().Key, k.Top.Help().Desc},
		{k.Bottom.Help().Key, k.Bottom.Help().Desc},
		{k.Reload.Help().Key, k.Reload.Help().Desc},
		{k.ToggleMode.Help().Key, k.ToggleMode.Help().Desc},
		{k.ToggleAllFiles.Help().Key, k.ToggleAllFiles.Help().Desc},
		{k.Help.Help().Key, k.Help.Help().Desc},
		{k.Quit.Help().Key, k.Quit.Help().Desc},
	}
	keyW := 0
	for _, r := range rows {
		if lw := lipgloss.Width(r[0]); lw > keyW {
			keyW = lw
		}
	}
	var b strings.Builder
	b.WriteString(m.styles.accent.Bold(true).Render("Keyboard shortcuts"))
	b.WriteString("\n\n")
	for _, r := range rows {
		pad := strings.Repeat(" ", keyW-lipgloss.Width(r[0]))
		b.WriteString("  " + m.styles.accent.Render(r[0]) + pad + "  " + r[1] + "\n")
	}
	b.WriteString("\n" + m.styles.dimmed.Render("Mouse: click to select/open, wheel to scroll,\nclick a tab to switch, ✕ to close."))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, b.String())
}

// ── status bar ──────────────────────────────────────────────────────

func (m Model) renderStatusBar() string {
	left := " "
	if m.watcher != nil {
		left += "[WATCH] "
	}
	if t := m.activeTab(); t != nil {
		left += "[" + m.mode.String() + "]  " + t.path
		left += fmt.Sprintf("  %3.0f%%", t.vp.ScrollPercent()*100)
		if t.renderErr != "" {
			left += "  [render error: " + t.renderErr + "]"
		}
	} else {
		left += m.root.Path
	}
	if m.statusMsg != "" {
		left += "  ⚠ " + m.statusMsg
	}
	// left is built from paths and error messages, both of which carry
	// names from disk.
	left = termsafe.String(left)
	right := m.keys.Help.Help().Key + " help │ " + m.keys.Quit.Help().Key + " quit "

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		left = truncate(left, maxInt(m.width-lipgloss.Width(right)-1, 0))
		gap = maxInt(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	}
	line := left + strings.Repeat(" ", gap) + right
	return m.styles.status.Render(padLine(line, m.width))
}

func padLine(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}
