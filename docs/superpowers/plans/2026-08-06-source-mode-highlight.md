# Source-Mode Syntax Highlighting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Syntax-highlight the raw text shown in source mode (and for non-markdown files) using chroma, replacing the current unstyled gray output.

**Architecture:** A new `internal/ui/highlight.go` wraps chroma (lexer matched from the filename, style derived from the glamour theme, formatter pinned once at startup from the terminal color profile). `ensureRendered` in `internal/ui/model.go` runs highlight before wordwrap for source-mode / non-markdown content. A new `theme.source_style` config key overrides the automatic style mapping. Every failure path falls back to today's plain text — display never breaks.

**Tech Stack:** Go, chroma v2 (already an indirect dependency via glamour — becomes direct, no new downloads), termenv, muesli/reflow (ANSI-aware wordwrap, already used).

**Spec:** `docs/superpowers/specs/2026-08-06-source-mode-highlight-design.md`

## Global Constraints

- Highlight failures must NEVER break display: on any error or unknown input, show the plain text exactly as today.
- The running program must not query the terminal — pin terminal-dependent decisions once at `New` (same pattern as `resolveStyle`, `internal/ui/model.go:462-472`).
- Style mapping: `dark` → `catppuccin-mocha`, `light` → `catppuccin-latte`, `dracula` → `dracula`, `notty`/`ascii` → no highlighting, anything else (incl. JSON paths) → mocha/latte per `termenv.HasDarkBackground()`.
- Tests must not depend on the terminal: pass explicit style/formatter names; the model tests set fields directly (same package).
- Run `gofmt -l .` and `go vet ./...` before each commit (CI enforces both).

---

### Task 1: `highlightSource` helper

**Files:**
- Create: `internal/ui/highlight.go`
- Create: `internal/ui/highlight_test.go`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces (used by Task 3):
  - `func highlightSource(src, path, styleName, formatterName string) (string, error)` — returns highlighted `src`; returns `src` unchanged when styleName/formatterName is empty, lexer unknown, or on error.
  - `func chromaStyleName(glamourStyle, override string, darkBG bool) string` — maps a resolved glamour style to a chroma style name; `""` means "do not highlight".
  - `func chromaFormatterName() string` — picks a chroma formatter from `termenv.ColorProfile()`; `""` means "do not highlight". (Queries the terminal — call only at startup.)

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/highlight_test.go`:

```go
package ui

