package ui

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// stubClipboard replaces the clipboard writer for one test and
// returns what it was asked to copy.
func stubClipboard(t *testing.T) *[]string {
	t.Helper()
	var copied []string
	orig := copyToClipboard
	copyToClipboard = func(s string) tea.Cmd {
		copied = append(copied, s)
		return func() tea.Msg { return nil }
	}
	t.Cleanup(func() { copyToClipboard = orig })
	return &copied
}

func TestCopyPathCopiesTheActiveTabsAbsolutePath(t *testing.T) {
	copied := stubClipboard(t)
	m := testModel(t, map[string]string{"docs/a.md": "# A", "b.md": "# B"}, "docs/a.md", "b.md")
	m.active = 0

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	m = next.(Model)

	want := m.tabs[0].path
	if !filepath.IsAbs(want) || !strings.HasSuffix(want, filepath.Join("docs", "a.md")) {
		t.Fatalf("tab path = %q, want an absolute path to docs/a.md", want)
	}
	if !slices.Equal(*copied, []string{want}) {
		t.Errorf("copied = %q, want [%q]", *copied, want)
	}
	if cmd == nil {
		t.Error("copy path returned no command; nothing would reach the clipboard")
	}
	if m.notice != m.text.CopiedPath || m.statusMsg != "" {
		t.Errorf("notice = %q, statusMsg = %q; want the copy confirmed as a notice, not a warning", m.notice, m.statusMsg)
	}
	if bar := m.renderStatusBar(); !strings.Contains(bar, m.text.CopiedPath) || strings.Contains(bar, "⚠") {
		t.Errorf("status bar = %q, want %q without a warning sign", bar, m.text.CopiedPath)
	}

	m = update(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.notice != "" {
		t.Errorf("notice = %q after the next key, want it gone", m.notice)
	}
}

func TestCopyPathWithNoTabSaysSo(t *testing.T) {
	copied := stubClipboard(t)
	m := testModel(t, map[string]string{"a.md": "# A"})

	m = update(t, m, tea.KeyPressMsg{Code: 'y', Text: "y"})

	if len(*copied) != 0 {
		t.Errorf("copied = %q with no tab open, want nothing", *copied)
	}
	if m.statusMsg != m.text.NoFileToCopy {
		t.Errorf("statusMsg = %q, want %q", m.statusMsg, m.text.NoFileToCopy)
	}
}

func TestClipboardCommand(t *testing.T) {
	has := func(names ...string) func(string) (string, error) {
		return func(n string) (string, error) {
			if slices.Contains(names, n) {
				return "/usr/bin/" + n, nil
			}
			return "", errors.New("not found")
		}
	}
	tests := []struct {
		name string
		goos string
		env  map[string]string
		look func(string) (string, error)
		want []string
	}{
		{"macOS", "darwin", nil, has("pbcopy"), []string{"pbcopy"}},
		{"over ssh the local clipboard is the wrong one", "darwin", map[string]string{"SSH_CONNECTION": "1 2 3 4"}, has("pbcopy"), nil},
		{"wayland", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, has("wl-copy", "xclip"), []string{"wl-copy"}},
		{"x11 xclip", "linux", map[string]string{"DISPLAY": ":0"}, has("xclip", "xsel"), []string{"xclip", "-selection", "clipboard"}},
		{"x11 xsel", "linux", map[string]string{"DISPLAY": ":0"}, has("xsel"), []string{"xsel", "--clipboard", "--input"}},
		{"linux console", "linux", nil, has("xclip"), nil},
		{"wsl", "linux", nil, has("clip.exe"), []string{"clip.exe"}},
		{"windows", "windows", nil, has("clip"), []string{"clip"}},
		{"nothing installed", "darwin", nil, has(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(k string) string { return tt.env[k] }
			if got := clipboardCommand(tt.goos, getenv, tt.look); !slices.Equal(got, tt.want) {
				t.Errorf("clipboardCommand = %q, want %q", got, tt.want)
			}
		})
	}
}
