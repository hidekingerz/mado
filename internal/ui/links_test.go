package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	keyO     = tea.KeyPressMsg{Code: 'o', Text: "o"}
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
)

func TestSplitDest(t *testing.T) {
	tests := []struct {
		dest, path, frag string
		ok               bool
	}{
		{"other.md", "other.md", "", true},
		{"docs/a%20b.md#Setup", "docs/a b.md", "Setup", true},
		{"#usage", "", "usage", true},
		{"a.md?plain=1#x", "a.md", "x", true},
		{"file:///tmp/a.md#x", "/tmp/a.md", "x", true},
		{`C:\notes\a.md`, `C:\notes\a.md`, "", true},
		{"https://example.com/a.md", "", "", false},
		{"mailto:someone@example.com", "", "", false},
		{"//example.com/a.md", "", "", false},
		{"", "", "", false},
	}
	for _, tt := range tests {
		path, frag, ok := splitDest(tt.dest)
		if path != tt.path || frag != tt.frag || ok != tt.ok {
			t.Errorf("splitDest(%q) = %q, %q, %v; want %q, %q, %v", tt.dest, path, frag, ok, tt.path, tt.frag, tt.ok)
		}
	}
}

func TestCollectLinksListsLocalTargetsOnly(t *testing.T) {
	m := testModel(t, map[string]string{
		"README.md": strings.Join([]string{
			"# Top",
			"[guide](docs/guide.md) and [the web](https://example.com) and <https://example.org>",
			"[again](docs/guide.md) [setup](docs/guide.md#setup) [here](#top)",
			"![shot](docs/shot.png) [root]",
			"[gone](missing.md) [dir](docs/) [bare](docs/guide) [abs](/docs/guide.md#x)",
			"",
			"[root]: ./other.md",
		}, "\n"),
		"docs/guide.md":  "# Guide",
		"docs/README.md": "# Docs",
		"docs/shot.png":  "not really a png",
		"other.md":       "# Other",
	}, "README.md")
	root := m.root.Path
	p := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

	got := m.tabLinks(m.activeTab())
	want := []link{
		{text: "guide", path: p("docs/guide.md")},
		{text: "setup", path: p("docs/guide.md"), frag: "setup"},
		{text: "here", path: p("README.md"), frag: "top"},
		{text: "shot", path: p("docs/shot.png")},
		{text: "root", path: p("other.md")},
		{text: "gone", path: p("missing.md"), missing: true},
		{text: "dir", path: p("docs/README.md")},
		{text: "abs", path: p("docs/guide.md"), frag: "x"},
	}
	if len(got) != len(want) {
		t.Fatalf("links = %+v, want %d of them", got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("link %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLinksPanelOpensTheSelectedLinkInATab(t *testing.T) {
	m := testModel(t, map[string]string{
		"a.md":      "# A\n\n[first](b.md) then [second](sub/c.md)",
		"b.md":      "# B",
		"sub/c.md":  "# C",
		"unused.md": "# U",
	}, "a.md")

	m = update(t, m, keyO)
	if !m.links.open || len(m.links.items) != 2 {
		t.Fatalf("links open = %v with %d items, want the panel with 2", m.links.open, len(m.links.items))
	}
	view := m.render()
	for _, want := range []string{"Links", "first", "second", "sub/c.md", "[LINKS]"} {
		if !strings.Contains(view, want) {
			t.Errorf("screen lacks %q:\n%s", want, view)
		}
	}

	m = update(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = update(t, m, keyEnter)
	if m.links.open {
		t.Error("panel still open after following a link")
	}
	if len(m.tabs) != 2 || !strings.HasSuffix(m.activeTab().path, filepath.Join("sub", "c.md")) {
		t.Errorf("active tab = %q of %d, want sub/c.md as the second tab", m.activeTab().path, len(m.tabs))
	}
	if m.focus != focusContent {
		t.Error("focus is not on the content after following a link")
	}
}

func TestLinksPanelClosesOnEscAndKeepsTheTab(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "[b](b.md)", "b.md": "# B"}, "a.md")
	m = update(t, m, keyO)
	m = update(t, m, keyEsc)
	if m.links.open || len(m.tabs) != 1 || m.focus != focusContent {
		t.Errorf("open = %v, tabs = %d, focus = %v; want the panel closed and the tab as it was", m.links.open, len(m.tabs), m.focus)
	}
}

func TestLinksWithNoneSaysSoWithoutAPanel(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "see [the web](https://example.com)", "b.txt": "[b](a.md)"}, "a.md", "b.txt")
	for i := range m.tabs {
		m.active = i
		m = update(t, m, keyO)
		if m.links.open || m.notice != m.text.NoLinks {
			t.Errorf("tab %d: open = %v, notice = %q; want no panel and %q", i, m.links.open, m.notice, m.text.NoLinks)
		}
	}
}

