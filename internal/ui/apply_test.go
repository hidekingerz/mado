package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestApplyConfigThemeRebuildsStylesAndRerenders(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	next := m.cfg
	next.Theme.AccentColor = "#FF0000"
	if cmd := m.applyConfig(next); cmd != nil {
		t.Error("a theme change has nothing to wait on")
	}
	if m.cfg.Theme.AccentColor != "#FF0000" {
		t.Errorf("cfg not replaced: %q", m.cfg.Theme.AccentColor)
	}
	if got := m.styles.accent.GetForeground(); got != lipgloss.Color("#FF0000") {
		t.Errorf("accent style = %v, want #FF0000", got)
	}
	if m.tabs[0].rendered != 0 {
		t.Error("background tabs should be marked dirty")
	}
	if m.tabs[1].rendered == 0 {
		t.Error("the active tab should be re-rendered at once")
	}
}

func TestApplyConfigStyleChangesTheRenderer(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	next := m.cfg
	next.Theme.Style = "ascii"
	m.applyConfig(next)
	if m.style != "ascii" || m.sourceStyle != "" {
		t.Errorf("style = %q sourceStyle = %q, want ascii and no highlighting", m.style, m.sourceStyle)
	}
	next.Theme.Style = "dracula"
	m.applyConfig(next)
	if m.style != "dracula" || m.sourceStyle != "dracula" {
		t.Errorf("style = %q sourceStyle = %q, want dracula twice", m.style, m.sourceStyle)
	}
}

func TestApplyConfigDefaultModeSwitchesTheView(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	next := m.cfg
	next.Theme.DefaultMode = "source"
	m.applyConfig(next)
	if m.mode != modeSource {
		t.Fatalf("mode = %v, want source", m.mode)
	}
	if !strings.Contains(m.tabs[0].content, "# A") {
		t.Error("source mode should show the raw heading marker")
	}
}

func TestApplyConfigKeysTakeEffectImmediately(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	next := m.cfg
	next.Keys.NextTab = []string{"ctrl+n"}
	m.applyConfig(next)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.active != 1 {
		t.Error("tab should no longer switch tabs")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.active != 0 {
		t.Error("ctrl+n should switch tabs now")
	}
}

func TestApplyConfigSidebarReloadsTheTree(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "notes.txt": "hi"})
	if strings.Contains(m.View(), "notes.txt") {
		t.Fatal("txt files are hidden by default")
	}
	next := m.cfg
	next.Sidebar.ShowAllFiles = true
	next.Sidebar.Width = 40
	m.applyConfig(next)
	if !m.treeOpts.ShowAllFiles {
		t.Error("treeOpts should follow the config")
	}
	if !strings.Contains(m.View(), "notes.txt") {
		t.Error("the tree should reload with all files")
	}
	if m.sidebarWidth() != 40 {
		t.Errorf("sidebar width = %d, want 40", m.sidebarWidth())
	}
}

func TestApplyConfigStartsAndStopsTheWatcher(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	t.Cleanup(func() {
		if m.watcher != nil {
			m.watcher.Close()
		}
	})
	next := m.cfg
	next.General.Watch = true
	cmd := m.applyConfig(next)
	if m.watcher == nil {
		t.Fatal("watch on should start a watcher")
	}
	if cmd == nil {
		t.Error("starting the watcher should hand back a command that waits on it")
	}
	if !strings.Contains(m.View(), "[WATCH]") {
		t.Error("status bar should show watching")
	}
	next.General.Watch = false
	if cmd := m.applyConfig(next); cmd != nil {
		t.Error("stopping the watcher has nothing to wait on")
	}
	if m.watcher != nil {
		t.Error("watch off should close the watcher")
	}
	if strings.Contains(m.View(), "[WATCH]") {
		t.Error("status bar should stop showing watching")
	}
}

func TestWithConfigPath(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	if m.configPath != "" {
		t.Fatalf("configPath starts empty, got %q", m.configPath)
	}
	m = m.WithConfigPath("/tmp/x.toml")
	if m.configPath != "/tmp/x.toml" {
		t.Errorf("configPath = %q", m.configPath)
	}
}