import (
	"strings"
	"testing"
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run 'TestHighlight|TestChromaStyleName' -v`
Expected: FAIL to build with "undefined: highlightSource" etc.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/highlight.go`:

```go
package ui

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/muesli/termenv"
)

// chromaStyleName maps the resolved glamour style to a chroma style.
// override (theme.source_style) wins when set. "" disables highlighting.
func chromaStyleName(glamourStyle, override string, darkBG bool) string {
	if override != "" {
		return override
	}
	switch glamourStyle {
	case "dark":
		return "catppuccin-mocha"
	case "light":
		return "catppuccin-latte"
	case "dracula":
		return "dracula"
	case "notty", "ascii":
		return ""
	default: // custom style JSON or anything unrecognized
		if darkBG {
			return "catppuccin-mocha"
		}
		return "catppuccin-latte"
	}
}

// chromaFormatterName picks a chroma formatter for the terminal's color
// capability. Queries the terminal — call once at startup only.
// "" disables highlighting.
func chromaFormatterName() string {
	switch termenv.ColorProfile() {
	case termenv.TrueColor:
		return "terminal16m"
	case termenv.ANSI256:
		return "terminal256"
	case termenv.ANSI:
		return "terminal16"
	default:
		return ""
	}
}

// highlightSource syntax-highlights src, detecting the language from
// path's filename. On any failure or unknown input it returns src
// unchanged so the display never breaks.
func highlightSource(src, path, styleName, formatterName string) (string, error) {
	if styleName == "" || formatterName == "" {
		return src, nil
	}
	lexer := lexers.Match(filepath.Base(path))
	if lexer == nil {
		return src, nil
	}
	lexer = chroma.Coalesce(lexer)
	formatter := formatters.Get(formatterName)
	if formatter == nil {
		return src, nil
	}
	it, err := lexer.Tokenise(nil, src)
	if err != nil {
		return src, err
	}
	var b strings.Builder
	if err := formatter.Format(&b, chromastyles.Get(styleName), it); err != nil {
		return src, err
	}
	return b.String(), nil
}
```

Note: the chroma styles package MUST be aliased (`chromastyles`) — package `ui` already declares a `styles` struct type (`internal/ui/model.go:99`).

- [ ] **Step 4: Tidy modules and run the tests**

Run: `go mod tidy && go test ./internal/ui/ -run 'TestHighlight|TestChromaStyleName' -v`
Expected: PASS. `go.mod` now lists `github.com/alecthomas/chroma/v2` as a direct dependency (it was indirect; version stays v2.20.0, nothing new is downloaded).

- [ ] **Step 5: Verify formatting and vet, then commit**

Run: `gofmt -l . && go vet ./...`
Expected: no output from gofmt, no vet errors.

```bash
git add internal/ui/highlight.go internal/ui/highlight_test.go go.mod go.sum
git commit -m "Add chroma-based highlightSource helper"
```

---

### Task 2: `theme.source_style` config key

**Files:**
- Modify: `internal/config/config.go` (Theme struct ~line 19-32, merge ~line 139)
- Modify: `config.example.toml` (theme table, ~line 8-21)
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces (used by Task 3): `Config.Theme.SourceStyle string` (TOML key `source_style`, default `""` = automatic mapping).

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go` (follow the file's existing test style — check it before adding):

```go
func TestSourceStyleKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[theme]\nsource_style = \"monokai\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.SourceStyle != "monokai" {
		t.Errorf("SourceStyle = %q, want monokai", cfg.Theme.SourceStyle)
	}
	if Default().Theme.SourceStyle != "" {
		t.Errorf("default SourceStyle should be empty (= automatic)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestSourceStyleKey -v`
Expected: FAIL to build with "unknown field SourceStyle".

- [ ] **Step 3: Implement the config key**

In `internal/config/config.go`, add to the `Theme` struct after `DefaultMode` (~line 25):

```go
	// SourceStyle is the chroma style used to highlight source-mode
	// text and non-markdown files (e.g. "monokai", "github").
	// Empty = pick automatically from Style.
	SourceStyle string `toml:"source_style"`
```

Add to `merge` next to the other Theme lines (~line 140):

```go
	mergeStr(&dst.Theme.SourceStyle, src.Theme.SourceStyle)
```

(`Default()` needs no change — the zero value `""` means automatic.)

In `config.example.toml`, after the `default_mode` line (~line 14), add:

```toml
# Chroma style for source mode and non-markdown files ("monokai",
# "github", "dracula", …). Unset = chosen automatically from `style`.
# source_style = "monokai"
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/config/ -v`
Expected: PASS (new test and all existing ones).

- [ ] **Step 5: Verify formatting and vet, then commit**

Run: `gofmt -l . && go vet ./...`
Expected: no output from gofmt, no vet errors.

```bash
git add internal/config/config.go internal/config/config_test.go config.example.toml
git commit -m "Add theme.source_style config key"
```

---

### Task 3: Wire highlighting into the model

**Files:**
- Modify: `internal/ui/model.go` (Model struct ~line 88, `New` ~line 125-144, `ensureRendered` ~line 438-440)
- Modify: `README.md` (Features list; Theme customization bullet)
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `highlightSource`, `chromaStyleName`, `chromaFormatterName` (Task 1); `cfg.Theme.SourceStyle` (Task 2).
- Produces: new unexported Model fields `sourceStyle string`, `formatter string`.

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/model_test.go`:

```go
func TestSourceModeHighlights(t *testing.T) {
	m := testModel(t, map[string]string{"doc.md": "# Title\n\nsome text\n"}, "doc.md")
	// testModel uses style "notty", which disables highlighting; force it
	// on directly (same package) to stay independent of the test terminal.
	m.sourceStyle = "catppuccin-mocha"
	m.formatter = "terminal16m"

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}) // reader → source
	if m.mode != modeSource {
		t.Fatalf("mode = %v, want source", m.mode)
	}
	if view := m.tabs[0].vp.View(); !strings.Contains(view, "\x1b[") {
		t.Error("source mode content should contain ANSI escapes")
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}) // back to reader: must not panic
	if m.mode != modeReader {
		t.Fatalf("mode = %v, want reader", m.mode)
	}
}

