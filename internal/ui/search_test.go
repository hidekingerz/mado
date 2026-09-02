package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/search"
)

// updateCmd is update, but keeps the command the model returned.
func updateCmd(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// runCmd executes a command synchronously and feeds its message back.
func runCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("the command produced no message")
	}
	return update(t, m, msg)
}

func keyRune(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

// typeQuery types s into the search prompt and waits for the results.
func typeQuery(t *testing.T, m Model, s string) Model {
	t.Helper()
	var cmd tea.Cmd
	for _, r := range s {
		if r == ' ' {
			m, cmd = updateCmd(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		} else {
			m, cmd = updateCmd(t, m, keyRune(r))
		}
	}
	return runCmd(t, m, cmd)
}

func resultRels(m Model) string {
	var out []string
	for _, r := range m.search.results {
		out = append(out, r.Rel)
	}
	return strings.Join(out, " ")
}

func TestSearchKeyOpensNamePanelAndFilters(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "docs/plan.md": "# P", "docs/notes.md": "# N"})
	m, cmd := updateCmd(t, m, keyRune('/'))
	if !m.search.open || m.search.target != search.Names {
		t.Fatalf("search open = %v target = %v, want open names", m.search.open, m.search.target)
	}
	if cmd != nil {
		t.Error("an empty query should not start a search")
	}
	if v := m.View(); !strings.Contains(v, "Search names") || !strings.Contains(v, "Type to search file names") {
		t.Errorf("panel should show its title and a hint:\n%s", v)
	}

	m = typeQuery(t, m, "plan")
	if got := resultRels(m); got != "docs/plan.md" {
		t.Errorf("results = %q, want docs/plan.md", got)
	}
	if v := m.View(); !strings.Contains(v, "docs/plan.md") || !strings.Contains(v, "1 match") {
		t.Errorf("panel should list the match and count it:\n%s", v)
	}
	if !strings.Contains(m.renderStatusBar(), "[SEARCH NAMES]") {
		t.Errorf("status bar should say a search is on: %q", m.renderStatusBar())
	}
}

func TestSearchEnterOpensTheSelectedFile(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "docs/plan.md": "# P"})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = typeQuery(t, m, "plan")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.search.open {
		t.Error("opening a result should close the panel")
	}
	if len(m.tabs) != 1 || m.tabs[0].title != "plan.md" {
		t.Fatalf("tabs = %+v, want plan.md open", m.tabs)
	}
	if m.focus != focusContent {
		t.Error("the opened file should have the focus")
	}
	if m.items[m.cursor].Node.Path != m.tabs[0].path {
		t.Error("the sidebar should follow the opened file")
	}
}

func TestSearchArrowsMoveTheSelection(t *testing.T) {
	m := testModel(t, map[string]string{"n1.md": "", "n2.md": "", "n3.md": ""})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = typeQuery(t, m, "n")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.search.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.search.cursor)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.tabs) != 1 || m.tabs[0].title != "n2.md" {
		t.Errorf("tabs = %+v, want n2.md", m.tabs)
	}
}

func TestSearchTypedQuitKeyIsPartOfTheQuery(t *testing.T) {
	m := testModel(t, map[string]string{"quick.md": "# Q"})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = typeQuery(t, m, "q")
	if !m.search.open || m.search.query != "q" {
		t.Fatalf("open = %v query = %q; typing q must not quit", m.search.open, m.search.query)
	}
	if got := resultRels(m); got != "quick.md" {
		t.Errorf("results = %q", got)
	}
	// ? and the other single-key actions are typed too.
	m = typeQuery(t, m, "?")
	if m.showHelp || m.search.query != "q?" {
		t.Errorf("help = %v query = %q", m.showHelp, m.search.query)
	}

	_, cmd := updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should still quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c should produce tea.Quit")
	}
}

