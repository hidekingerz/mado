package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/termsafe"
)

// settingsHeaderRows is the title and the blank line above the list;
// settingsFooterRows the blank line and the description below it.
const (
	settingsHeaderRows = 2
	settingsFooterRows = 2
)

type settingsEdit int

const (
	editNone    settingsEdit = iota
	editText                 // an inline prompt holds the value
	editCapture              // the next key press is the value
)

// settingsRow is one line of the panel: a table heading or a field.
type settingsRow struct {
	heading string // "[theme]"; "" for a field row
	field   int    // index into settingsState.fields; -1 for a heading
}

// settingsState is the settings panel: every config field listed
// under its table, a cursor on one of them, and an edit in progress.
type settingsState struct {
	open      bool
	fields    []config.Field
	rows      []settingsRow
	cursor    int // index into rows; always a field row
	off       int
	editing   settingsEdit
	input     string // the text prompt's contents
	prevFocus focusArea
}

// openSettings shows the panel over whatever the content pane held,
// search and help included.
func (m *Model) openSettings() {
	if m.settings.open {
		return
	}
	m.closeSearch()
	m.showHelp = false
	s := &m.settings
	s.open = true
	s.prevFocus = m.focus
	s.editing = editNone
	s.fields = config.Fields()
	s.rows = s.rows[:0]
	table := ""
	for i, f := range s.fields {
		if f.Table != table {
			table = f.Table
			s.rows = append(s.rows, settingsRow{heading: "[" + table + "]", field: -1})
		}
		s.rows = append(s.rows, settingsRow{field: i})
	}
	s.cursor, s.off = 0, 0
	m.moveSettingsCursor(1) // off the first heading, onto the first field
	m.statusMsg = ""
}

// closeSettings hides the panel and returns the focus to where it was.
func (m *Model) closeSettings() {
	if !m.settings.open {
		return
	}
	m.settings.open = false
	m.settings.editing = editNone
	m.restoreFocus(m.settings.prevFocus)
}

// restoreFocus puts the focus back on prev after a panel closes,
// unless that pane is gone: a hidden sidebar hands to the content,
// an empty content pane hands to the sidebar.
func (m *Model) restoreFocus(prev focusArea) {
	m.focus = prev
	if m.focus == focusSidebar && !m.sidebar {
		m.focus = focusContent
	}
	if m.focus == focusContent && len(m.tabs) == 0 && m.sidebar {
		m.focus = focusSidebar
	}
}

// selectedSetting returns the field under the cursor.
func (m *Model) selectedSetting() (config.Field, bool) {
	s := m.settings
	if !s.open || s.cursor < 0 || s.cursor >= len(s.rows) || s.rows[s.cursor].field < 0 {
		return config.Field{}, false
	}
	return s.fields[s.rows[s.cursor].field], true
}

func (m *Model) settingsListHeight() int {
	return maxInt(m.contentInnerHeight()-settingsHeaderRows-settingsFooterRows, 0)
}

// moveSettingsCursor moves delta field rows, stepping over headings,
// and keeps the cursor on screen.
func (m *Model) moveSettingsCursor(delta int) {
	s := &m.settings
	step := 1
	if delta < 0 {
		step, delta = -1, -delta
	}
	for ; delta > 0; delta-- {
		next := s.cursor
		for {
			next += step
			if next < 0 || next >= len(s.rows) || s.rows[next].field >= 0 {
				break
			}
		}
		if next < 0 || next >= len(s.rows) {
			break
		}
		s.cursor = next
	}
	m.clampSettings()
}

// scrollSettings moves the list without moving the cursor.
func (m *Model) scrollSettings(delta int) {
	s := &m.settings
	s.off += delta
	if max := maxInt(len(s.rows)-m.settingsListHeight(), 0); s.off > max {
		s.off = max
	}
	if s.off < 0 {
		s.off = 0
	}
}

// clampSettings keeps the cursor in range and on screen.
func (m *Model) clampSettings() {
	s := &m.settings
	n := len(s.rows)
	if s.cursor >= n {
		s.cursor = n - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	h := m.settingsListHeight()
	if max := maxInt(n-h, 0); s.off > max {
		s.off = max
	}
	if s.off < 0 {
		s.off = 0
	}
	if h <= 0 {
		return
	}
	if s.cursor < s.off {
		s.off = s.cursor
	}
	if s.cursor >= s.off+h {
		s.off = s.cursor - h + 1
	}
}

// handleSettingsKey runs the panel's fixed keys. The keys that edit a
// field are dispatched by its kind.
func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := &m.settings
	if key.Matches(msg, m.keys.Settings) {
		m.closeSettings()
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.closeSettings()
	case tea.KeyUp, tea.KeyCtrlP:
		m.moveSettingsCursor(-1)
	case tea.KeyDown, tea.KeyCtrlN:
		m.moveSettingsCursor(1)
	case tea.KeyPgUp:
		m.moveSettingsCursor(-maxInt(m.settingsListHeight(), 1))
	case tea.KeyPgDown:
		m.moveSettingsCursor(maxInt(m.settingsListHeight(), 1))
	case tea.KeyHome:
		m.moveSettingsCursor(-len(s.rows))
	case tea.KeyEnd:
		m.moveSettingsCursor(len(s.rows))
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "k":
			m.moveSettingsCursor(-1)
		case "j":
			m.moveSettingsCursor(1)
		}
	}
	return m, nil
}

