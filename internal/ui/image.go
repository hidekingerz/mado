package ui

import (
	"bytes"
	"image"
	_ "image/gif" // register stdlib decoders for image.Decode
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"

	"github.com/muesli/termenv"
)

// imageExts are the formats the stdlib can decode. BMP/WebP/ICO are
// deliberately absent — they fall back to the binary placeholder.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
}

// hasImageExt reports whether path has a displayable image extension.
func hasImageExt(path string) bool {
	return imageExts[strings.ToLower(filepath.Ext(path))]
}

// decodeImage decodes PNG/JPEG/GIF data (GIF: first frame).
func decodeImage(data []byte) (image.Image, error) {
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
