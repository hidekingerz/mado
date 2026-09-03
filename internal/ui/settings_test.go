package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hidekingerz/mado/internal/config"
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
	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if got := selectedField(m); got != "style" {
		t.Errorf("two downs from watch should skip [theme] and land on style, got %q", got)
	}
	m = update(t, m, keyRune('k'))
	m = update(t, m, keyRune('k'))
	if got := selectedField(m); got != "watch" {
		t.Errorf("k twice should move back up to watch, got %q", got)
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

func TestSettingsCtrlCQuitsEvenWhenQuitIsRebound(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.Style = "notty" // deterministic in tests, no terminal probing
	cfg.Keys.Quit = []string{"Q"}
	m := testModelWithConfig(t, cfg, map[string]string{"a.md": "# A"}, "a.md")
	m = update(t, m, keyRune(','))
	if !m.settings.open {
		t.Fatal("',' should open the settings panel")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should quit even when quit is rebound off ctrl+c")
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
	// header (2). Rows: [general], watch, mermaid, [theme], style.
	top := tabBarHeight + 1 + settingsHeaderRows
	m = update(t, m, tea.MouseMsg{X: x, Y: top + 4, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if got := selectedField(m); got != "style" {
		t.Errorf("click on row 4 should select style, got %q", got)
	}
	m = update(t, m, tea.MouseMsg{X: x, Y: top + 3, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if got := selectedField(m); got != "style" {
		t.Errorf("clicking a heading changes nothing, got %q", got)
	}
	m = update(t, m, tea.MouseMsg{X: x, Y: top, Button: tea.MouseButtonWheelDown})
	if m.settings.off != wheelStep {
		t.Errorf("wheel should scroll the list, off = %d", m.settings.off)
	}
}

// checkSettingsFitsPane asserts that every line of m.View() is no wider
// than the pane and that there are exactly m.height lines: a row whose
// plain layout does not fit w gets word-wrapped by the outer lipgloss
// box, which both can miss (a wrapped line can still measure <= width)
// but which always grows the frame taller than m.height.
func checkSettingsFitsPane(t *testing.T, m Model) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	for _, line := range lines {
		if w := lipgloss.Width(line); w > m.width {
			t.Errorf("line is %d wide, pane is %d: %q", w, m.width, line)
		}
	}
	if len(lines) != m.height {
		t.Errorf("View() has %d lines, want %d (a wrapped row grows the frame)", len(lines), m.height)
	}
}

func TestSettingsRowsFitThePane(t *testing.T) {
	// At width 44 with the cursor on the last field ("help"), the
	// selected row's own truncation is exercised.
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, tea.WindowSizeMsg{Width: 44, Height: 12})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnd})
	checkSettingsFitsPane(t, m)
}

func TestSettingsUnselectedLongKeyFitsThePane(t *testing.T) {
	// At width 40 the content pane is only 18 columns, narrower than
	// "toggle_line_numbers" (the longest key). With the cursor moved
	// past it — onto "search" — that row is unselected, which is what
	// exercises the plain (non-cursor) row layout: it must not push
	// the row wider than the pane and get word-wrapped onto a second
	// line by the outer box.
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 12})
	m = moveTo(t, m, "search")
	if selectedField(m) != "search" {
		t.Fatalf("cursor should be on search, got %q", selectedField(m))
	}
	checkSettingsFitsPane(t, m)
}

// checkSettingsRowWidths asserts that every row renderSettings itself
// produces (before the outer bordered box gets a chance to word-wrap
// or silently clip an overlong single line) is no wider than the
// content pane. This is what actually catches a row that overflows
// but happens not to contain a wrappable second word: the outer box
// can crop such a line to size without growing the frame, so
// checkSettingsFitsPane alone would miss it.
func checkSettingsRowWidths(t *testing.T, m Model) {
	t.Helper()
	w := m.contentInnerWidth()
	body := m.renderSettings(w, m.contentInnerHeight())
	for _, line := range strings.Split(body, "\n") {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("settings row is %d wide, pane is %d: %q", lw, w, line)
		}
	}
}

