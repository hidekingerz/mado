package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hidekingerz/mado/internal/filetree"
	"github.com/hidekingerz/mado/internal/termsafe"
)

const maxTabTitleWidth = 20

// View implements tea.Model. The alternate screen and mouse reporting
// are declared here, on every frame, rather than requested once as
// program options: that is how Bubble Tea v2 is told what state the
// terminal should be in.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// render draws the whole screen.
func (m Model) render() string {
	if m.width == 0 || m.height == 0 {
		return m.text.Loading
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

// tabLabel returns the printable label for tab i.
func (m Model) tabLabel(i int) string {
	return " " + truncate(m.tabs[i].title, maxTabTitleWidth) + " ✕ "
}

// tabBarLayout is the tab bar as drawn. When the tabs are wider than
// the terminal it shows a run of them that always includes the active
// one, with markers counting the tabs hidden on each side. Drawing and
// mouse hit-testing both use it, so a click lands on the tab under the
// pointer.
type tabBarLayout struct {
	first, last int         // the tabs shown, inclusive
	left, right string      // "‹ 3 " and " 2 ›", or "" when none are hidden
	rightStart  int         // column where the right marker begins
	regions     []tabRegion // screen spans of tabs first..last
}

func (m Model) layoutTabBar() tabBarLayout {
	n := len(m.tabs)
	l := tabBarLayout{last: n - 1}
	if n == 0 {
		return l
	}
	widths := make([]int, n)
	total := 0
	for i := range m.tabs {
		widths[i] = lipgloss.Width(m.tabLabel(i))
		total += widths[i]
	}
	if m.width > 0 && total > m.width {
		avail := m.width - 2*(len(strconv.Itoa(n))+3) // room left by both markers
		// Keep the bar anchored at the first tab while the active one
		// fits from there; otherwise end the run at the active tab and
		// fill what room is left with the tabs on either side.
		l.first, l.last = 0, -1
		used := 0
		for l.last+1 < n && used+widths[l.last+1] <= avail {
			l.last++
			used += widths[l.last]
		}
		if l.last < m.active {
			l.first, l.last, used = m.active, m.active, widths[m.active]
			for l.first > 0 && used+widths[l.first-1] <= avail {
				l.first--
				used += widths[l.first]
			}
			for l.last+1 < n && used+widths[l.last+1] <= avail {
				l.last++
				used += widths[l.last]
			}
		}
		if l.first > 0 {
			l.left = fmt.Sprintf("‹ %d ", l.first)
		}
		if l.last < n-1 {
			l.right = fmt.Sprintf(" %d ›", n-1-l.last)
			l.rightStart = m.width - lipgloss.Width(l.right)
		}
	}
	x := lipgloss.Width(l.left)
	for i := l.first; i <= l.last; i++ {
		w := widths[i]
		l.regions = append(l.regions, tabRegion{index: i, start: x, end: x + w, closeX: x + w - 2})
		x += w
	}
	return l
}

// tabRegions returns the column span of every tab on screen.
func (m Model) tabRegions() []tabRegion {
	return m.layoutTabBar().regions
}

func (m Model) renderTabBar() string {
	if len(m.tabs) == 0 {
		title := m.styles.accent.Bold(true).Render(" mado ") +
			m.styles.dimmed.Render(m.text.Tagline)
		return padLine(title, m.width)
	}
	l := m.layoutTabBar()
	var b strings.Builder
	b.WriteString(m.styles.dimmed.Render(l.left))
	for i := l.first; i <= l.last; i++ {
		label := m.tabLabel(i)
		if i == m.active {
			b.WriteString(m.styles.tabActive.Render(label))
		} else {
			b.WriteString(m.styles.tabInactive.Render(label))
		}
	}
	line := b.String()
	if l.right != "" {
		line = padLine(line, l.rightStart) + m.styles.dimmed.Render(l.right)
	}
	// The active tab alone can be wider than a very narrow terminal.
	return padLine(clip(line, m.width), m.width)
}

// ── sidebar ─────────────────────────────────────────────────────────

func (m Model) renderSidebar() string {
	innerW := m.sidebarWidth() - 2
	innerH := m.treeInnerHeight()

	var rows []string
	for row := 0; row < innerH; row++ {
		idx := m.treeOff + row
		if row == 0 && len(m.items) == 0 {
			// An empty tree says why; the content pane has the keys.
			text := truncate(" "+m.emptyTreeTitle(), innerW)
			rows = append(rows, m.styles.dimmed.Render(text+strings.Repeat(" ", innerW-lipgloss.Width(text))))
			continue
		}
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
		case idx == m.cursor && m.focus == focusSidebar && !m.search.open && !m.links.open && !m.settings.open:
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
	if m.focus == focusSidebar && !m.search.open && !m.links.open && !m.settings.open {
		borderColor = m.cfg.Theme.AccentColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(innerW + 2). // Lip Gloss v2 sizes include the border
		Height(innerH + 2).
		Render(strings.Join(rows, "\n"))
}

// ── content ─────────────────────────────────────────────────────────

func (m Model) renderContent() string {
	innerW := m.contentInnerWidth()
	innerH := m.contentInnerHeight()

	var body string
	switch {
	case m.settings.open:
		body = m.renderSettings(innerW, innerH)
	case m.search.open:
		body = m.renderSearch(innerW, innerH)
	case m.links.open:
		body = m.renderLinks(innerW, innerH)
	case m.showHelp:
		body = m.renderHelp(innerW, innerH)
	case len(m.tabs) == 0:
		body = m.renderWelcome(innerW, innerH)
	default:
		body = m.tabs[m.active].vp.View()
	}

	borderColor := m.cfg.Theme.BorderColor
	if m.focus == focusContent || m.search.open || m.links.open || m.settings.open {
		borderColor = m.cfg.Theme.AccentColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(innerW + 2). // Lip Gloss v2 sizes include the border
		Height(innerH + 2).
		Render(body)
}

func (m Model) renderWelcome(w, h int) string {
	var text string
	var keys [][2]string
	if len(m.items) == 0 {
		// Nothing in the sidebar to select: say why, and offer the
		// filters that would show more (#44).
		text = m.emptyTreeTitle()
		if !m.treeOpts.ShowAllFiles {
			keys = append(keys, [2]string{m.keys.ToggleAllFiles.Help().Key, m.text.ShowAllFiles})
		}
		if !m.treeOpts.ShowHidden {
			keys = append(keys, [2]string{m.keys.ToggleHidden.Help().Key, m.text.ShowDotfiles})
		}
	} else {
		text = m.text.Welcome
		keys = [][2]string{
			{m.keys.Search.Help().Key, m.keys.Search.Help().Desc},
			{m.keys.SearchContent.Help().Key, m.keys.SearchContent.Help().Desc},
		}
	}
	keys = append(keys, [2]string{m.keys.Help.Help().Key, m.text.Help})
	keyW := 0
	for _, k := range keys {
		keyW = maxInt(keyW, lipgloss.Width(k[0]))
	}
	var b strings.Builder
	for _, k := range keys {
		b.WriteString("\n" + m.styles.accent.Render(padLine(k[0], keyW)) + "  " + m.styles.dimmed.Render(k[1]))
	}
	keyBlock := lipgloss.NewStyle().Align(lipgloss.Left).Render(strings.TrimPrefix(b.String(), "\n"))
	msg := m.styles.accent.Bold(true).Render("mado") + "\n\n" +
		m.styles.dimmed.Render(text) + "\n\n" + keyBlock
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg)
}

// emptyTreeTitle says why the sidebar has no rows, from the filters
// that are on.
func (m Model) emptyTreeTitle() string {
	switch {
	case !m.treeOpts.ShowAllFiles:
		return m.text.EmptyNoMarkdown
	case !m.treeOpts.ShowHidden:
		return m.text.EmptyNoFiles
	default:
		return m.text.EmptyNothing
	}
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
		{k.ToggleHidden.Help().Key, k.ToggleHidden.Help().Desc},
		{k.ToggleLineNums.Help().Key, k.ToggleLineNums.Help().Desc},
		{k.CopyPath.Help().Key, k.CopyPath.Help().Desc},
		{k.Links.Help().Key, k.Links.Help().Desc},
		{k.Search.Help().Key, k.Search.Help().Desc},
		{k.SearchContent.Help().Key, k.SearchContent.Help().Desc},
		{k.Settings.Help().Key, k.Settings.Help().Desc},
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
	b.WriteString(m.styles.accent.Bold(true).Render(m.text.HelpTitle))
	b.WriteString("\n\n")
	for _, r := range rows {
		pad := strings.Repeat(" ", keyW-lipgloss.Width(r[0]))
		b.WriteString("  " + m.styles.accent.Render(r[0]) + pad + "  " + r[1] + "\n")
	}
	b.WriteString("\n" + m.styles.dimmed.Render(m.text.HelpSearch))
	b.WriteString("\n\n" + m.styles.dimmed.Render(m.text.HelpMouse))
	// Place centers each line of a multi-line string on its own, which
	// would scatter the key column; padding every line to the block's
	// width first makes the block move as one.
	block := lipgloss.NewStyle().Align(lipgloss.Left).Render(b.String())
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, block)
}

// ── status bar ──────────────────────────────────────────────────────

func (m Model) renderStatusBar() string {
	// The bar is built from three parts: tags on the left that always
	// stay, a path that is the first thing to give way when the pane is
	// narrow, and details after it (scroll percent, errors) that should
	// outlast the path (#42).
	prefix := " "
	if m.watcher != nil {
		prefix += "[WATCH] "
	}
	var path, suffix string
	if m.settings.open {
		prefix += "[SETTINGS]  "
		switch {
		case m.statusMsg != "":
			// A long config path would otherwise push the reason for a
			// rejected or unsaved change off the edge of the bar; the
			// panel's own title row already shows the path, so leave
			// it out here and show the reason instead.
			suffix = "⚠ " + m.statusMsg
		case m.configPath != "":
			path = tildePath(m.configPath)
		default:
			suffix = m.text.NotSaved
		}
	} else {
		if m.search.open {
			// The match count is in the panel's header row already.
			prefix += "[SEARCH " + strings.ToUpper(m.search.target.String()) + "]  "
			path = tildePath(m.root.Path)
		} else if t := m.activeTab(); t != nil && m.links.open {
			prefix += "[LINKS]  "
			path = m.displayPath(t.path)
		} else if t := m.activeTab(); t != nil {
			label := m.mode.String()
			if !filetree.IsMarkdown(t.path) {
				// Plain text is shown as is in either mode.
				label = "TEXT"
			}
			prefix += "[" + label + "]  "
			path = m.displayPath(t.path)
			suffix = fmt.Sprintf("  %3.0f%%", t.vp.ScrollPercent()*100)
			if t.renderErr != "" {
				suffix += "  " + fmt.Sprintf(m.text.RenderErrorF, t.renderErr)
			}
		} else {
			path = tildePath(m.root.Path)
		}
		if m.statusMsg != "" {
			suffix += "  ⚠ " + m.statusMsg
		}
		if m.notice != "" {
			suffix += "  " + m.notice
		}
	}
	// path and suffix carry names from disk and error messages.
	path = termsafe.String(path)
	suffix = termsafe.String(suffix)
	right := m.keys.Help.Help().Key + " " + m.text.BarHelp + " │ " + m.keys.Quit.Help().Key + " " + m.text.BarQuit + " "
	if m.search.open {
		right = fmt.Sprintf(m.text.SearchBarHintF, m.targetName(m.search.target.Toggle()))
	}
	if m.links.open {
		right = m.text.LinksBarHint
	}
	if m.settings.open {
		right = m.settingsHint()
	}

	room := m.width - lipgloss.Width(right) - 1 - lipgloss.Width(prefix) - lipgloss.Width(suffix)
	if lipgloss.Width(path) > room {
		// Keep the end of the path: the file name says more than the
		// directories above it.
		path = truncateLeft(path, maxInt(room, 0))
	}
	left := prefix + path + suffix
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		left = truncate(left, maxInt(m.width-lipgloss.Width(right)-1, 0))
		gap = m.width - lipgloss.Width(left) - lipgloss.Width(right)
	}
	if gap < 1 {
		// left alone (or emptied) still does not leave room: the right
		// side itself is wider than the pane, which the settings hint
		// can be in a narrow terminal. Cut it too, rather than overflow.
		right = truncate(right, maxInt(m.width-lipgloss.Width(left)-1, 0))
		gap = maxInt(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	}
	line := left + strings.Repeat(" ", gap) + right
	return m.styles.status.Render(padLine(line, m.width))
}

// displayPath names a file relative to the root when it sits under it,
// and with ~ for the home directory otherwise.
func (m Model) displayPath(p string) string {
	if rel, err := filepath.Rel(m.root.Path, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return rel
	}
	return tildePath(p)
}

// tildePath replaces a leading home directory with ~.
func tildePath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

func padLine(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}
