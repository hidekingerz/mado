# Settings panel — design

Date: 2026-09-03
Status: approved

## Problem

Every setting in mado lives in `~/.config/mado/config.toml`, and the
only way to change one is to leave mado, edit the file, and restart.
Trying a theme, widening the sidebar, or moving a key means a round
trip through an editor, with no feedback until the next launch, and a
typo is only reported on that launch. mado should let its settings be
changed from inside the window, with the result visible at once and
kept for next time.

## Goal

A settings panel over the content pane, opened with a key, that lists
every configurable value, edits it in place, applies it immediately,
and writes it back to the config file.

- Every key in `config.example.toml` is editable: `[general] watch`,
  the `[theme]` style, mode and colors, `[sidebar]`, `[search]
  exclude`, and every action in `[keys]` — including the new
  `settings` action that opens the panel (default `,`).
- A change takes effect the moment it is made: colors repaint, the
  markdown style re-renders the open tabs, the tree reloads, the
  watcher starts or stops, and a rebound key works from the next
  press.
- Every change is written to the config file at once. Only the changed
  key is rewritten; comments, ordering, and untouched lines in a
  hand-written file survive.
- Values are validated before they are applied, and a value that does
  not pass is reported and left unchanged.

Out of scope: resetting everything to defaults at once, noticing edits
made to the config file by another program while mado runs, reordering
the key list of an action, editing the config file as text.

## Design

### Three layers

- `internal/config` gains a declarative list of the settings (`Fields`)
  and an in-place TOML writer (`Update`). Neither knows about the UI.
- `internal/ui/settings.go` is the panel: `settingsState` on the model,
  the same shape as `searchState`, owning the keyboard and the content
  pane while open.
- `Model.applyConfig` re-derives from a `config.Config` everything
  `New` derives at startup — `styles`, `keyMap`, the resolved glamour
  and chroma styles, `treeOpts`, the watcher — and marks every tab
  dirty so it re-renders when shown.

`main.go` hands the path chosen by `-config` to the model with
`m.WithConfigPath(path)`, alongside `Serve`. An empty path (no home
directory) leaves the panel usable but unsaved, and the status bar
says so.

### Fields

`config.Fields()` returns the settings in the order the panel shows
them. A `Field` carries its table and key, a kind, a one-line
description, the default as a string, and `Get(*Config) string` /
`Set(*Config, string) error`. `Set` validates and returns an error
without touching the config when the value is rejected.

| Kind | Fields | Editing |
| ---- | ------ | ------- |
| bool | `watch`, `show_all_files`, `show_hidden` | `enter` / `space` toggles |
| enum | `style` (auto, dark, light, dracula, notty, ascii, custom…), `default_mode` (reader, source) | `enter` / `→` next, `←` previous, wrapping. `custom…` on `style` opens a text prompt for a style JSON path, and a style that is a path is shown as that path |
| text | `source_style`, the eight colors | `enter` opens an inline prompt holding the current value. Colors accept `#RRGGBB` or an ANSI index 0–255; `source_style` accepts any chroma style name, empty for automatic |
| int | `width` | as text; integers of 16 or more |
| list | `exclude` | as text; patterns separated by spaces. Empty means `exclude = []` |
| keys | every action in `[keys]`, plus `settings` | `enter` captures the next key press and appends it; `backspace` removes the last key. A key already bound to another action is refused with a message naming it; the last key of an action cannot be removed |

Values are shown as the user would write them in TOML, minus the
quotes, so the panel and the file read the same.

### The panel

The panel replaces the body of the content pane, like search. Row one
is the title; then each table as a heading (`[theme]`, in the accent
color) followed by its fields, key on the left and value on the right.
A value that differs from the default is drawn in the accent color so
customizations stand out. The last row is the selected field's
description and default (`default: #7C6AEF`). Rows beyond the pane
scroll, keeping the cursor visible. The status bar shows `[SETTINGS]`
and the config path; its right side shows the operations for the
selected kind (`enter edit │ esc close`, `enter capture │ backspace
remove │ esc close`, …). The help overlay lists the `settings` key.

