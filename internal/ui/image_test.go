package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// encodePNG returns a w×h PNG filled with c, as a string.
func encodePNG(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestHasImageExt(t *testing.T) {
	for _, p := range []string{"a.png", "b.JPG", "c.jpeg", "d.gif"} {
		if !hasImageExt(p) {
			t.Errorf("hasImageExt(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"a.md", "b.zip", "c.webp", "d.bmp", "noext"} {
		if hasImageExt(p) {
			t.Errorf("hasImageExt(%q) = true, want false", p)
		}
	}
}

func TestDecodeImageRoundTrip(t *testing.T) {
	img, err := decodeImage([]byte(encodePNG(t, 4, 4, color.RGBA{255, 0, 0, 255})))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("bounds = %v, want 4x4", b)
	}
	if _, err := decodeImage([]byte("not an image at all")); err == nil {
		t.Error("garbage should fail to decode")
	}
}

func TestRenderImageFitsBounds(t *testing.T) {
	img, _ := decodeImage([]byte(encodePNG(t, 16, 16, color.RGBA{200, 30, 30, 255})))
	out := renderImage(img, 4, 4, termenv.TrueColor)
	lines := strings.Split(out, "\n")
	if len(lines) > 4 {
		t.Errorf("rendered %d rows, want <= 4", len(lines))
	}
	for i, l := range lines {
		if lipgloss.Width(l) > 4 {
			t.Errorf("row %d width %d, want <= 4", i, lipgloss.Width(l))
		}
	}
	if !strings.Contains(out, "▀") {
		t.Error("output should contain half-block cells")
	}
	if !strings.Contains(out, "38;2;") {
		t.Error("TrueColor profile should emit 24-bit foreground sequences")
	}
}

func TestRenderImage256Profile(t *testing.T) {
	img, _ := decodeImage([]byte(encodePNG(t, 8, 8, color.RGBA{0, 120, 255, 255})))
	out := renderImage(img, 4, 4, termenv.ANSI256)
	if !strings.Contains(out, "38;5;") {
		t.Error("ANSI256 profile should emit 256-color sequences")
	}
	if strings.Contains(out, "38;2;") {
		t.Error("ANSI256 profile must not emit 24-bit sequences")
	}
}

func TestRenderImageUnusablePane(t *testing.T) {
	img, _ := decodeImage([]byte(encodePNG(t, 8, 8, color.RGBA{0, 0, 0, 255})))
	if out := renderImage(img, 1, 0, termenv.TrueColor); out != "" {
		t.Errorf("unusable pane should render empty, got %q", out)
	}
}
