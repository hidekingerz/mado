package ui

import (
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
