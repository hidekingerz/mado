package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// settingsModel is testModel with a config file to save into.
func settingsModel(t *testing.T, files map[string]string, open ...string) Model {
	t.Helper()
	m := testModel(t, files, open...)
	return m.WithConfigPath(filepath.Join(t.TempDir(), "config.toml"))
}

// selectedField is the key of the field under the settings cursor.
func selectedField(m Model) string {
	f, ok := m.selectedSetting()
	if !ok {
		return ""
	}
	return f.Key
}

// moveTo puts the settings cursor on the named field.
func moveTo(t *testing.T, m Model, key string) Model {
	t.Helper()
	m = update(t, m, tea.KeyMsg{Type: tea.KeyHome})
	for range m.settings.rows {
		if selectedField(m) == key {
			return m
		}
		m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	t.Fatalf("no field %q in the panel", key)
	return m
}

func esc() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyEsc} }
func enter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

func TestSettingsKeyOpensThePanel(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m = update(t, m, keyRune(','))
	if !m.settings.open {
		t.Fatal("',' should open the settings panel")
	}
	if m.settings.prevFocus != focusContent {
		t.Errorf("prevFocus = %v, want content", m.settings.prevFocus)
	}
	v := m.View()
	for _, want := range []string{"Settings", "[general]", "watch", "[theme]", "accent_color", "[sidebar]", "config.toml"} {
		if !strings.Contains(v, want) {
			t.Errorf("panel should show %q:\n%s", want, v)
		}
	}
	if !strings.Contains(m.renderStatusBar(), "[SETTINGS]") {
		t.Errorf("status bar = %q, want [SETTINGS]", m.renderStatusBar())
	}
	if selectedField(m) != "watch" {
		t.Errorf("cursor starts on the first field, got %q", selectedField(m))
	}
	if !strings.Contains(v, "default: false") {
		t.Errorf("footer should describe the selected field:\n%s", v)
	}
}

func TestSettingsEscClosesAndRestoresFocus(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, esc())
	if m.settings.open || m.focus != focusSidebar {
		t.Errorf("open = %v focus = %v, want closed and sidebar", m.settings.open, m.focus)
	}
	m = update(t, m, keyRune(','))
	m = update(t, m, keyRune(','))
	if m.settings.open {
		t.Error("the settings key should close the panel too")
	}
}

func TestSettingsCursorSkipsHeadings(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if got := selectedField(m); got != "style" {
		t.Errorf("down from watch should skip [theme] and land on style, got %q", got)
	}
	m = update(t, m, keyRune('k'))
	if got := selectedField(m); got != "watch" {
		t.Errorf("k should move back up to watch, got %q", got)
	}
	m = update(t, m, keyRune('k'))
	if got := selectedField(m); got != "watch" {
		t.Errorf("up at the top stays put, got %q", got)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnd})
	if got := selectedField(m); got != "help" {
		t.Errorf("end should reach the last field, got %q", got)
	}
	if !strings.Contains(m.View(), "[keys]") {
		t.Error("the list should scroll to keep the cursor in view")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyHome})
	if got := selectedField(m); got != "watch" {
		t.Errorf("home should reach the first field, got %q", got)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	if selectedField(m) == "watch" {
		t.Error("pgdown should move")
	}
}

func TestSettingsTypedKeysDoNotAct(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m = update(t, m, keyRune(','))
	m = update(t, m, keyRune('q'))
	if !m.settings.open {
		t.Fatal("q must not quit or close the panel")
	}
	m = update(t, m, keyRune('/'))
	if m.search.open {
		t.Error("/ must not open the search over the panel")
	}
	m = update(t, m, keyRune('x'))
	if len(m.tabs) != 1 {
		t.Error("x must not close the tab")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c should produce tea.Quit")
	}
}

func TestSettingsOpensOverHelpNotOverSearch(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	// While the search prompt has the keyboard a typed "," is query
	// text, as every typed character is.
	m = update(t, m, keyRune('/'))
	m = update(t, m, keyRune(','))
	if m.settings.open || !m.search.open || m.search.query != "," {
		t.Errorf("settings = %v search = %v query = %q, want the comma typed into the query", m.settings.open, m.search.open, m.search.query)
	}
	m = update(t, m, esc())
	m = update(t, m, keyRune('?'))
	m = update(t, m, keyRune(','))
	if m.showHelp || !m.settings.open {
		t.Errorf("help = %v settings = %v, want settings over help", m.showHelp, m.settings.open)
	}
}

func TestSettingsMouseSelectsAndScrolls(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	x := m.sidebarWidth() + 2
	// List rows start below the tab bar (1), the border (1) and the
	// header (2). Rows: [general], watch, [theme], style.
	top := tabBarHeight + 1 + settingsHeaderRows
	m = update(t, m, tea.MouseMsg{X: x, Y: top + 3, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if got := selectedField(m); got != "style" {
		t.Errorf("click on row 3 should select style, got %q", got)
	}
	m = update(t, m, tea.MouseMsg{X: x, Y: top + 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if got := selectedField(m); got != "style" {
		t.Errorf("clicking a heading changes nothing, got %q", got)
	}
	m = update(t, m, tea.MouseMsg{X: x, Y: top, Button: tea.MouseButtonWheelDown})
	if m.settings.off != wheelStep {
		t.Errorf("wheel should scroll the list, off = %d", m.settings.off)
	}
}

func TestSettingsRowsFitThePane(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, tea.WindowSizeMsg{Width: 44, Height: 12})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnd})
	for _, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > 44 {
			t.Errorf("line is %d wide, pane is 44: %q", w, line)
		}
	}
}

func TestSettingsRemoteOpenClosesThePanel(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A", "b.md": "# B"})
	m, srv := serveModel(t, m)
	m = update(t, m, keyRune(','))
	m, err := sendRemote(t, m, srv, "open", filepath.Join(m.root.Path, "b.md"))
	if err != nil {
		t.Fatal(err)
	}
	if m.settings.open {
		t.Error("a remote open should put the file in front, closing the panel")
	}
}