func TestSearchContentKeyAndTabSwitchTargets(t *testing.T) {
	m := testModel(t, map[string]string{
		"a.md": "# A\n\nnothing\nthe needle is here\n",
		"b.md": "# needle\n",
	})
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlF})
	if !m.search.open || m.search.target != search.Contents {
		t.Fatalf("open = %v target = %v, want open contents", m.search.open, m.search.target)
	}
	m = typeQuery(t, m, "needle")
	if got := resultRels(m); got != "a.md b.md" {
		t.Fatalf("results = %q", got)
	}
	if r := m.search.results[0]; r.Line != 4 || r.Text != "the needle is here" {
		t.Errorf("first match = %+v", r)
	}
	v := m.View()
	if !strings.Contains(v, "a.md:4") || !strings.Contains(v, "the needle is here") {
		t.Errorf("panel should show path, line and text:\n%s", v)
	}

	m, cmd := updateCmd(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.search.target != search.Names {
		t.Fatal("tab should switch to names")
	}
	m = runCmd(t, m, cmd)
	if got := resultRels(m); got != "" {
		t.Errorf("no file is named needle, got %q", got)
	}
	m, cmd = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m = runCmd(t, m, cmd)
	if m.search.target != search.Contents || len(m.search.results) != 2 {
		t.Errorf("tab again: target = %v results = %d", m.search.target, len(m.search.results))
	}
	// The content-search key switches target while the panel is open.
	m, cmd = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m = runCmd(t, m, cmd)
	m, cmd = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlF})
	m = runCmd(t, m, cmd)
	if m.search.target != search.Contents {
		t.Error("ctrl+f inside the panel should switch to contents")
	}
}

func TestSearchContentMatchScrollsSourceToItsLine(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		if i == 60 {
			b.WriteString("the needle line\n")
		} else {
			b.WriteString("line\n")
		}
	}
	cfg := config.Default()
	cfg.Theme.Style = "notty"
	cfg.Theme.DefaultMode = "source"
	m := testModelWithConfig(t, cfg, map[string]string{"a.md": b.String()})
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlF})
	m = typeQuery(t, m, "needle")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	tab := m.activeTab()
	if tab == nil || tab.title != "a.md" {
		t.Fatalf("active tab = %+v", tab)
	}
	if tab.vp.YOffset != 59 {
		t.Errorf("YOffset = %d, want 59 (line 60 at the top)", tab.vp.YOffset)
	}
	if top := strings.SplitN(tab.vp.View(), "\n", 2)[0]; !strings.Contains(top, "needle") {
		t.Errorf("top row = %q, want the match", top)
	}
}

func TestSearchContentMatchScrollsRenderedMarkdown(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Title\n\n")
	for i := 0; i < 80; i++ {
		b.WriteString("Paragraph text.\n\n")
	}
	b.WriteString("Here is the needle paragraph.\n\n")
	for i := 0; i < 20; i++ {
		b.WriteString("Trailing text.\n\n")
	}
	m := testModel(t, map[string]string{"a.md": b.String()})
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlF})
	m = typeQuery(t, m, "needle")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	tab := m.activeTab()
	if tab == nil || m.mode != modeReader {
		t.Fatalf("tab = %+v mode = %v", tab, m.mode)
	}
	if tab.vp.YOffset == 0 {
		t.Fatal("the viewport should have scrolled to the match")
	}
	if top := strings.SplitN(tab.vp.View(), "\n", 2)[0]; !strings.Contains(top, "needle") {
		t.Errorf("top row = %q, want the match", top)
	}
}

func TestSourceRowFollowsWrappingAndLineNumbers(t *testing.T) {
	raw := "short\n" + strings.Repeat("word ", 20) + "\nthird\n"
	// Width 30: the 100-column second line wraps onto 4 rows.
	if got := sourceRow(raw, 3, 30, false); got != 5 {
		t.Errorf("row of line 3 = %d, want 5", got)
	}
	// With a gutter ("3 " = 2 columns) the text width is 28: 4 rows still.
	if got := sourceRow(raw, 3, 30, true); got != 5 {
		t.Errorf("row of line 3 with numbers = %d, want 5", got)
	}
	if got := sourceRow(raw, 1, 30, false); got != 0 {
		t.Errorf("row of line 1 = %d, want 0", got)
	}
	if got := sourceRow(raw, 99, 30, false); got != -1 {
		t.Errorf("row of a missing line = %d, want -1", got)
	}
}

