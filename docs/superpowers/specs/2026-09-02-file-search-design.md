# File search — design

Date: 2026-09-02
Status: approved

## Problem

Finding a file in mado means walking the sidebar tree, and finding
where something is said means opening files one by one. On a tree of
any size — a repository, a notes directory — neither is practical.
mado should find files by name and by content, from the keyboard,
without leaving the window.

## Goal

A search panel over the content pane, opened with a key, that lists
matches as the query is typed and opens the selected one.

- Two targets: **names** (default key `/`) matches the query against
  paths relative to the root; **contents** (default key `ctrl+f`)
  matches it against the lines of text files. `tab` switches target,
  keeping the query.
- Matching is a case-insensitive substring, exact when the query has an
  upper-case letter (smart case, as in vim and ripgrep).
- The search sees what the sidebar could show: the same markdown-only /
  all-files and hidden-files filter, so `a` widens both.
- `[search] exclude` in the config lists glob patterns for files and
  directories the search skips — `node_modules`, `.git`, `vendor`,
  `dist`, `build`, `target` by default. A pattern without a slash
  matches a name at any depth, one with a slash a root-relative path.
  Setting the key replaces the defaults; `exclude = []` searches
  everything.
- `enter` opens the selected file in a tab and closes the panel. A
  content hit also scrolls the tab to its line.
- Results are bounded: content search skips binary files and files over
  8 MB, and stops at 1000 hits, saying so.
- Paths and matched lines are displayed through `termsafe`, like every
  other string from disk.

Out of scope: fuzzy or regular-expression matching, search within the
open document, highlighting hits in the rendered document, a
persistent index.

## Changes

- New `internal/search`:
  - `Run(ctx, root, target, query, opts) Result` walks the tree with
    `filepath.WalkDir`, applying the sidebar filter and the exclusions,
    and stops early on cancellation. Name matches are ranked: a hit in
    the file name before one only in a directory, then shallower
    paths, then alphabetically. Content matches come in walk order,
    each with its 1-based line and text.
  - `Find(s, query) (start, end)` is the smart-case matcher, returning
    the byte range of the hit so the UI can emphasize it. Case folding
    is rune by rune so the range maps onto the original bytes.
  - `Excluded(rel, patterns) bool` implements the pattern rules.
- `internal/config`: `Search.Exclude`, `Keys.Search`,
  `Keys.SearchContent`; `nil` (key absent) keeps the default exclusions,
  an explicit empty list clears them.
- `internal/ui/search.go`: the panel. Its state is `searchState` on the
  model. Each edit to the query cancels the running search and starts
  another as a `tea.Cmd`; results carry a generation number and stale
  ones are dropped. While the panel is open, typed characters go to
  the query, so only keys that cannot be typed act on it — including
  the quit binding, where `ctrl+c` still quits but `q` is typed.
- Jump to a content hit: source-rendered text maps a file line to a
  viewport row exactly by wrapping the preceding lines the way
  `ensureRendered` does (`sourceRow`). A rendered markdown document is
  rewrapped and loses syntax, so the hit is found by counting
  occurrences of the query in the rendered text (`readerRow`), falling
  back to a proportional estimate. The tab keeps its rendered content
  for this.
- `internal/ui/view.go`: the panel replaces the content pane's body
  while open; the status bar shows the target and the count; the help
  overlay lists the two keys and the prompt keys.
- `README.md`, `config.example.toml`: document the keys and
  `[search] exclude`.

## Testing

- `internal/search`: relative-path and smart-case matching, ranking,
  the sidebar filter, exclusion patterns (name, path, glob, trailing
  slash, malformed), binary and size skipping, the result cap, early
  cancellation, byte ranges over multi-byte folds.
- `internal/config`: default keys, rebinding, exclude replace / clear /
  absent.
- `internal/ui`: the panel opens and filters as typed; `enter` opens the
  file and the sidebar follows; arrows and mouse pick a result; typed
  `q` and `?` are query text while `ctrl+c` quits; `tab` and `ctrl+f`
  switch targets; a content hit scrolls a source tab to the exact row
  and a reader tab to a row showing the text; stale results are
  dropped; exclusions and the sidebar filter apply; names and lines
  containing escapes cannot drive the terminal; every row fits the
  pane.
- Manual: tmux — `/` and `ctrl+f` on this repository, resize, open a
  hit in both modes, quit cleanly.
