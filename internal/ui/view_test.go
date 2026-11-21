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

// A directory with nothing the filter lets through should say why the
// sidebar is empty and how to show more (#44).
func TestEmptySidebarExplainsItself(t *testing.T) {
	m := testModel(t, map[string]string{"main.go": "package main", ".env": "X=1"})
	v := ansi.Strip(m.render())
	if !strings.Contains(v, "No markdown files") {
		t.Errorf("sidebar and welcome should say there is no markdown:\n%s", v)
	}
	if !strings.Contains(v, "show all files") || !strings.Contains(v, "show dotfiles") {
		t.Errorf("welcome should name the keys that show more:\n%s", v)
	}

	m = update(t, m, keyRune('a'))
	m = update(t, m, keyRune('.'))
	m = update(t, m, keyRune('a'))
	if v := ansi.Strip(m.render()); strings.Contains(v, "show dotfiles") {
		t.Errorf("dotfiles are already shown; the hint should drop that key:\n%s", v)
	}
}

func TestEmptyDirectorySaysSo(t *testing.T) {
	m := testModel(t, map[string]string{})
	m = update(t, m, keyRune('a'))
	m = update(t, m, keyRune('.'))
	if v := ansi.Strip(m.render()); !strings.Contains(v, "Nothing to show") {
		t.Errorf("an empty directory with every filter off should say so:\n%s", v)
	}
}

// The start screen names the search keys, so a first-time user finds
// them without opening help (#50).
func TestWelcomeNamesSearchKeys(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	v := ansi.Strip(m.render())
	for _, want := range []string{"search file names", "search file contents"} {
		if !strings.Contains(v, want) {
			t.Errorf("welcome should mention %q:\n%s", want, v)
		}
	}
}
