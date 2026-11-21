package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/hidekingerz/mado/internal/config"
)

func TestHighlightSourceMarkdown(t *testing.T) {
	out, err := highlightSource("# Title\n\n**bold** text\n", "README.md", "catppuccin-mocha", "terminal16m")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("markdown source should contain ANSI escapes")
	}
	if !strings.Contains(out, "Title") {
		t.Error("highlighted output should preserve the text")
	}
}

func TestHighlightSourceGo(t *testing.T) {
	out, err := highlightSource("package main\n\nfunc main() {}\n", "main.go", "catppuccin-mocha", "terminal16m")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("go source should contain ANSI escapes")
	}
}

func TestHighlightSourceUnknownExtensionFallsBack(t *testing.T) {
	src := "no idea what language this is\n"
	out, err := highlightSource(src, "data.zzz9", "catppuccin-mocha", "terminal16m")
	if err != nil {
		t.Fatal(err)
	}
	if out != src {
		t.Errorf("unknown extension should return input unchanged, got %q", out)
	}
}

func TestHighlightSourceEmptyStyleOrFormatterSkips(t *testing.T) {
	src := "# hi\n"
	for _, tc := range [][2]string{{"", "terminal16m"}, {"catppuccin-mocha", ""}} {
		out, err := highlightSource(src, "a.md", tc[0], tc[1])
		if err != nil {
			t.Fatal(err)
		}
		if out != src {
			t.Errorf("style=%q formatter=%q: want input unchanged, got %q", tc[0], tc[1], out)
		}
	}
}

func TestHighlightSourceInvalidStyleStillRenders(t *testing.T) {
	out, err := highlightSource("# hi\n", "a.md", "no-such-style", "terminal16m")
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Error("invalid style name should fall back, not produce empty output")
	}
}

func TestChromaStyleName(t *testing.T) {
	cases := []struct {
		glamour, override string
		darkBG            bool
		want              string
	}{
		{"dark", "", true, "catppuccin-mocha"},
		{"light", "", false, "catppuccin-latte"},
		{"dracula", "", true, "dracula"},
		{"notty", "", true, ""},
		{"ascii", "", false, ""},
		{"/tmp/custom.json", "", true, "catppuccin-mocha"},
		{"/tmp/custom.json", "", false, "catppuccin-latte"},
		{"dark", "monokai", true, "monokai"},
	}
	for _, c := range cases {
		if got := chromaStyleName(c.glamour, c.override, c.darkBG); got != c.want {
			t.Errorf("chromaStyleName(%q, %q, %v) = %q, want %q", c.glamour, c.override, c.darkBG, got, c.want)
		}
	}
}

// Bubble Tea brings colors down to what the terminal shows when it
// draws, so chroma always writes full color unless there is no color
// at all (#35).
func TestChromaFormatterFollowsProfile(t *testing.T) {
	cases := map[colorprofile.Profile]string{
		colorprofile.TrueColor: "terminal16m",
		colorprofile.ANSI256:   "terminal16m",
		colorprofile.ANSI:      "terminal16m",
		colorprofile.Ascii:     "",
		colorprofile.NoTTY:     "",
	}
	for p, want := range cases {
		if got := chromaFormatterFor(p); got != want {
			t.Errorf("chromaFormatterFor(%v) = %q, want %q", p, got, want)
		}
	}
}

// The terminal is asked for its background only when a setting depends
// on it (#35).
func TestNeedsBackground(t *testing.T) {
	cases := []struct {
		style, source string
		want          bool
	}{
		{"auto", "", true},
		{"", "", true},
		{"auto", "monokai", true},
		{"dark", "", false},
		{"light", "", false},
		{"dracula", "", false},
		{"notty", "", false},
		{"/path/to/custom.json", "", true},
		{"/path/to/custom.json", "monokai", false},
	}
	for _, c := range cases {
		if got := needsBackground(config.Theme{Style: c.style, SourceStyle: c.source}); got != c.want {
			t.Errorf("needsBackground(%q, %q) = %v, want %v", c.style, c.source, got, c.want)
		}
	}
}