// handleSettingsMouse selects the row under a click and scrolls with
// the wheel. Clicks outside the panel close it and act as they would
// without it.
func (m Model) handleSettingsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	inSidebar := m.sidebar && msg.X < m.sidebarWidth()
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inSidebar {
			m.scrollTree(-wheelStep)
		} else {
			m.scrollSettings(-wheelStep)
		}
	case tea.MouseButtonWheelDown:
		if inSidebar {
			m.scrollTree(wheelStep)
		} else {
			m.scrollSettings(wheelStep)
		}
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		switch {
		case msg.Y < tabBarHeight:
			m.closeSettings()
			m.clickTabBar(msg.X)
		case inSidebar:
			m.closeSettings()
			m.clickSidebar(msg.X, msg.Y)
		default:
			row := msg.Y - tabBarHeight - 1 - settingsHeaderRows
			idx := m.settings.off + row
			if row >= 0 && row < m.settingsListHeight() && idx < len(m.settings.rows) && m.settings.rows[idx].field >= 0 {
				m.settings.editing = editNone
				m.settings.cursor = idx
			}
		}
	}
	return m, nil
}

// ── rendering ───────────────────────────────────────────────────────

func (m Model) renderSettings(w, h int) string {
	s := m.settings
	rows := make([]string, 0, h)

	title := m.styles.accent.Bold(true).Render("Settings")
	where := "not saved: no config path"
	if m.configPath != "" {
		// The path came from a flag or the environment: show it, do
		// not obey it.
		where = termsafe.String(m.configPath)
	}
	// A long path is cut from the left so its file name stays readable.
	where = truncateLeft(where, maxInt(w-1-lipgloss.Width(title)-2, 0))
	gap := maxInt(w-1-lipgloss.Width(title)-lipgloss.Width(where), 1)
	rows = append(rows, clip(" "+title+strings.Repeat(" ", gap)+m.styles.dimmed.Render(where), w))
	rows = append(rows, "")

	keyW := 0
	for _, f := range s.fields {
		if lw := lipgloss.Width(f.Key); lw > keyW {
			keyW = lw
		}
	}
	listH := maxInt(h-settingsHeaderRows-settingsFooterRows, 0)
	for i := 0; i < listH; i++ {
		idx := s.off + i
		if idx >= len(s.rows) {
			rows = append(rows, "")
			continue
		}
		rows = append(rows, m.renderSettingsRow(s.rows[idx], keyW, w, idx == s.cursor))
	}
	if len(rows)+settingsFooterRows <= h {
		rows = append(rows, "", clip(m.renderSettingsFooter(w), w))
	}
	return strings.Join(rows, "\n")
}

// renderSettingsRow lays out one row in w columns: the key, then the
// value — in the accent color when it is not the default, in the
// selection style under the cursor, and as a prompt or a capture
// notice while that field is being edited.
func (m Model) renderSettingsRow(r settingsRow, keyW, w int, selected bool) string {
	if r.field < 0 {
		return clip(m.styles.accent.Bold(true).Render(r.heading), w)
	}
	f := m.settings.fields[r.field]
	// Style paths and chroma names are strings from the config file.
	value := termsafe.String(f.Get(&m.cfg))
	name := " " + f.Key + strings.Repeat(" ", keyW-lipgloss.Width(f.Key)) + "  "
	valueW := maxInt(w-lipgloss.Width(name), 0)
	switch {
	case selected && m.settings.editing == editText:
		text := truncateLeft(termsafe.String(m.settings.input), maxInt(valueW-1, 0))
		return name + text + m.styles.selection.Render(" ")
	case selected && m.settings.editing == editCapture:
		return m.styles.selection.Render(padLine(truncate(name+value+"  press a key…", w), w))
	case selected:
		return m.styles.selection.Render(padLine(truncate(name+value, w), w))
	case value != f.Default():
		return renderSettingsPlainRow(name, value, w, m.styles.accent)
	default:
		return renderSettingsPlainRow(name, value, w, m.styles.file)
	}
}

// renderSettingsPlainRow lays out name+value in w columns for a
// non-selected row: value is styled and truncated to what is left
// after name, but when name itself does not fit in w (a long key
// name in a narrow pane) name is truncated instead, so the row never
// exceeds w and wraps onto a second line.
func renderSettingsPlainRow(name, value string, w int, style lipgloss.Style) string {
	if lipgloss.Width(name) > w {
		return truncate(name, w)
	}
	valueW := maxInt(w-lipgloss.Width(name), 0)
	return name + style.Render(truncate(value, valueW))
}

// renderSettingsFooter describes the selected field and its default,
// in w columns. The description is cut short, never the default: a
// clipped default would misreport what "default" means for a long
// description.
func (m Model) renderSettingsFooter(w int) string {
	f, ok := m.selectedSetting()
	if !ok {
		return ""
	}
	def := f.Default()
	if def == "" {
		def = "(empty)"
	}
	suffix := "  ·  default: " + def
	desc := truncate(f.Desc, maxInt(w-1-lipgloss.Width(suffix), 0))
	return m.styles.dimmed.Render(" " + desc + suffix)
}

// settingsHint is the status bar's right side while the panel is
// open: what the keys do for the selected field.
func (m Model) settingsHint() string {
	switch m.settings.editing {
	case editText:
		return "enter apply │ esc cancel "
	case editCapture:
		return "press a key │ ctrl+c quit "
	}
	f, ok := m.selectedSetting()
	if !ok {
		return "esc close "
	}
	switch f.Kind {
	case config.KindBool:
		return "enter toggle │ esc close "
	case config.KindEnum:
		return "←/→ change │ esc close "
	case config.KindKeys:
		return "enter capture │ backspace remove │ esc close "
	default:
		return "enter edit │ esc close "
	}
}