func TestSourceModeNottyStaysPlain(t *testing.T) {
	m := testModel(t, map[string]string{"doc.md": "# Title\n"}, "doc.md")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if view := m.tabs[0].vp.View(); strings.Contains(view, "\x1b[38") {
		t.Error("notty style must not colorize source mode")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run TestSourceMode -v`
Expected: FAIL to build with "m.sourceStyle undefined".

- [ ] **Step 3: Implement the wiring**

In `internal/ui/model.go`:

1. Model struct — after `style string` (~line 88) add:

```go
	sourceStyle string // chroma style for source mode; "" = no highlighting
	formatter   string // chroma formatter; pinned at startup, "" = no color
```

2. In `New` (~line 133), replace the single `style:` line's surroundings so both values are computed once (keep the existing behavior of pinning terminal queries at startup):

```go
	style := resolveStyle(cfg.Theme.Style)
```

before constructing `m`, and inside the literal use:

```go
		style:       style,
		sourceStyle: chromaStyleName(style, cfg.Theme.SourceStyle, termenv.HasDarkBackground()),
		formatter:   chromaFormatterName(),
```

3. In `ensureRendered` (~line 438-440), change:

```go
	content := t.raw
	if m.mode == modeSource || !filetree.IsMarkdown(t.path) {
		content = wordwrap.String(t.raw, w-2)
	} else {
```

to:

```go
	content := t.raw
	if m.mode == modeSource || !filetree.IsMarkdown(t.path) {
		if hl, err := highlightSource(t.raw, t.path, m.sourceStyle, m.formatter); err == nil {
			content = hl
		}
		content = wordwrap.String(content, w-2)
	} else {
```

(On highlight error the plain `t.raw` is wrapped — exactly today's behavior.)

4. `README.md`: in the Features list change the reader/source bullet to mention highlighting, e.g. "**Reader / source modes** — toggle between a clean document view and syntax-highlighted raw markdown with one key", and add `source_style` to the theme-customization bullet ("… plus UI colors and a `source_style` for source-mode highlighting").

- [ ] **Step 4: Run the full test suite**

Run: `go test -race ./...`
Expected: all packages PASS (CI runs with `-race`).

- [ ] **Step 5: Verify formatting and vet, then commit**

Run: `gofmt -l . && go vet ./...`
Expected: no output from gofmt, no vet errors.

```bash
git add internal/ui/model.go internal/ui/model_test.go README.md
git commit -m "Highlight source mode and non-markdown files with chroma"
```

- [ ] **Step 6: Manual verification in tmux**

```bash
go build -o /tmp/mado .
tmux new-session -d -s mado -x 140 -y 40 '/tmp/mado README.md'
sleep 1
tmux send-keys -t mado 'm'   # source mode
sleep 0.5
tmux capture-pane -t mado -p -e | head -20   # expect colored heading/bold/link markup
tmux kill-session -t mado
```

Also open a `.go` file (e.g. `/tmp/mado main.go` — non-markdown files need no mode toggle) and confirm Go code is colorized. Verify `-style light` still works and text remains readable.