func TestSettingsTextPromptFitsThePane(t *testing.T) {
	// The text-prompt row must not escape the pane at a width narrower
	// than the key column: name is never clipped, so this exercises
	// the row's own clamping rather than the outer word-wrap.
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 12})
	m = moveTo(t, m, "accent_color")
	m = update(t, m, enter())
	if m.settings.editing != editText {
		t.Fatalf("editing = %v, want editText", m.settings.editing)
	}
	checkSettingsRowWidths(t, m)
	checkSettingsFitsPane(t, m)
	m = typeText(t, m, "12345678901234567890")
	checkSettingsRowWidths(t, m)
	checkSettingsFitsPane(t, m)
}

func TestSettingsCaptureFitsThePane(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 12})
	m = moveTo(t, m, "next_tab")
	m = update(t, m, enter())
	if m.settings.editing != editCapture {
		t.Fatalf("editing = %v, want editCapture", m.settings.editing)
	}
	checkSettingsRowWidths(t, m)
	checkSettingsFitsPane(t, m)
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

// typeText types s into the settings prompt.
func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		if r == ' ' {
			m = update(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		} else {
			m = update(t, m, keyRune(r))
		}
	}
	return m
}

func savedConfig(t *testing.T, m Model) string {
	t.Helper()
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		t.Fatalf("config not saved: %v", err)
	}
	return string(data)
}

func TestSettingsToggleBoolAppliesAndSaves(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A", "notes.txt": "n"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "show_all_files")
	m = update(t, m, enter())
	if !m.cfg.Sidebar.ShowAllFiles {
		t.Fatal("enter should toggle the value on")
	}
	if !strings.Contains(m.View(), "notes.txt") {
		t.Error("the tree should reload with all files")
	}
	if got := savedConfig(t, m); got != "[sidebar]\nshow_all_files = true\n" {
		t.Errorf("saved:\n%s", got)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeySpace})
	if m.cfg.Sidebar.ShowAllFiles {
		t.Fatal("space should toggle the value off")
	}
	if got := savedConfig(t, m); got != "[sidebar]\nshow_all_files = false\n" {
		t.Errorf("saved:\n%s", got)
	}
}

func TestSettingsWatchToggleWaitsOnTheWatcher(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"}, "a.md")
	t.Cleanup(func() {
		if m.watcher != nil {
			m.watcher.Close()
		}
	})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "watch")
	var cmd tea.Cmd
	m, cmd = updateCmd(t, m, enter())
	if m.watcher == nil || cmd == nil {
		t.Fatalf("watch on: watcher = %v cmd = %v", m.watcher != nil, cmd != nil)
	}
	m = update(t, m, enter())
	if m.watcher != nil {
		t.Error("watch off should stop the watcher")
	}
}

func TestSettingsEnumCyclesWithArrows(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "default_mode")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.cfg.Theme.DefaultMode != "source" || m.mode != modeSource {
		t.Fatalf("right: default_mode = %q mode = %v", m.cfg.Theme.DefaultMode, m.mode)
	}
	if !strings.Contains(savedConfig(t, m), "default_mode = \"source\"") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.cfg.Theme.DefaultMode != "reader" {
		t.Error("right past the last option wraps to the first")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.cfg.Theme.DefaultMode != "source" {
		t.Error("left cycles backwards")
	}
	m = update(t, m, enter())
	if m.cfg.Theme.DefaultMode != "reader" {
		t.Error("enter goes forward")
	}
}