Keys inside the panel are fixed, as in the search prompt:

- `↑`/`↓`, `k`/`j`, `ctrl+p`/`ctrl+n` move between fields, skipping
  headings; `pgup`/`pgdown` move a page; `home`/`end` go to the ends.
- `enter`, `space`, `←`, `→`, `backspace` edit, as the table says.
- `esc` or the `settings` key closes the panel and returns the focus
  to where it was, with the same fallbacks as `closeSearch`.
- `ctrl+c` quits, as everywhere. `q` does nothing.
- In a text prompt, typed characters edit the value; `enter` commits,
  `esc` cancels, `ctrl+u` clears, `ctrl+w` deletes a word.
- During key capture the next press, whatever it is, becomes the value.
  `ctrl+c` is the one exception and quits.

Mouse: a click selects the row under it, the wheel scrolls.

### Apply and save

Each committed value goes through three steps:

1. `Field.Set` validates. On failure the status bar shows the reason
   and nothing else happens.
2. `applyConfig` puts the new config on screen. Theme fields rebuild
   `styles`, re-resolve the glamour and chroma styles and mark all tabs
   dirty; `default_mode` also switches the current view mode so the
   change is visible; sidebar fields update `treeOpts` and reload the
   tree, and `width` relayouts; `watch` starts or closes the watcher;
   key fields rebuild `keyMap`; `exclude` is read by the next search.
3. `config.Update(path, table, key, value)` writes the file. A failure
   leaves the on-screen value in force and shows `save failed: …`.

`show_all_files` toggled with `a` at runtime is not a config change,
so the panel shows the config value; setting it from the panel reloads
the tree with that value.

### In-place TOML writer

`config.Update(path, table, key string, value Value) error`, in
`internal/config/update.go`, edits one key of a TOML file without
disturbing the rest:

- The file is read as lines. `[table]` headings track the current
  table; within the target table the first line matching `key =` is
  the one to replace. Only the value is rewritten; a trailing
  `# comment` on that line is kept.
- A value that spans lines (a multi-line array) is found by counting
  `[` and `]` outside quoted strings until they balance, and those
  lines are replaced by one.
- A key absent from its table is inserted after the table's last key
  line, before any blank or comment lines that lead into the next
  table. A table absent from the file is appended at the end with a
  blank line before it. A missing file is created, along with its
  parent directory.
- Values are encoded as TOML: basic strings with escapes, `true` /
  `false`, integers, and one-line arrays of strings.
- Before anything is written the result is parsed with
  `toml.Unmarshal` into `Config`. A result that does not parse — a
  dotted key such as `theme.style` elsewhere in the file would now be a
  duplicate — is refused with an error, and the file is untouched.
- The write goes to a temporary file in the same directory, then
  renames over the original, so an interrupted write leaves the old
  file whole.

## Testing

- `internal/config`: every field's `Get`/`Set` round trip; rejected
  colors, widths, modes, style paths and duplicate keys; `Update`
  keeps comments, ordering and untouched lines, replaces a multi-line
  array, adds a missing key at the right place, adds a missing table,
  creates a missing file and directory, refuses an unparsable result
  and leaves the original file intact, and never leaves a temporary
  file behind.
- `internal/ui`: the panel opens with `,` and closes with `esc`,
  restoring focus; movement skips headings and stays in range; each
  kind edits as specified; a theme change re-renders open tabs and
  repaints borders; a key change works on the next press and the old
  key stops working; a duplicate key is refused; `watch` starts and
  stops the watcher; `width` and the sidebar fields take effect; a
  change lands in a temporary config file with the rest of the file
  unchanged; a save failure is shown and the value stays applied; `q`
  types nothing and does not quit while `ctrl+c` does; every row fits
  the pane at narrow widths.
- Manual: tmux — change colors, style, sidebar width and a key, watch
  the window follow, and read the file afterwards.
