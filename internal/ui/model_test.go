package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/remote"
)

func testModel(t *testing.T, files map[string]string, open ...string) Model {
	t.Helper()
	cfg := config.Default()
	cfg.Theme.Style = "notty" // deterministic in tests, no terminal probing
	return testModelWithConfig(t, cfg, files, open...)
}

func testModelWithConfig(t *testing.T, cfg config.Config, files map[string]string, open ...string) Model {
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
	m, err := New(cfg, dir, abs)
	if err != nil {
		t.Fatal(err)
	}
	if m.watcher != nil {
		t.Cleanup(func() { m.watcher.Close() })
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

func TestToggleModeSwitchesReaderAndSource(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "## Heading\n\nbody text\n"}, "a.md")
	if m.mode != modeReader {
		t.Fatal("default mode should be reader")
	}
	reader := m.tabs[0].vp.View()
	if strings.Contains(reader, "## Heading") {
		t.Errorf("reader mode should not show the ## marker:\n%s", reader)
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if m.mode != modeSource {
		t.Fatal("mode should toggle to source")
	}
	source := m.tabs[0].vp.View()
	if !strings.Contains(source, "## Heading") {
		t.Errorf("source mode should show the raw syntax:\n%s", source)
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if m.mode != modeReader {
		t.Fatal("mode should toggle back to reader")
	}
}

func TestDefaultModeFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	if err := os.WriteFile(path, []byte("# A"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Theme.Style = "notty"
	cfg.Theme.DefaultMode = "source"
	m, err := New(cfg, dir, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if m.mode != modeSource {
		t.Error("default_mode = source should start in source mode")
	}
}

func TestSourceModeHighlights(t *testing.T) {
	m := testModel(t, map[string]string{"doc.md": "# Title\n\nsome text\n"}, "doc.md")
	// testModel uses style "notty", which disables highlighting; force it
	// on directly (same package) to stay independent of the test terminal.
	m.sourceStyle = "catppuccin-mocha"
	m.formatter = "terminal16m"

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}) // reader → source
	if m.mode != modeSource {
		t.Fatalf("mode = %v, want source", m.mode)
	}
	if view := m.tabs[0].vp.View(); !strings.Contains(view, "\x1b[") {
		t.Error("source mode content should contain ANSI escapes")
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}) // back to reader: must not panic
	if m.mode != modeReader {
		t.Fatalf("mode = %v, want reader", m.mode)
	}
}

func TestSourceModeNottyStaysPlain(t *testing.T) {
	m := testModel(t, map[string]string{"doc.md": "# Title\n"}, "doc.md")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if view := m.tabs[0].vp.View(); strings.Contains(view, "\x1b[38") {
		t.Error("notty style must not colorize source mode")
	}
}

func TestToggleAllFilesShowsNonMarkdown(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "main.go": "package main\n"})
	has := func(name string) bool {
		for _, it := range m.items {
			if it.Node.Name == name {
				return true
			}
		}
		return false
	}
	if has("main.go") {
		t.Fatal("main.go should be hidden by default")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !has("main.go") {
		t.Fatal("main.go should appear after toggle")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if has("main.go") {
		t.Fatal("main.go should be hidden again after second toggle")
	}
}

func TestToggleAllFilesKeepsExpansion(t *testing.T) {
	m := testModel(t, map[string]string{"docs/a.md": "# A", "b.md": "# B", "main.go": "package main\n"})
	// Directories sort first: cursor 0 is docs. Enter expands it.
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	var docsExpanded, hasGo bool
	for _, it := range m.items {
		if it.Node.Name == "docs" && it.Node.Expanded {
			docsExpanded = true
		}
		if it.Node.Name == "main.go" {
			hasGo = true
		}
	}
	if !docsExpanded {
		t.Error("docs should stay expanded across the toggle")
	}
	if !hasGo {
		t.Error("main.go should be visible after the toggle")
	}
}

// Regression tests for issue #9: reload must keep the sidebar state —
// nested expansion and the selected node.
func TestReloadKeepsNestedExpansion(t *testing.T) {
	m := testModel(t, map[string]string{"docs/inner/deep.md": "# D", "top.md": "# T"})
	// cursor 0 = docs; expand it, then expand docs/inner below it.
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	rows := len(m.items) // docs, inner, deep.md, top.md
	if rows != 4 {
		t.Fatalf("setup: %d rows, want 4", rows)
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if len(m.items) != rows {
		var names []string
		for _, it := range m.items {
			names = append(names, it.Node.Name)
		}
		t.Fatalf("reload collapsed nested dirs: %d rows, want %d (%v)", len(m.items), rows, names)
	}
}

func TestReloadKeepsCursorOnSameNode(t *testing.T) {
	m := testModel(t, map[string]string{"b.md": "# B", "z.md": "# Z"})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown}) // cursor on z.md
	if m.items[m.cursor].Node.Name != "z.md" {
		t.Fatalf("setup: cursor on %q", m.items[m.cursor].Node.Name)
	}
	// A new file that sorts before z.md shifts the row indexes.
	if err := os.WriteFile(filepath.Join(m.root.Path, "a.md"), []byte("# A"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if got := m.items[m.cursor].Node.Name; got != "z.md" {
		t.Errorf("cursor drifted to %q after reload, want z.md", got)
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

func TestSidebarMarksDirectories(t *testing.T) {
	m := testModel(t, map[string]string{"docs/a.md": "# A", "b.md": "# B"})
	view := m.View()
	if !strings.Contains(view, "docs/") {
		t.Error("directory should render with a trailing slash")
	}
	if strings.Contains(view, "b.md/") {
		t.Error("file must not have a trailing slash")
	}
	if !m.styles.dir.GetBold() {
		t.Error("dir style should be bold")
	}
	if fg, ok := m.styles.dir.GetForeground().(lipgloss.Color); !ok || string(fg) != "#89B4FA" {
		t.Errorf("dir style foreground = %v, want #89B4FA", m.styles.dir.GetForeground())
	}
}

func TestUnfocusedCursorRowStyle(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	if fg, ok := m.styles.cursor.GetForeground().(lipgloss.Color); !ok || string(fg) != "#FFFFFF" {
		t.Errorf("cursor style foreground = %v, want #FFFFFF (file color)", m.styles.cursor.GetForeground())
	}
	if !m.styles.cursor.GetBold() {
		t.Error("unfocused cursor row style should be bold")
	}
}

// ── auto-reload (--watch) ───────────────────────────────────────────

func watchModel(t *testing.T, files map[string]string, open ...string) Model {
	t.Helper()
	cfg := config.Default()
	cfg.Theme.Style = "notty"
	cfg.General.Watch = true
	return testModelWithConfig(t, cfg, files, open...)
}

func TestWatchOffByDefault(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	if m.watcher != nil {
		t.Error("watcher should not start unless watch is enabled")
	}
	if m.Init() != nil {
		t.Error("Init should have nothing to wait on without a watcher")
	}
	if strings.Contains(m.View(), "[WATCH]") {
		t.Error("status bar should not advertise watching when it is off")
	}
}

func TestWatchEnabledStartsWatching(t *testing.T) {
	m := watchModel(t, map[string]string{"a.md": "# A"}, "a.md")
	if m.watcher == nil {
		t.Fatal("watch enabled but no watcher started")
	}
	if m.Init() == nil {
		t.Error("Init should wait for changes when watching")
	}
	if !strings.Contains(m.View(), "[WATCH]") {
		t.Error("status bar should show that watching is on")
	}
}

func TestFileChangedMsgRerendersActiveTab(t *testing.T) {
	m := watchModel(t, map[string]string{"a.md": "# before"}, "a.md")
	if err := os.WriteFile(m.tabs[0].path, []byte("# after"), 0o644); err != nil {
		t.Fatal(err)
	}

	next, cmd := m.Update(fileChangedMsg{})
	m = next.(Model)
	if m.tabs[0].raw != "# after" {
		t.Errorf("tab content = %q, want %q", m.tabs[0].raw, "# after")
	}
	if !strings.Contains(m.tabs[0].vp.View(), "after") {
		t.Error("viewport still shows the old render")
	}
	if cmd == nil {
		t.Error("the model should keep waiting for the next change")
	}
}

func TestFileChangedMsgKeepsScrollPosition(t *testing.T) {
	var long strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&long, "line %d\n\n", i)
	}
	m := watchModel(t, map[string]string{"a.md": long.String()}, "a.md")
	m.tabs[0].vp.SetYOffset(40)
	want := m.tabs[0].vp.YOffset
	if want == 0 {
		t.Fatal("setup: viewport did not scroll")
	}

	if err := os.WriteFile(m.tabs[0].path, []byte(long.String()+"\nlast\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = update(t, m, fileChangedMsg{})
	if got := m.tabs[0].vp.YOffset; got != want {
		t.Errorf("scroll offset = %d after reload, want %d", got, want)
	}
}

func TestFileChangedMsgPicksUpNewFilesInTheTree(t *testing.T) {
	m := watchModel(t, map[string]string{"a.md": "# A"})
	rows := len(m.items)
	if err := os.WriteFile(filepath.Join(m.root.Path, "b.md"), []byte("# B"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = update(t, m, fileChangedMsg{})
	if len(m.items) != rows+1 {
		t.Errorf("tree rows = %d after reload, want %d", len(m.items), rows+1)
	}
}

// A reload refreshes every open tab, not just the visible one, so
// switching to a background tab never shows content from before the
// reload.
func TestReloadRefreshesBackgroundTabs(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	background := m.tabs[0]
	if err := os.WriteFile(background.path, []byte("# A2"), 0o644); err != nil {
		t.Fatal(err)
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if background.raw != "# A2" {
		t.Errorf("background tab content = %q, want %q", background.raw, "# A2")
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab}) // back to a.md
	if !strings.Contains(m.tabs[m.active].vp.View(), "A2") {
		t.Error("background tab was not re-rendered when it became active")
	}
}

func TestQuitStopsTheWatcher(t *testing.T) {
	m := watchModel(t, map[string]string{"a.md": "# A"}, "a.md")
	events := m.watcher.Events()
	update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected the watcher to be closed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watcher was not closed on quit")
	}
}

// The watched set must follow what is on screen: the tree root, the
// directories expanded in the sidebar, and the parents of open files.
func TestWatchCoversOpenFilesAndExpandedDirs(t *testing.T) {
	m := watchModel(t, map[string]string{"docs/a.md": "# A", "top.md": "# T"})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // expand docs/
	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // open docs/a.md
	if len(m.tabs) != 1 {
		t.Fatalf("setup: %d tabs, want 1", len(m.tabs))
	}

	for _, path := range []string{
		filepath.Join(m.root.Path, "top.md"),
		filepath.Join(m.root.Path, "docs", "a.md"),
	} {
		if err := os.WriteFile(path, []byte("# changed"), 0o644); err != nil {
			t.Fatal(err)
		}
		select {
		case _, ok := <-m.watcher.Events():
			if !ok {
				t.Fatalf("%s: watcher closed", path)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("no change reported for %s", path)
		}
	}
}

// ── remote commands (mado --remote …) ───────────────────────────────

func serveModel(t *testing.T, m Model) (Model, *remote.Server) {
	t.Helper()
	srv, err := remote.Listen(filepath.Join(t.TempDir(), "mado.sock"))
	if err != nil {
		t.Fatalf("remote.Listen: %v", err)
	}
	t.Cleanup(func() { srv.Close() })
	return m.Serve(srv), srv
}

// sendRemote drives a full round trip: another process sends a
// command, the model handles it, and the sender's answer comes back.
func sendRemote(t *testing.T, m Model, srv *remote.Server, cmd, path string) (Model, error) {
	t.Helper()
	answered := make(chan error, 1)
	go func() { answered <- remote.Send(filepath.Dir(srv.Path()), cmd, path) }()

	req, ok := srv.Next()
	if !ok {
		t.Fatal("server closed before the request arrived")
	}
	next, _ := m.Update(remoteRequestMsg{req})

	select {
	case err := <-answered:
		return next.(Model), err
	case <-time.After(5 * time.Second):
		t.Fatal("the sender never got an answer")
		return m, nil
	}
}

func TestNoRemoteServerByDefault(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	if m.srv != nil {
		t.Error("a model should not serve remote commands until it is given a server")
	}
	if m.Init() != nil {
		t.Error("Init should have nothing to wait on without a server")
	}
}

func TestRemoteOpenAddsATab(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md")
	m, srv := serveModel(t, m)

	path := filepath.Join(m.root.Path, "b.md")
	m, err := sendRemote(t, m, srv, remote.CmdOpen, path)
	if err != nil {
		t.Fatalf("remote open: %v", err)
	}
	if len(m.tabs) != 2 {
		t.Fatalf("tabs = %d, want 2", len(m.tabs))
	}
	if m.tabs[m.active].path != path {
		t.Errorf("active tab = %q, want %q", m.tabs[m.active].path, path)
	}
	if m.focus != focusContent {
		t.Error("a remotely opened file should take the focus")
	}
}

func TestRemoteOpenReusesTheExistingTab(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	m, srv := serveModel(t, m)
	if m.active != 1 {
		t.Fatalf("setup: active = %d, want 1", m.active)
	}

	path := filepath.Join(m.root.Path, "a.md")
	m, err := sendRemote(t, m, srv, remote.CmdOpen, path)
	if err != nil {
		t.Fatalf("remote open: %v", err)
	}
	if len(m.tabs) != 2 {
		t.Errorf("tabs = %d, want 2 — the file was already open", len(m.tabs))
	}
	if m.active != 0 {
		t.Errorf("active = %d, want the tab that already held the file", m.active)
	}
}

func TestRemoteOpenReportsAnUnreadableFile(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m, srv := serveModel(t, m)

	m, err := sendRemote(t, m, srv, remote.CmdOpen, filepath.Join(m.root.Path, "gone.md"))
	if err == nil {
		t.Fatal("expected the sender to be told the file could not be opened")
	}
	if len(m.tabs) != 1 {
		t.Errorf("tabs = %d, want 1 — nothing should have been added", len(m.tabs))
	}
}

func TestRemoteFocusSwitchesTabs(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	m, srv := serveModel(t, m)

	m, err := sendRemote(t, m, srv, remote.CmdFocus, filepath.Join(m.root.Path, "a.md"))
	if err != nil {
		t.Fatalf("remote focus: %v", err)
	}
	if m.active != 0 {
		t.Errorf("active = %d, want 0", m.active)
	}
	if m.focus != focusContent {
		t.Error("focusing a tab should move the focus to the content pane")
	}
}

// focus never opens anything: it is for pointing an instance at a file
// it is already showing.
func TestRemoteFocusFailsForAClosedFile(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md")
	m, srv := serveModel(t, m)

	m, err := sendRemote(t, m, srv, remote.CmdFocus, filepath.Join(m.root.Path, "b.md"))
	if err == nil {
		t.Fatal("expected focus on an unopened file to fail")
	}
	if len(m.tabs) != 1 {
		t.Errorf("tabs = %d, want 1 — focus must not open files", len(m.tabs))
	}
}

func TestRemoteUnknownCommand(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m, srv := serveModel(t, m)

	if _, err := sendRemote(t, m, srv, "explode", filepath.Join(m.root.Path, "a.md")); err == nil {
		t.Fatal("expected an unknown command to be refused")
	}
}

func TestQuitClosesTheRemoteServer(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m, srv := serveModel(t, m)
	if m.Init() == nil {
		t.Error("Init should wait for remote commands when serving")
	}

	update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if _, err := os.Stat(srv.Path()); !os.IsNotExist(err) {
		t.Errorf("socket left behind after quit (stat err = %v)", err)
	}
}

// The sidebar and remote commands name files by absolute path, the
// command line usually by a relative one. They must land on one tab.
func TestOpenFileIdentifiesTabsByAbsolutePath(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	t.Chdir(m.root.Path)

	if err := m.openFile("a.md"); err != nil {
		t.Fatalf("open by relative path: %v", err)
	}
	if err := m.openFile(filepath.Join(m.root.Path, "a.md")); err != nil {
		t.Fatalf("open by absolute path: %v", err)
	}
	if len(m.tabs) != 1 {
		t.Fatalf("tabs = %d, want 1 — the same file named two ways", len(m.tabs))
	}
	if !filepath.IsAbs(m.tabs[0].path) {
		t.Errorf("tab path = %q, want an absolute path", m.tabs[0].path)
	}
}

func TestRemoteFocusFindsAFileOpenedByRelativePath(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	t.Chdir(m.root.Path)
	if err := m.openFile("a.md"); err != nil {
		t.Fatal(err)
	}
	m, srv := serveModel(t, m)

	if _, err := sendRemote(t, m, srv, remote.CmdFocus, filepath.Join(m.root.Path, "a.md")); err != nil {
		t.Errorf("remote focus: %v", err)
	}
}

// ── watching and remote commands together ───────────────────────────

// A file handed over by another process is watched like any other open
// file, even when it sits outside the directories the sidebar tree is
// already watching.
func TestRemoteOpenStartsWatchingTheFile(t *testing.T) {
	m := watchModel(t, map[string]string{"a.md": "# A", "sub/b.md": "# B"})
	m, srv := serveModel(t, m)
	// sub/ is collapsed in the sidebar, so nothing is watching it yet.
	path := filepath.Join(m.root.Path, "sub", "b.md")

	m, err := sendRemote(t, m, srv, remote.CmdOpen, path)
	if err != nil {
		t.Fatalf("remote open: %v", err)
	}
	if err := os.WriteFile(path, []byte("# B2"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-m.watcher.Events():
		if !ok {
			t.Fatal("watcher closed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a change to the remotely opened file was not reported")
	}
}

func TestQuitStopsWatcherAndRemoteServerTogether(t *testing.T) {
	m := watchModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m, srv := serveModel(t, m)
	if m.Init() == nil {
		t.Fatal("Init should wait on both the watcher and the remote server")
	}
	events := m.watcher.Events()

	update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if _, err := os.Stat(srv.Path()); !os.IsNotExist(err) {
		t.Errorf("socket left behind after quit (stat err = %v)", err)
	}
	select {
	case _, ok := <-events:
		if ok {
			t.Error("expected the watcher to be closed too")
		}
	case <-time.After(3 * time.Second):
		t.Error("watcher was not closed on quit")
	}
}

// ── terminal escapes in files are data, not commands ────────────────

const osc52 = "\x1b]52;c;aGFjaw==\x07"

func TestFileContentCannotDriveTheTerminal(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# Title\n\npwned " + osc52 + " more\n"}, "a.md")

	if got := m.tabs[0].raw; strings.Contains(got, "\x1b") {
		t.Errorf("tab content still holds an escape: %q", got)
	}
	view := m.tabs[0].vp.View()
	if strings.Contains(view, "]52;c;") && !strings.Contains(view, "^[]52;c;") {
		t.Error("the OSC 52 sequence reached the rendered view intact")
	}
	if !strings.Contains(view, "^[]52;c;") {
		t.Errorf("expected the sequence to show as text, got %q", view)
	}
}

func TestSourceModeCannotDriveTheTerminal(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "pwned " + osc52 + " more\n"}, "a.md")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if m.mode != modeSource {
		t.Fatal("setup: not in source mode")
	}
	if view := m.tabs[0].vp.View(); !strings.Contains(view, "^[]52;c;") {
		t.Errorf("expected the sequence to show as text in source mode, got %q", view)
	}
}

// A reload picks up whatever the file says now, with no keystroke when
// watching. Content that arrives that way gets the same treatment.
func TestReloadedContentCannotDriveTheTerminal(t *testing.T) {
	m := watchModel(t, map[string]string{"a.md": "# harmless\n"}, "a.md")
	if err := os.WriteFile(m.tabs[0].path, []byte("pwned "+osc52+" more\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = update(t, m, fileChangedMsg{})

	if got := m.tabs[0].raw; strings.Contains(got, "\x1b") {
		t.Errorf("reloaded content still holds an escape: %q", got)
	}
}

// A file name is chosen by whoever created the file, and --watch makes
// new files appear in the sidebar without anyone asking for them.
func TestFileNamesCannotDriveTheTerminal(t *testing.T) {
	m := testModel(t, map[string]string{"evil\x1b]0;pwn\x07.md": "# hi\n"})

	view := m.View()
	if strings.Contains(view, "\x1b]0;pwn") {
		t.Error("a file name drove the terminal from the sidebar")
	}
	if !strings.Contains(view, "^[]0;pwn") {
		t.Errorf("expected the name to show as text, got %q", view)
	}
}

func TestStatusBarPathCannotDriveTheTerminal(t *testing.T) {
	m := testModel(t, map[string]string{"evil\x1b]0;pwn\x07.md": "# hi\n"}, "evil\x1b]0;pwn\x07.md")
	if strings.Contains(m.renderStatusBar(), "\x1b]0;pwn") {
		t.Error("the status bar path drove the terminal")
	}
}

func TestToggleLineNumbersInSourceMode(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "one\ntwo\nthree\n"}, "a.md")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}) // reader → source
	if v := m.tabs[0].vp.View(); strings.Contains(v, "1 one") {
		t.Fatalf("line numbers should be off by default:\n%s", v)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	v := m.tabs[0].vp.View()
	if !strings.Contains(v, "1 one") || !strings.Contains(v, "3 three") {
		t.Errorf("source mode should number lines after toggle:\n%s", v)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if v := m.tabs[0].vp.View(); strings.Contains(v, "1 one") {
		t.Errorf("line numbers should toggle back off:\n%s", v)
	}
}

func TestLineNumbersOnPlainTextFile(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.Style = "notty"
	cfg.Sidebar.ShowAllFiles = true
	m := testModelWithConfig(t, cfg, map[string]string{"notes.txt": "alpha\nbeta\n"}, "notes.txt")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	v := m.tabs[0].vp.View()
	if !strings.Contains(v, "1 alpha") || !strings.Contains(v, "2 beta") {
		t.Errorf("text files render as source, so numbers apply without a mode toggle:\n%s", v)
	}
}

func TestLineNumbersLeaveReaderModeAlone(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# Title\n\nbody\n"}, "a.md")
	before := m.tabs[0].vp.View()
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if after := m.tabs[0].vp.View(); after != before {
		t.Errorf("reader mode must not change when line numbers toggle:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestLineNumbersCoexistWithHighlighting(t *testing.T) {
	m := testModel(t, map[string]string{"doc.md": "# Title\n\ntext\n"}, "doc.md")
	m.sourceStyle = "catppuccin-mocha"
	m.formatter = "terminal16m"
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	v := m.tabs[0].vp.View()
	if !strings.Contains(v, "\x1b[") {
		t.Error("highlighting should survive line numbering")
	}
	if !strings.Contains(stripAnsi(v), "1 # Title") {
		t.Errorf("numbers should prefix highlighted lines:\n%s", stripAnsi(v))
	}
}

// cursorName reports the sidebar row the cursor is on, "" if none.
func cursorName(m Model) string {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return ""
	}
	return m.items[m.cursor].Node.Name
}

func TestOpenFileRevealsItInTheSidebar(t *testing.T) {
	m := testModel(t,
		map[string]string{"docs/inner/deep.md": "# D", "top.md": "# T"},
		"docs/inner/deep.md",
	)
	if got := cursorName(m); got != "deep.md" {
		t.Fatalf("cursor = %q, want deep.md; rows: %v", got, len(m.items))
	}
	var docs, inner bool
	for _, it := range m.items {
		if it.Node.Name == "docs" && it.Node.Expanded {
			docs = true
		}
		if it.Node.Name == "inner" && it.Node.Expanded {
			inner = true
		}
	}
	if !docs || !inner {
		t.Errorf("ancestors should be expanded: docs=%v inner=%v", docs, inner)
	}
}

func TestSwitchTabFollowsInSidebar(t *testing.T) {
	m := testModel(t,
		map[string]string{"a/one.md": "# 1", "b/two.md": "# 2"},
		"a/one.md", "b/two.md",
	)
	if got := cursorName(m); got != "two.md" {
		t.Fatalf("cursor = %q, want two.md (last opened)", got)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if got := cursorName(m); got != "one.md" {
		t.Errorf("cursor = %q, want one.md after tab switch", got)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if got := cursorName(m); got != "two.md" {
		t.Errorf("cursor = %q, want two.md after switching back", got)
	}
}

func TestRevealFilteredFileExpandsAncestorsOnly(t *testing.T) {
	m := testModel(t, map[string]string{"sub/notes.txt": "plain", "top.md": "# T"})
	before := cursorName(m)
	if err := m.openFile(filepath.Join(m.root.Path, "sub", "notes.txt")); err != nil {
		t.Fatal(err)
	}
	var subExpanded, txtVisible bool
	for _, it := range m.items {
		if it.Node.Name == "sub" && it.Node.Expanded {
			subExpanded = true
		}
		if it.Node.Name == "notes.txt" {
			txtVisible = true
		}
	}
	if !subExpanded {
		t.Error("sub should be expanded even though notes.txt is filtered out")
	}
	if txtVisible {
		t.Error("notes.txt must stay hidden in markdown-only mode")
	}
	if got := cursorName(m); got != before {
		t.Errorf("cursor moved to %q; it should stay on %q for a hidden file", got, before)
	}
}

func TestRevealOutsideRootLeavesSidebarAlone(t *testing.T) {
	m := testModel(t, map[string]string{"top.md": "# T"})
	outside := filepath.Join(t.TempDir(), "elsewhere.md")
	if err := os.WriteFile(outside, []byte("# E"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.openFile(outside); err != nil {
		t.Fatal(err)
	}
	if got := cursorName(m); got != "top.md" {
		t.Errorf("cursor = %q, want top.md untouched", got)
	}
}