func TestSettingsStyleCustomOpensAPrompt(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "style")
	// The test config starts at notty; ascii is next, then custom….
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.cfg.Theme.Style != "ascii" {
		t.Fatalf("style = %q, want ascii", m.cfg.Theme.Style)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.settings.editing != editText || m.settings.input != "" {
		t.Fatalf("custom… should open an empty prompt: editing = %v input = %q", m.settings.editing, m.settings.input)
	}
	if m.cfg.Theme.Style != "ascii" {
		t.Error("nothing applies until a path is entered")
	}
	path := filepath.Join(t.TempDir(), "my.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = typeText(t, m, path)
	m = update(t, m, enter())
	if m.cfg.Theme.Style != path || m.settings.editing != editNone {
		t.Fatalf("style = %q editing = %v", m.cfg.Theme.Style, m.settings.editing)
	}
	if !strings.Contains(savedConfig(t, m), "style = \""+path+"\"") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.cfg.Theme.Style != "ascii" {
		t.Errorf("left from a custom path goes to the option before custom…, got %q", m.cfg.Theme.Style)
	}
}

func TestSettingsTextPromptValidatesAndKeepsInput(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "accent_color")
	m = update(t, m, enter())
	if m.settings.editing != editText || m.settings.input != "#7C6AEF" {
		t.Fatalf("the prompt should start with the current value: editing = %v input = %q", m.settings.editing, m.settings.input)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "#12")
	m = update(t, m, enter())
	if m.settings.editing != editText {
		t.Error("a rejected value keeps the prompt open")
	}
	if !strings.Contains(m.renderStatusBar(), "accent_color") {
		t.Errorf("status bar should say why: %q", m.renderStatusBar())
	}
	if m.cfg.Theme.AccentColor != "#7C6AEF" {
		t.Error("a rejected value must not apply")
	}
	m = typeText(t, m, "3456")
	m = update(t, m, enter())
	if m.cfg.Theme.AccentColor != "#123456" || m.settings.editing != editNone {
		t.Fatalf("accent = %q editing = %v", m.cfg.Theme.AccentColor, m.settings.editing)
	}
	if got := m.styles.accent.GetForeground(); got != lipgloss.Color("#123456") {
		t.Errorf("accent style = %v, want #123456", got)
	}
	if !strings.Contains(savedConfig(t, m), "accent_color = \"#123456\"") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, enter())
	m = typeText(t, m, "zzz")
	m = update(t, m, esc())
	if m.settings.editing != editNone || m.cfg.Theme.AccentColor != "#123456" {
		t.Error("esc cancels the prompt and leaves the value")
	}
}

func TestSettingsPromptEditingKeys(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "exclude")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "dist *.log")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.settings.input != "dist *.lo" {
		t.Errorf("backspace: input = %q", m.settings.input)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlW})
	if m.settings.input != "dist " {
		t.Errorf("ctrl+w: input = %q", m.settings.input)
	}
}

func TestSettingsWidthAndExclude(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "width")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "40")
	m = update(t, m, enter())
	if m.sidebarWidth() != 40 {
		t.Errorf("sidebar width = %d, want 40", m.sidebarWidth())
	}
	if !strings.Contains(savedConfig(t, m), "width = 40") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}

	m = moveTo(t, m, "exclude")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "dist *.log")
	m = update(t, m, enter())
	if !reflect.DeepEqual(m.cfg.Search.Exclude, []string{"dist", "*.log"}) {
		t.Errorf("exclude = %v", m.cfg.Search.Exclude)
	}
	if !strings.Contains(savedConfig(t, m), "exclude = [\"dist\", \"*.log\"]") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = update(t, m, enter())
	if len(m.cfg.Search.Exclude) != 0 {
		t.Errorf("an empty prompt clears the list, got %v", m.cfg.Search.Exclude)
	}
	if !strings.Contains(savedConfig(t, m), "exclude = []") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
}

func TestSettingsSaveFailureKeepsTheChange(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m.configPath = t.TempDir() // a directory: reading it as a file fails
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "show_hidden")
	m = update(t, m, enter())
	if !m.cfg.Sidebar.ShowHidden {
		t.Error("the change should stay applied")
	}
	if !strings.Contains(m.renderStatusBar(), "save failed") {
		t.Errorf("status bar = %q, want a save failure", m.renderStatusBar())
	}
}

