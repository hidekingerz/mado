# mado

**mado** (窓, "window") is a TUI markdown viewer written in Go, built on
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Glamour](https://github.com/charmbracelet/glamour).

![mado screenshot](docs/assets/screenshot.png)

## Features

- **Markdown rendering** in the terminal via Glamour
- **Reader / source modes** — toggle between a clean document view and syntax-highlighted raw markdown with one key
- **Inline image preview** — PNG/JPEG/GIF files render as half-block pixel art fitted to the pane
- **Sidebar file tree** rooted at the current directory (lazy-loaded, collapsible; `a` toggles between markdown-only and all files). The tree follows the file being viewed: opening a file or switching tabs expands its directories and moves the cursor onto it
- **Multiple open files** — each file opens in its own tab
- **File search** — `/` finds files by name, `ctrl+f` by content, as you type; a content hit opens the file scrolled to its line. Directories such as `node_modules` are skipped, and the list is configurable
- **Mouse support** — click files to open, click tabs to switch, click `✕` to close, scroll wheel in both panes
- **Auto-reload** — `--watch` re-renders open files as they change on disk, so mado can sit in a pane as a live view
- **Remote commands** — `mado --remote open FILE` adds a tab to the instance already on screen instead of starting a second one
- **TOML configuration** — `~/.config/mado/config.toml`
- **Configurable keyboard shortcuts** — every action can be rebound
- **Theme customization** — Glamour styles (`auto`, `dark`, `light`, `dracula`, or your own style JSON) plus UI colors and a `source_style` for source-mode highlighting

## Install

Download a prebuilt binary for Linux, macOS, or Windows from the
[releases page](https://github.com/hidekingerz/mado/releases), or
install with Go:

```sh
go install github.com/hidekingerz/mado@latest
```

Or build from source:

```sh
git clone https://github.com/hidekingerz/mado.git
cd mado
go build -o mado .
```

## Usage

```sh
mado                  # browse markdown files under the current directory
mado docs/            # root the sidebar at docs/
mado README.md a.md   # open files in tabs immediately
mado --watch TASKS.md # reload automatically when the file changes
mado --remote open notes.md   # add a tab to a running mado
mado -style dracula   # override the markdown theme
mado -config my.toml  # use an alternate config file
mado -version         # print the version
```

### View modes

There are two ways to look at a file, switched with `m`:

- **Reader** (default) — markdown rendered as a clean document. Heading
  markers (`##`) are hidden; levels are distinguished by color,
  underline, and italics.
- **Source** — the raw markdown text, for checking the syntax itself.

![reader and source modes](docs/assets/modes.png)

The startup mode can be set with `default_mode = "source"` in the
config.

In source view (and for non-markdown text files, which always render as
source), `n` toggles line numbers, like vi's `:set nu`. Wrapped
continuation rows get a blank gutter, so the numbers track the file's
lines rather than the fold.

### Search

`/` opens the search panel over the content pane and finds files by
name; `ctrl+f` opens it for file contents. Results appear as you type.
`tab` switches between the two targets without losing the query,
`↑`/`↓` move through the results, `enter` opens the selected file and
`esc` puts the panel away. A content hit opens the file scrolled to
the matching line. Typed letters go to the query, so `q` and `?` do
not act while the prompt is open; `ctrl+c` still quits.

Matching is a plain substring — case-insensitive unless the query
contains an upper-case letter, and against the path relative to the
root, so `docs/plan` narrows by directory as well as name. The search
sees what the sidebar could show: markdown files only unless `a` has
switched to all files, and no dotfiles unless `show_hidden` is on.
Binary files and files over 8 MB are skipped for content search, and
the list stops at 1000 hits.

Directories that are never worth searching are left out through
`[search] exclude` in the config — by default `node_modules`, `.git`,
`vendor`, `dist`, `build` and `target`. A pattern without a slash
matches a name at any depth (`node_modules`, `*.log`); one with a
slash matches a path relative to the root (`docs/drafts`). Setting the
key replaces the default list, so `exclude = []` searches everything.

### Auto-reload

By default files are re-read only when you press `r` / `F5`. With
`--watch` (or `watch = true` in the config) mado watches the open files
and the sidebar directories and reloads them as soon as they change,
keeping your scroll position. `[WATCH]` in the status bar shows it is
on.

This is what makes mado usable as a live dashboard: leave it open on a
plan or a log file that another process keeps rewriting, and the pane
follows along. Rapid successive writes are coalesced, so a burst of
saves costs one reload.

What a file says is shown, never obeyed: control characters are drawn
in caret notation (`^[` for escape) rather than passed to the terminal,
so a file cannot set your clipboard, rewrite the window title, or move
the cursor to overwrite what is on screen. That holds for file names in
the sidebar too. It is why an ANSI-coloured log shows its escape codes
as text instead of colours.

### Default key bindings

| Key                | Action                          |
| ------------------ | ------------------------------- |
| `↑`/`k`, `↓`/`j`   | move cursor / scroll            |
| `enter`/`l`        | open file / expand directory    |
| `esc`/`h`          | focus the sidebar               |
| `tab`/`]`          | next tab                        |
| `shift+tab`/`[`    | previous tab                    |
| `x`/`ctrl+w`       | close tab                       |
| `b`/`ctrl+b`       | toggle sidebar                  |
| `ctrl+d`/`ctrl+u`  | half page down / up             |
| `g`/`G`            | go to top / bottom              |
| `r`/`F5`           | reload tree and current file    |
| `m`                | toggle reader / source mode     |
| `a`                | show all files / markdown only  |
| `n`                | toggle line numbers (source)    |
| `/`                | search file names               |
| `ctrl+f`           | search file contents            |
| `?`                | help                            |
| `q`/`ctrl+c`       | quit                            |

### Mouse

- Click a file in the sidebar to open it; click a directory to expand or collapse it.
- Click a tab to switch to it; click the `✕` on a tab to close it.
- Scroll wheel scrolls whichever pane is under the pointer.

### Remote commands

A running mado accepts commands from other processes, so a script, an
editor hook, or a terminal multiplexer plugin can put a file in front
of you without opening a second viewer:

```sh
mado --remote open docs/plan.md    # add a tab (or switch to it if already open)
mado --remote focus docs/plan.md   # switch to the file's tab; fails if it is not open
```

If no instance is running, `--remote open` starts one with those files
— so it is safe to use unconditionally. `--remote focus` never opens a
file in an existing instance; it only moves between what is already
there.

Each instance listens on a Unix socket named after its pid, under
`$XDG_RUNTIME_DIR/mado` (a private directory under the system temp dir
when `XDG_RUNTIME_DIR` is unset). Commands go to the most recently
started instance that answers; set `MADO_SOCKET` to a socket path to
pin both ends to one instance:

```sh
MADO_SOCKET=/tmp/mado-notes.sock mado notes/     # this instance…
MADO_SOCKET=/tmp/mado-notes.sock mado --remote open notes/today.md   # …gets the file
```

The socket directory is a trust boundary: anything a command opens, it
opens as you. mado creates it private and, if it is already there,
checks that it belongs to you and that no other user can write to it —
on a shared machine with no `XDG_RUNTIME_DIR`, the directory name is
predictable enough for someone else to create first and plant a socket
in. A directory that does not pass turns remote commands off for that
instance, with a line on stderr saying why; everything else about mado
still works.

## Configuration

mado reads `$XDG_CONFIG_HOME/mado/config.toml` (default
`~/.config/mado/config.toml`). Every key is optional; unset values fall
back to the defaults. See [`config.example.toml`](config.example.toml)
for the full annotated reference.

```toml
[general]
watch = false              # true = auto-reload files when they change

[theme]
style = "dracula"          # glamour style name or path to a style JSON
accent_color = "#7C6AEF"
dir_color = "#89B4FA"      # sidebar directory names
file_color = "#FFFFFF"     # sidebar text files (known binaries are dimmed)

[sidebar]
width = 32
show_all_files = false     # true = list non-markdown files too
show_hidden = false

[search]
exclude = ["node_modules", ".git", "vendor", "dist", "build", "target"]

[keys]
quit = ["q", "ctrl+c"]
open = ["enter", "l"]
next_tab = ["tab", "]"]
# … any action from config.example.toml
```

## License

[Apache-2.0](LICENSE)
