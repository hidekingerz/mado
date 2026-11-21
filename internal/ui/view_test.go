package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The help overlay is a two-column table: every key must start in the
// same column, however wide the pane is and however long each row's
// description happens to be (#46).
func TestHelpKeysShareOneColumn(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, tea.WindowSizeMsg{Width: 160, Height: 50})
	m = update(t, m, keyRune('?'))
	starts := map[int][]string{}
	for _, l := range strings.Split(ansi.Strip(m.render()), "\n") {
		for _, key := range []string{"up/k ", "m  ", "ctrl+f ", "q/ctrl+c "} {
			if i := strings.Index(l, " "+key); i >= 0 && strings.HasPrefix(l, "│") {
				starts[ansi.StringWidth(l[:i])] = append(starts[ansi.StringWidth(l[:i])], strings.TrimSpace(key))
			}
		}
	}
	if len(starts) != 1 {
		t.Errorf("help keys start in %d different columns: %v", len(starts), starts)
	}
}

func TestStatusBarShowsFilePathRelativeToRoot(t *testing.T) {
	m := testModel(t, map[string]string{"docs/guide/plan.md": "# P"}, "docs/guide/plan.md")
	bar := ansi.Strip(m.renderStatusBar())
	if !strings.Contains(bar, "docs/guide/plan.md") {
		t.Errorf("status bar should name the file from the root: %q", bar)
	}
	if strings.Contains(bar, m.root.Path) {
		t.Errorf("status bar should not repeat the root's absolute path: %q", bar)
	}
}

func TestStatusBarCutsThePathBeforeTheScrollPercent(t *testing.T) {
	m := testModel(t, map[string]string{"docs/guide/a-rather-long-file-name.md": "# P"}, "docs/guide/a-rather-long-file-name.md")
	m = update(t, m, tea.WindowSizeMsg{Width: 50, Height: 24})
	bar := ansi.Strip(m.renderStatusBar())
	if !strings.Contains(bar, "%") || !strings.Contains(bar, "[READER]") {
		t.Errorf("mode and scroll percent should survive a narrow bar: %q", bar)
	}
	if !strings.Contains(bar, "-name.md") {
		t.Errorf("the file name end of the path should survive: %q", bar)
	}
	if !strings.Contains(bar, "quit") {
		t.Errorf("right-hand hints should survive: %q", bar)
	}
}

func TestStatusBarShortensRootWithTilde(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	t.Setenv("HOME", filepath.Dir(m.root.Path))
	bar := ansi.Strip(m.renderStatusBar())
	want := "~" + string(filepath.Separator) + filepath.Base(m.root.Path)
	if !strings.Contains(bar, want) {
		t.Errorf("status bar = %q, want %q", bar, want)
	}
}

func TestSearchStatusBarKeepsCountInHeaderOnly(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "docs/plan.md": "# P"})
	m = update(t, m, tea.WindowSizeMsg{Width: 200, Height: 30})
	m, _ = updateCmd(t, m, keyRune('/'))
	m = typeQuery(t, m, "plan")
	if bar := ansi.Strip(m.renderStatusBar()); strings.Contains(bar, "match") {
		t.Errorf("the match count already sits in the panel header: %q", bar)
	}
}