func TestSettingsWithoutConfigPathWarns(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	if !strings.Contains(m.View(), "not saved") {
		t.Error("the panel should say changes are not saved")
	}
	m = moveTo(t, m, "show_hidden")
	m = update(t, m, enter())
	if !m.cfg.Sidebar.ShowHidden {
		t.Error("the change should still apply for the session")
	}
	if !strings.Contains(m.renderStatusBar(), "not saved") {
		t.Errorf("status bar = %q", m.renderStatusBar())
	}
}

func TestSettingsPreservesTheRestOfTheFile(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	src := "# mine\n[theme]\nstyle = \"notty\" # keep\n\n[sidebar]\nwidth = 32\n"
	if err := os.WriteFile(m.configPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "width")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "48")
	m = update(t, m, enter())
	want := "# mine\n[theme]\nstyle = \"notty\" # keep\n\n[sidebar]\nwidth = 48\n"
	if got := savedConfig(t, m); got != want {
		t.Errorf("saved:\n%s\nwant:\n%s", got, want)
	}
}

func TestSettingsCaptureBindsAKey(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "next_tab")
	m = update(t, m, enter())
	if m.settings.editing != editCapture {
		t.Fatalf("enter should start a capture, editing = %v", m.settings.editing)
	}
	if !strings.Contains(m.View(), "press a key") || !strings.Contains(m.renderStatusBar(), "press a key") {
		t.Error("the row and the status bar should ask for a key")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.settings.editing != editNone {
		t.Error("one press ends the capture")
	}
	if !reflect.DeepEqual(m.cfg.Keys.NextTab, []string{"tab", "]", "ctrl+n"}) {
		t.Errorf("next_tab = %v", m.cfg.Keys.NextTab)
	}
	if !strings.Contains(savedConfig(t, m), "next_tab = [\"tab\", \"]\", \"ctrl+n\"]") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, esc())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.active != 0 {
		t.Error("the new key should switch tabs as soon as the panel closes")
	}
}

func TestSettingsCaptureRefusesAKeyAnotherActionHas(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "reload")
	m = update(t, m, enter())
	m = update(t, m, keyRune('q'))
	if m.settings.editing != editNone {
		t.Error("a refused key still ends the capture")
	}
	if !strings.Contains(m.renderStatusBar(), "quit") {
		t.Errorf("status bar should name the owner: %q", m.renderStatusBar())
	}
	if !reflect.DeepEqual(m.cfg.Keys.Reload, []string{"r", "f5"}) {
		t.Errorf("reload = %v, want unchanged", m.cfg.Keys.Reload)
	}
	if _, err := os.Stat(m.configPath); !os.IsNotExist(err) {
		t.Error("nothing should be saved for a refused key")
	}
}

func TestSettingsCaptureTakesEscAsAKey(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "back")
	m = update(t, m, enter())
	m = update(t, m, esc())
	if !m.settings.open {
		t.Fatal("esc during a capture is the key, not a close")
	}
	if !strings.Contains(m.renderStatusBar(), "already bound to back") {
		t.Errorf("esc reached the binding and was refused as a duplicate: %q", m.renderStatusBar())
	}
}

func TestSettingsSpaceCannotBeBound(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "help")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if !strings.Contains(m.renderStatusBar(), "space") {
		t.Errorf("status bar = %q", m.renderStatusBar())
	}
	if !reflect.DeepEqual(m.cfg.Keys.Help, []string{"?"}) {
		t.Errorf("help = %v", m.cfg.Keys.Help)
	}
}

func TestSettingsBackspaceRemovesTheLastKey(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "reload")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !reflect.DeepEqual(m.cfg.Keys.Reload, []string{"r"}) {
		t.Errorf("reload = %v, want [r]", m.cfg.Keys.Reload)
	}
	if !strings.Contains(savedConfig(t, m), "reload = [\"r\"]") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !reflect.DeepEqual(m.cfg.Keys.Reload, []string{"r"}) {
		t.Error("the last key stays")
	}
	if !strings.Contains(m.renderStatusBar(), "at least one key") {
		t.Errorf("status bar = %q", m.renderStatusBar())
	}
}
