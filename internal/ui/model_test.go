package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hidekingerz/mado/internal/config"
)

func testModel(t *testing.T, files map[string]string, open ...string) Model {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var abs []string
	for _, f := range open {
		abs = append(abs, filepath.Join(dir, f))
	}
	cfg := config.Default()
	cfg.Theme.Style = "notty" // deterministic in tests, no terminal probing
	m, err := New(cfg, dir, abs)
	if err != nil {
		t.Fatal(err)
	}
	resized, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return resized.(Model)
}

func update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestOpenMultipleFilesAsTabs(t *testing.T) {
	m := testModel(t,
		map[string]string{"a.md": "# A", "b.md": "# B"},
		"a.md", "b.md",
	)
	if len(m.tabs) != 2 {
		t.Fatalf("tabs = %d, want 2", len(m.tabs))
	}
	if m.active != 1 {
		t.Errorf("active = %d, want 1", m.active)
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.active != 0 {
		t.Errorf("after next_tab active = %d, want 0 (wrap)", m.active)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.active != 1 {
		t.Errorf("after prev_tab active = %d, want 1", m.active)
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if len(m.tabs) != 1 {
		t.Fatalf("after close tabs = %d, want 1", len(m.tabs))
	}
	if m.tabs[0].title != "a.md" {
		t.Errorf("remaining tab = %q, want a.md", m.tabs[0].title)
	}
}

func TestOpeningSameFileTwiceReusesTab(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md", "a.md")
	if len(m.tabs) != 1 {
		t.Fatalf("tabs = %d, want 1", len(m.tabs))
	}
}

func TestKeyboardOpensFileFromTree(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"})
	if m.focus != focusSidebar {
		t.Fatal("initial focus should be the sidebar")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.tabs) != 1 || m.tabs[0].title != "b.md" {
		t.Fatalf("expected b.md open, got %+v", m.tabs)
	}
	if m.focus != focusContent {
		t.Error("focus should move to content after opening")
	}
}

func TestMouseClickOpensFile(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"})
	// Tree rows start below the tab bar and sidebar top border (y=2).
	click := tea.MouseMsg{X: 3, Y: 3, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m = update(t, m, click)
	if len(m.tabs) != 1 || m.tabs[0].title != "b.md" {
		t.Fatalf("expected b.md open via mouse, got %+v", m.tabs)
	}
}

func TestMouseClickTabBarSwitchesAndCloses(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	if m.active != 1 {
		t.Fatalf("active = %d, want 1", m.active)
	}
	first := m.tabRegions[0]
	m = update(t, m, tea.MouseMsg{X: first.start + 1, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.active != 0 {
		t.Errorf("click tab 0: active = %d, want 0", m.active)
	}
	m = update(t, m, tea.MouseMsg{X: m.tabRegions[0].closeX, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if len(m.tabs) != 1 || m.tabs[0].title != "b.md" {
		t.Fatalf("expected a.md closed via ✕, got %+v", m.tabs)
	}
}

func TestMouseWheelScrollsContent(t *testing.T) {
	long := "# Title\n"
	for i := 0; i < 200; i++ {
		long += "line\n\n"
	}
	m := testModel(t, map[string]string{"a.md": long}, "a.md")
	contentX := m.sidebarWidth() + 5
	m = update(t, m, tea.MouseMsg{X: contentX, Y: 10, Button: tea.MouseButtonWheelDown})
	if m.tabs[0].vp.YOffset == 0 {
		t.Error("wheel down should scroll the viewport")
	}
	m = update(t, m, tea.MouseMsg{X: contentX, Y: 10, Button: tea.MouseButtonWheelUp})
	if m.tabs[0].vp.YOffset != 0 {
		t.Errorf("wheel up should scroll back, offset = %d", m.tabs[0].vp.YOffset)
	}
}

func TestToggleSidebar(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	w := m.tabs[0].vp.Width
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if m.sidebar {
		t.Fatal("sidebar should be hidden")
	}
	if m.tabs[0].vp.Width <= w {
		t.Errorf("content should widen: %d -> %d", w, m.tabs[0].vp.Width)
	}
	if m.View() == "" {
		t.Error("view should render without sidebar")
	}
}

func TestViewRenders(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	v := m.View()
	if v == "" {
		t.Fatal("empty view")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !m.showHelp {
		t.Fatal("help should toggle on")
	}
	if m.View() == "" {
		t.Fatal("empty help view")
	}
}
