# Source-mode syntax highlighting — design

Date: 2026-08-06
Status: approved

## Problem

Source mode (and any non-markdown file) renders the raw text with no
styling, so it appears in the terminal's default foreground — typically
a flat gray. Compared to the Glamour-rendered reader mode it is hard to
read. Non-markdown files opened in mado have the same problem.

## Goal

Syntax-highlight raw text shown in the content pane:

- Source mode for markdown files (markdown lexer: headings, emphasis,
  links, code fences get colors).
- Non-markdown files, with the language detected from the filename.

Out of scope: reader-mode changes, line numbers, search.

## Approach

Use chroma directly (`github.com/alecthomas/chroma/v2` — already in the
dependency graph via glamour, so no new downloads; it merely becomes a
direct dependency).

### New: `internal/ui/highlight.go`

```
func highlightSource(src, path, chromaStyle string) (string, error)
```

- Lexer: `lexers.Match(filepath.Base(path))`; fall back to
  `lexers.Fallback` (equivalent to plain text) when unknown.
- Formatter: chosen from the terminal color profile via termenv —
  TrueColor → `terminal16m`, 256 colors → `terminal256`, otherwise
  no color (plain passthrough).
- Style: `styles.Get(chromaStyle)`; unknown names fall back to a sane
  default rather than erroring.
- Any error → caller keeps the plain text (display never breaks).

### Change: `ensureRendered` (`internal/ui/model.go`)

Where it currently does `wordwrap.String(t.raw, w-2)` for source mode /
non-markdown files, first run `highlightSource`, then wordwrap. The
reflow wordwrap is ANSI-aware, so highlighted text can be wrapped as-is.

### Theme mapping

Derive the chroma style from the resolved glamour style:

| glamour style   | chroma style       |
| --------------- | ------------------ |
| dark (default)  | catppuccin-mocha   |
| light           | catppuccin-latte   |
| dracula         | dracula            |
| other / JSON    | catppuccin-mocha or catppuccin-latte, per `termenv.HasDarkBackground()` |

New optional config key `theme.source_style` (TOML) overrides the
mapping with an explicit chroma style name (e.g. `monokai`, `github`),
consistent with mado's "everything is configurable" policy. Document it
in `config.example.toml`.

## Testing

- Unit tests for `highlightSource`: `.md` and `.go` inputs produce ANSI
  escapes; unknown extension falls back to plain; invalid style name
  falls back without error.
- Existing `ensureRendered`/model tests keep passing.
- Manual: run in tmux, toggle source mode, open a `.go` file, verify
  colors; check `-style light` and `source_style` override.
