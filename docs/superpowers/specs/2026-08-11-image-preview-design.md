# Inline image preview — design

Date: 2026-08-11
Status: approved

## Problem

Opening an image file shows only the binary placeholder. mado ("window")
should be able to show the image itself in the content pane.

## Goal

PNG / JPEG / GIF files render inline as half-block art: the image is
scaled to fit the content pane (aspect ratio preserved) and drawn with
`▀` cells, one character = 1×2 pixels, using the upper-half foreground
and lower-half background color. A one-line caption
(`screenshot.png  800×600`) appears under the image; the whole block is
centered.

- GIF shows its first frame only. No animation, zoom, or pan.
- All decoding uses the Go standard library (`image/png`, `image/jpeg`,
  `image/gif`) — no new dependencies.
- Color output follows the terminal capability pinned at startup:
  TrueColor → 24-bit RGB; 256-color → quantized approximation;
  16-color or less → fall back to the existing binary placeholder.
- Any failure (decode error, tiny pane, low color) falls back to the
  existing placeholder path — the display never breaks.

Out of scope: BMP/WebP/ICO (stay on the placeholder), animation,
zoom/pan, Kitty/iTerm2/Sixel protocols, new config keys.

## Changes

- New `internal/ui/image.go`:
  - `hasImageExt(path string) bool` — png/jpg/jpeg/gif.
  - `decodeImage(data []byte) (image.Image, error)` — `image.Decode`
    with the three stdlib decoders registered.
  - `renderImage(img image.Image, maxW, maxH int, profile termenv.Profile) string`
    — nearest-neighbor scale to (≤maxW chars, ≤maxH×2 pixels), rows of
    `▀` styled per cell; empty string when the pane is unusably small.
- `internal/ui/model.go`:
  - `tab` gains `img image.Image` (nil = not an image tab); the raw
    bytes of an image tab are not kept.
  - Model pins `profile termenv.Profile` at `New` (alongside the
    existing formatter pinning) for image color output.
  - `setContent`: when `hasImageExt` and `decodeImage` succeeds and the
    profile is at least 256-color, mark the tab as an image; otherwise
    fall through to the existing looksBinary/placeholder logic.
  - `ensureRendered`: image tabs render via `renderImage` + caption,
    centered with `lipgloss.Place`; re-renders on pane size change via
    the existing `rendered` width guard (no re-decode).
- `internal/ui/view.go` / `internal/ui/binary.go`: sidebar treats
  displayable image extensions as normal (white) files — png/jpg/jpeg/
  gif leave the dimmed set; bmp/webp/ico and the rest stay dimmed.
- `README.md`: mention inline image preview in Features.

## Testing

- Unit: `hasImageExt`; `decodeImage` round-trip on a generated PNG;
  `renderImage` output fits the given bounds, contains `▀`, and carries
  ANSI color in TrueColor profile; unusable bounds → empty.
- Integration: opening a generated PNG produces a viewport containing
  `▀` and the caption; a corrupt "png" falls back to the placeholder;
  Ascii profile falls back to the placeholder.
- Sidebar: png renders white (not dimmed); zip stays dimmed.
- Manual: tmux — open docs/assets/screenshot.png, verify the picture,
  resize, reload, quit cleanly.
