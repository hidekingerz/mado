# Sidebar directory visibility — design

Date: 2026-08-07
Status: approved

## Problem

In the sidebar, directories and files are hard to tell apart. Both are
drawn in the default foreground; directories differ only by the ▸/▾
icon and bold weight. With all-files mode (v0.2.x) showing many
non-markdown files, the distinction matters more.

## Goal

Directories are visually distinct in two redundant ways:

1. **Trailing slash** — directory names render as `docs/` (works
   without color perception).
2. **Dedicated color** — directory rows use a configurable color plus
   bold. Default `#89B4FA` (blue), deliberately distinct from the
   accent purple `#7C6AEF` so a cursor-highlighted row is not confused
   with a directory row.

Display priority is unchanged from today: focused-cursor selection
style > cursor accent style > directory style > plain file text.
The cursor row keeps its current look.

Out of scope: Nerd Font icons, per-file-type colors, tree guide lines.

## Changes

- `internal/ui/view.go` (`renderSidebar`) — append `/` to directory
  names before truncation so long names truncate correctly; use the
  new `m.styles.dir` style in the `case it.Node.IsDir:` branch
  (replacing the inline `lipgloss.NewStyle().Bold(true)`).
- `internal/ui/model.go` — add `dir lipgloss.Style` to the `styles`
  struct; build it in `New` as Foreground(`theme.dir_color`) + Bold.
- `internal/config/config.go` — add `Theme.DirColor string`
  (TOML `dir_color`, default `"#89B4FA"`), merged with `mergeStr` like
  the other color keys.
- `config.example.toml` / `README.md` — document `dir_color`.

## Error handling

None new — color values follow the same path as existing theme colors
(lipgloss accepts any string; invalid values degrade to no color).

## Testing

- View test: `View()` output contains `docs/` for a directory and no
  trailing slash on file names.
- Config test: `dir_color` default is `#89B4FA`; a TOML override is
  parsed and merged.
- Mouse hit-testing is row-based and unaffected (no test change
  needed).
- Manual: run in tmux, confirm directories are blue with `/`, cursor
  row unchanged.
