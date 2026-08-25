package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripAnsi(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func TestNumberLinesBasic(t *testing.T) {
	got := numberLines("one\ntwo\nthree", 20, lipgloss.NewStyle())
	want := "1 one\n2 two\n3 three"
	if got != want {
		t.Errorf("numberLines = %q, want %q", got, want)
	}
}

func TestNumberLinesGutterWidthGrows(t *testing.T) {
	src := strings.Repeat("x\n", 9) + "ten"
	got := numberLines(src, 20, lipgloss.NewStyle())
	if !strings.HasPrefix(got, " 1 x\n") {
		t.Errorf("single digits should be right-aligned in a 2-wide gutter:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n10 ten") {
		t.Errorf("line 10 should fill the gutter:\n%s", got)
	}
}

func TestNumberLinesWrapContinuation(t *testing.T) {
	got := numberLines("aaa bbb", 6, lipgloss.NewStyle())
	want := "1 aaa\n  bbb"
	if got != want {
		t.Errorf("wrapped continuation should get a blank gutter: %q, want %q", got, want)
	}
}

func TestNumberLinesTrailingNewline(t *testing.T) {
	got := numberLines("a\nb\n", 20, lipgloss.NewStyle())
	want := "1 a\n2 b"
	if got != want {
		t.Errorf("trailing newline must not number a phantom line: %q, want %q", got, want)
	}
}

func TestNumberLinesNarrowFallsBackToPlainWrap(t *testing.T) {
	got := numberLines("ab cd", 2, lipgloss.NewStyle())
	if strings.Contains(got, "1 ") {
		t.Errorf("too-narrow width should skip the gutter: %q", got)
	}
}

func TestNumberLinesStyledGutter(t *testing.T) {
	style := lipgloss.NewStyle().SetString("") // plain; rendering path exercised
	got := numberLines("only", 10, style)
	if !strings.Contains(got, "1 only") {
		t.Errorf("styled gutter should still contain the number: %q", got)
	}
}
