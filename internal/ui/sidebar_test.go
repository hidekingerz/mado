package ui

import (
	"strings"
	"testing"
)

func TestShowHiddenLeavesOutSearchExcludes(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", ".git/HEAD": "ref", ".notes.md": "# N"})
	m = update(t, m, keyRune('.'))
	var got []string
	for _, it := range m.items {
		got = append(got, it.Node.Name)
	}
	s := strings.Join(got, " ")
	if strings.Contains(" "+s+" ", " .git ") {
		t.Errorf("tree = %q, .git is in the default search exclude", s)
	}
	if !strings.Contains(s, ".notes.md") {
		t.Errorf("tree = %q, want dotfiles shown", s)
	}
}
