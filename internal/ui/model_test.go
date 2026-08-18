package ui

import (
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