func TestMissingLinkStaysInThePanelWithTheReason(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "[gone](gone.md)"}, "a.md")
	m = update(t, m, keyO)
	if view := m.render(); !strings.Contains(view, m.text.LinkMissing) {
		t.Errorf("screen does not mark the link as missing:\n%s", view)
	}
	m = update(t, m, keyEnter)
	if !m.links.open || len(m.tabs) != 1 || m.statusMsg == "" {
		t.Errorf("open = %v, tabs = %d, statusMsg = %q; want the panel open and the error shown", m.links.open, len(m.tabs), m.statusMsg)
	}
}

func TestAnchorLinkScrollsToItsHeading(t *testing.T) {
	filler := strings.Repeat("text\n\n", 60)
	for _, mode := range []viewMode{modeReader, modeSource} {
		m := testModel(t, map[string]string{
			"a.md": "# A\n\n[jump](b.md#second-part)\n\n[down](#far-below)\n\n" + filler + "## Far below\n\nend\n" + filler,
			"b.md": "# B\n\n" + filler + "## Second part\n\nbody\n" + filler,
		}, "a.md")
		m.mode = mode
		m.ensureRendered(m.activeTab())

		// Another file's heading.
		m = update(t, m, keyO)
		m = update(t, m, keyEnter)
		tab := m.activeTab()
		if !strings.HasSuffix(tab.path, "b.md") {
			t.Fatalf("%v: active tab = %q, want b.md", mode, tab.path)
		}
		if top := ansi.Strip(strings.Split(tab.content, "\n")[tab.vp.YOffset()]); !strings.Contains(top, "Second part") {
			t.Errorf("%v: top row of b.md = %q, want the heading", mode, top)
		}

		// A heading of the file itself: no new tab, just the scroll.
		m.active = 0
		m = update(t, m, keyO)
		m = update(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
		m = update(t, m, keyEnter)
		tab = m.activeTab()
		if len(m.tabs) != 2 || !strings.HasSuffix(tab.path, "a.md") {
			t.Fatalf("%v: active tab = %q of %d, want a.md of 2", mode, tab.path, len(m.tabs))
		}
		if top := ansi.Strip(strings.Split(tab.content, "\n")[tab.vp.YOffset()]); !strings.Contains(top, "Far below") {
			t.Errorf("%v: top row of a.md = %q, want the heading", mode, top)
		}
	}
}

func TestHeadingLine(t *testing.T) {
	raw := "# Setup\n\ntext\n\n## Setup\n\n### 日本語の 見出し!\n\n## With `code` inside\n"
	tests := []struct {
		anchor string
		line   int
	}{
		{"setup", 1},
		{"Setup-1", 5},
		{"日本語の-見出し", 7},
		{"with-code-inside", 9},
		{"nope", 0},
	}
	for _, tt := range tests {
		if line, _ := headingLine(raw, tt.anchor); line != tt.line {
			t.Errorf("headingLine(%q) = %d, want %d", tt.anchor, line, tt.line)
		}
	}
}

func TestLinksPanelClickOpensTheRow(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "[b](b.md) [c](c.md)", "b.md": "# B", "c.md": "# C"}, "a.md")
	m = update(t, m, keyO)
	// The list starts below the tab bar, the pane's border and the header.
	y := tabBarHeight + 1 + linksHeaderRows + 1
	m = update(t, m, tea.MouseClickMsg{X: m.sidebarWidth() + 4, Y: y, Button: tea.MouseLeft})
	if m.links.open || !strings.HasSuffix(m.activeTab().path, "c.md") {
		t.Errorf("open = %v, active = %q; want c.md opened by the click", m.links.open, m.activeTab().path)
	}
}