func TestSearchEscRestoresTheFocus(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	if m.focus != focusContent {
		t.Fatal("setup: focus should be content")
	}
	m, _ = updateCmd(t, m, keyRune('/'))
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.search.open || m.focus != focusContent {
		t.Errorf("open = %v focus = %v, want closed/content", m.search.open, m.focus)
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyEsc}) // back to the sidebar
	m, _ = updateCmd(t, m, keyRune('/'))
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.focus != focusSidebar {
		t.Errorf("focus = %v, want sidebar", m.focus)
	}
}

func TestSearchKeepsTheQueryAcrossReopen(t *testing.T) {
	m := testModel(t, map[string]string{"plan.md": "# P"})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = typeQuery(t, m, "plan")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m, cmd := updateCmd(t, m, keyRune('/'))
	if m.search.query != "plan" {
		t.Fatalf("query = %q, want plan", m.search.query)
	}
	m = runCmd(t, m, cmd)
	if got := resultRels(m); got != "plan.md" {
		t.Errorf("reopening should search again, got %q", got)
	}
}

func TestSearchEditingKeys(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": ""})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = typeQuery(t, m, "one two")
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.search.query != "one tw" {
		t.Errorf("after backspace = %q", m.search.query)
	}
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlW})
	if m.search.query != "one " {
		t.Errorf("after ctrl+w = %q", m.search.query)
	}
	m, cmd := updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.search.query != "" || cmd != nil {
		t.Errorf("after ctrl+u = %q (cmd %v)", m.search.query, cmd != nil)
	}
	if len(m.search.results) != 0 {
		t.Error("clearing the query should clear the results")
	}
	// A pasted newline is dropped, the rest kept.
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\nb"), Paste: true})
	if m.search.query != "ab" {
		t.Errorf("after paste = %q", m.search.query)
	}
}

func TestSearchDropsResultsOfAnOlderQuery(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "", "b.md": ""})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = typeQuery(t, m, "b")
	stale := searchResultMsg{gen: m.search.gen - 1, res: search.Result{Matches: []search.Match{{Rel: "a.md"}}}}
	m = update(t, m, stale)
	if got := resultRels(m); got != "b.md" {
		t.Errorf("results = %q, want the current query's", got)
	}
}

func TestSearchMouseClickOpensAResult(t *testing.T) {
	m := testModel(t, map[string]string{"n1.md": "", "n2.md": ""})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = typeQuery(t, m, "n")
	// The list starts below the tab bar, the pane border and the header.
	y := tabBarHeight + 1 + searchHeaderRows + 1
	m = update(t, m, tea.MouseMsg{X: m.sidebarWidth() + 3, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if len(m.tabs) != 1 || m.tabs[0].title != "n2.md" {
		t.Errorf("tabs = %+v, want n2.md", m.tabs)
	}
}

func TestSearchClickOnTheSidebarClosesThePanel(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = update(t, m, tea.MouseMsg{X: 3, Y: 3, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.search.open {
		t.Error("clicking the tree should close the search")
	}
	if len(m.tabs) != 1 || m.tabs[0].title != "b.md" {
		t.Errorf("tabs = %+v, want b.md opened by the click", m.tabs)
	}
}

func TestSearchHonoursTheConfiguredExclusions(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.Style = "notty"
	cfg.Search.Exclude = []string{"node_modules", "docs/drafts"}
	m := testModelWithConfig(t, cfg, map[string]string{
		"plan.md":                  "the plan",
		"node_modules/x/plan.md":   "the plan",
		"docs/drafts/plan.md":      "the plan",
		"docs/plan.md":             "the plan",
		"other/node_modules/pl.md": "the plan",
	})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = typeQuery(t, m, "plan")
	if got := resultRels(m); got != "plan.md docs/plan.md" {
		t.Errorf("names = %q", got)
	}
	m, cmd := updateCmd(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m = runCmd(t, m, cmd)
	if got := resultRels(m); got != "docs/plan.md plan.md" {
		t.Errorf("contents = %q", got)
	}
}

func TestSearchFollowsTheSidebarFilter(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "needle", "a.txt": "needle"})
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlF})
	m = typeQuery(t, m, "needle")
	if got := resultRels(m); got != "a.md" {
		t.Fatalf("markdown only: %q", got)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = update(t, m, keyRune('a')) // show all files
	m, cmd := updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlF})
	m = runCmd(t, m, cmd)
	if got := resultRels(m); got != "a.md a.txt" {
		t.Errorf("all files: %q", got)
	}
}

