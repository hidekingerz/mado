# Sidebar all-files toggle key — design

Date: 2026-08-06
Status: approved

## Problem

The sidebar lists only markdown files by default. Showing other text
files requires setting `sidebar.show_all_files = true` in the config —
there is no way to switch at runtime. Since v0.2.0 non-markdown files
are syntax-highlighted when opened, so browsing them is genuinely
useful.

## Goal

A key (default `a`) toggles the sidebar between "markdown only" and
"all files" while mado is running.

- Expanded directories keep their expansion state across the toggle
  (the existing `filetree.Reload` already guarantees this).
- The startup default stays "markdown only"; `show_all_files = true`
  still works and simply sets the toggle's initial state.
- The key is rebindable like every other action.

Out of scope: binary-file filtering (all files means all files, same
as today's `show_all_files = true`), a hidden-files toggle, CLI flags.

## Changes

- `internal/config/config.go` — add `Keys.ToggleAllFiles []string`
  (TOML `toggle_all_files`, default `["a"]`) plus its merge line.
- `internal/ui/keys.go` — new binding, help text
  "all files / markdown only".
- `internal/ui/model.go` — extract the tree-refresh half of `reload()`
  into a helper `reloadTree()` (Reload + Flatten + clampTree +
  statusMsg on error); the new key handler flips
  `m.treeOpts.ShowAllFiles` and calls it. `reload()` keeps its
  tab-content refresh and now calls the helper for the tree part.
- `internal/ui/view.go` — help screen gains the new row.
- `config.example.toml` / `README.md` — document the key.

## Error handling

Directory read failures surface via `statusMsg`, exactly as the
existing `reload()` path does; the UI never breaks.

## Testing

- Model test: pressing `a` makes a non-markdown file (e.g. `main.go`)
  appear in the sidebar items; pressing again hides it; an expanded
  directory stays expanded across the toggle.
- Config test: `toggle_all_files` override is parsed and merged;
  default is `["a"]`.
- Manual: run in tmux, toggle, open a `.go` file, confirm it renders
  highlighted.
