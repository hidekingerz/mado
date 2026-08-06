package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestLooksBinary(t *testing.T) {
	if !looksBinary([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")) {
		t.Error("png header should look binary")
	}
	if looksBinary([]byte("plain text\nwith lines\n")) {
		t.Error("plain text should not look binary")
	}
	if looksBinary(nil) {
		t.Error("empty data should not look binary")
	}
}

func TestOpenBinaryFileShowsPlaceholder(t *testing.T) {
	png := "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDRfakebinarypayload"
	m := testModel(t, map[string]string{"img.png": png, "a.md": "# A"}, "img.png")
	if len(m.tabs) != 1 {
		t.Fatalf("tabs = %d, want 1", len(m.tabs))
	}
	if !m.tabs[0].binary {
		t.Error("tab should be marked binary")
	}
	view := m.tabs[0].vp.View()
	if !strings.Contains(view, "binary file") {
		t.Errorf("viewport should show the placeholder, got %q", view)
	}
	if strings.Contains(view, "\x00") {
		t.Error("raw NUL bytes must not reach the viewport")
	}

	// Reload must not resurrect the raw bytes.
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if view := m.tabs[0].vp.View(); strings.Contains(view, "\x00") {
		t.Error("reload must keep the placeholder for binary files")
	}
}

func TestHasBinaryExt(t *testing.T) {
	binary := []string{"a.png", "b.JPG", "c.zip", "d.pdf", "e.woff2"}
	for _, p := range binary {
		if !hasBinaryExt(p) {
			t.Errorf("hasBinaryExt(%q) = false, want true", p)
		}
	}
	text := []string{"a.md", "b.go", "c.svg", "d.toml", "Makefile"}
	for _, p := range text {
		if hasBinaryExt(p) {
			t.Errorf("hasBinaryExt(%q) = true, want false", p)
		}
	}
}

func TestSidebarFileStyle(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	if fg, ok := m.styles.file.GetForeground().(lipgloss.Color); !ok || string(fg) != "#FFFFFF" {
		t.Errorf("file style foreground = %v, want #FFFFFF", m.styles.file.GetForeground())
	}
	if m.styles.file.GetBold() {
		t.Error("file style must not be bold")
	}
}
