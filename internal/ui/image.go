package ui

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // register stdlib decoders for image.Decode
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"

	"github.com/muesli/termenv"
)

// maxImagePixels caps the declared width*height of an image before mado
// attempts a full decode. Without this cap, a small crafted file (e.g. a
// 13-byte hand-built GIF header) can declare arbitrarily large dimensions
// and cause image.Decode to allocate the full pixel buffer — at
// 50,000,000 pixels and 4 bytes/pixel for RGBA, that's already ~200 MB
// from a single sidebar click, enough to freeze or crash the TUI.
const maxImagePixels = 50_000_000

// errImageTooLarge is wrapped into the error decodeImage returns when an
// image's declared dimensions exceed maxImagePixels.
var errImageTooLarge = errors.New("image dimensions exceed maximum allowed pixel count")

// imageExts are the formats the stdlib can decode. BMP/WebP/ICO are
// deliberately absent — they fall back to the binary placeholder.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
}

// hasImageExt reports whether path has a displayable image extension.
func hasImageExt(path string) bool {
	return imageExts[strings.ToLower(filepath.Ext(path))]
}

// decodeImage decodes PNG/JPEG/GIF data (GIF: first frame). It first reads
// only the declared dimensions via image.DecodeConfig and rejects images
// whose pixel count would exceed maxImagePixels, so a hostile file can't
// force a huge allocation before mado gets a chance to say no.
func decodeImage(data []byte) (image.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("decodeImage: non-positive dimensions %dx%d", cfg.Width, cfg.Height)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return nil, fmt.Errorf("decodeImage: %dx%d exceeds %d-pixel cap: %w", cfg.Width, cfg.Height, maxImagePixels, errImageTooLarge)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

// renderImage draws img as half-block art: one character cell is 1×2
// pixels, the upper pixel as foreground and the lower as background of
// "▀". The image is scaled with nearest-neighbor sampling to fit
// maxW columns × maxH rows, preserving aspect ratio. Colors are
// rendered for the given profile, so output is deterministic in tests.
// Returns "" when the pane cannot hold an image.
func renderImage(img image.Image, maxW, maxH int, profile termenv.Profile) string {
	if maxW < 2 || maxH < 1 {
		return ""
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return ""
	}
	scale := 1.0
	if s := float64(maxW) / float64(w); s < scale {
		scale = s
	}
	if s := float64(maxH*2) / float64(h); s < scale {
		scale = s
	}
	tw, th := int(float64(w)*scale), int(float64(h)*scale)
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}

	sample := func(x, y int) termenv.Color {
		sx := b.Min.X + x*w/tw
		sy := b.Min.Y + y*h/th
		return profile.FromColor(img.At(sx, sy))
	}

	var sb strings.Builder
	for y := 0; y < th; y += 2 {
		if y > 0 {
			sb.WriteString("\n")
		}
		for x := 0; x < tw; x++ {
			cell := profile.String("▀").Foreground(sample(x, y))
			if y+1 < th {
				cell = cell.Background(sample(x, y+1))
			}
			sb.WriteString(cell.String())
		}
	}
	return sb.String()
}
