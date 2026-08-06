# Sidebar file colors — design

Date: 2026-08-07
Status: approved

## Problem

With directories now blue, all files still share the terminal default
foreground. Displayable (text) files and non-displayable binaries look
identical in the sidebar.

## Goal

- Displayable files render in a dedicated color: new
  `theme.file_color`, default `#FFFFFF` (white), not bold.
- Files with a well-known binary extension render dimmed (the existing
  `styles.dimmed`, ANSI 245), signaling that opening them shows only a
  placeholder.
- Directory rows and cursor-row priority are unchanged:
  selection > cursor accent > dir > binary-dimmed > file-white.

Extension-based detection is used because the sidebar cannot afford to
read file contents. It is display-only: opening a file still uses the
content-based `looksBinary` NUL check for the placeholder. SVG counts
as displayable (it is XML text).

Extensions treated as binary: png jpg jpeg gif bmp webp ico · mp3 wav
flac mp4 mov avi · zip tar gz tgz bz2 xz 7z rar · pdf · exe dll so
dylib bin o a · ttf otf woff woff2.

## Changes

- `internal/ui/binary.go` — `hasBinaryExt(path string) bool` with the
  extension set above.
- `internal/config/config.go` — `Theme.FileColor string`
  (TOML `file_color`, default `"#FFFFFF"`), merged via `mergeStr`.
- `internal/ui/model.go` — `styles.file` (Foreground FileColor).
- `internal/ui/view.go` — `renderSidebar` switch gains
  `case hasBinaryExt(...)` (dimmed) and a `default` (file style) after
  the existing dir case.
- `config.example.toml` / `README.md` — document `file_color`.

## Testing

- `hasBinaryExt`: png/zip true; md/go/svg false; case-insensitive.
- Config: `file_color` default and TOML override.
- UI: `styles.file` foreground is `#FFFFFF` by default.
- Manual: tmux, all-files mode — text files white, png dimmed.