func TestSearchResultsCannotDriveTheTerminal(t *testing.T) {
	m := testModel(t, map[string]string{
		"a.md":              "needle " + osc52 + " tail\n",
		"n" + osc52 + ".md": "",
	})
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlF})
	m = typeQuery(t, m, "needle")
	if v := m.View(); strings.Contains(v, "\x1b]52") {
		t.Errorf("a matching line reached the terminal unescaped:\n%q", v)
	}
	m, cmd := updateCmd(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m = runCmd(t, m, cmd)
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeQuery(t, m, "n")
	if len(m.search.results) == 0 {
		t.Fatal("the escape-named file should match")
	}
	if v := m.View(); strings.Contains(v, "\x1b]52") {
		t.Errorf("a file name reached the terminal unescaped:\n%q", v)
	}
	// The typed query is echoed; an escape typed into it is shown too.
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(osc52), Paste: true})
	if v := m.View(); strings.Contains(v, "\x1b]52") {
		t.Errorf("a pasted escape reached the terminal:\n%q", v)
	}
}

func TestSearchSnippetKeepsTheHitInView(t *testing.T) {
	line := "\t" + strings.Repeat("x", 60) + " needle " + strings.Repeat("y", 10)
	got := searchSnippet(line, "needle", 30)
	if !strings.HasPrefix(got, "…") || !strings.Contains(got, "needle") {
		t.Errorf("snippet = %q, want the hit visible after an ellipsis", got)
	}
	if strings.Contains(got, "\t") {
		t.Errorf("snippet keeps a tab: %q", got)
	}
	if got := searchSnippet("  short   needle", "needle", 30); got != "short needle" {
		t.Errorf("snippet = %q, want it trimmed and whole", got)
	}
}

func TestTruncateLeftKeepsTheTail(t *testing.T) {
	if got := truncateLeft("docs/deep/dir/file.md", 10); got != "…r/file.md" {
		t.Errorf("truncateLeft = %q", got)
	}
	if got := truncateLeft("short", 10); got != "short" {
		t.Errorf("truncateLeft = %q, want unchanged", got)
	}
	if got := truncateLeft("x", 0); got != "" {
		t.Errorf("truncateLeft = %q, want empty", got)
	}
}

func TestHelpListsTheSearchKeys(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune('?'))
	v := m.View()
	if !strings.Contains(v, "search file names") || !strings.Contains(v, "search file contents") {
		t.Errorf("help should list the search actions:\n%s", v)
	}
}

func TestRemoteOpenClosesTheSearch(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"})
	m, srv := serveModel(t, m)
	m, _ = updateCmd(t, m, keyRune('/'))
	m, err := sendRemote(t, m, srv, "open", filepath.Join(m.root.Path, "b.md"))
	if err != nil {
		t.Fatal(err)
	}
	if m.search.open {
		t.Error("a remotely opened file should replace the search panel")
	}
}

func TestSearchPanelFitsThePane(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 40; i++ {
		files[filepath.Join("some", "rather", "deeply", "nested", "directory", strings.Repeat("n", i+1)+".md")] = strings.Repeat("needle ", 40)
	}
	m := testModel(t, files)
	m, _ = updateCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlF})
	m = typeQuery(t, m, "needle")
	lines := strings.Split(m.View(), "\n")
	if len(lines) != m.height {
		t.Errorf("view has %d lines, want %d — a long row wrapped", len(lines), m.height)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnd})
	if m.search.cursor != len(m.search.results)-1 {
		t.Errorf("end: cursor = %d", m.search.cursor)
	}
	if lines := strings.Split(m.View(), "\n"); len(lines) != m.height {
		t.Errorf("scrolled view has %d lines, want %d", len(lines), m.height)
	}
	if _, err := os.Stat(m.root.Path); err != nil {
		t.Fatal(err)
	}
}
