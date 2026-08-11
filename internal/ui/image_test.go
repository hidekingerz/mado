package ui

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// encodePNG returns a w×h PNG filled with c, as a string.
func encodePNG(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, solidRGBA(w, h, c)); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// encodeGIF returns a w×h GIF filled with c, as a string.
func encodeGIF(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, solidRGBA(w, h, c), nil); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// encodeJPEG returns a w×h JPEG filled with c, as a string.
func encodeJPEG(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, solidRGBA(w, h, c), nil); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// solidRGBA returns a w×h image filled with c.
func solidRGBA(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// buildGIFHeader hand-builds a minimal 13-byte GIF89a header (magic +
// logical screen descriptor, no global color table) declaring a w×h
// image. image.DecodeConfig only needs these 13 bytes to report the
// declared dimensions, without ever reading frame data.
func buildGIFHeader(t *testing.T, w, h int) []byte {
	t.Helper()
	if w < 0 || w > 0xFFFF || h < 0 || h > 0xFFFF {
		t.Fatalf("dimensions %dx%d out of GIF logical screen descriptor range", w, h)
	}
	var buf bytes.Buffer
	buf.WriteString("GIF89a")
	buf.WriteByte(byte(w))
	buf.WriteByte(byte(w >> 8))
	buf.WriteByte(byte(h))
	buf.WriteByte(byte(h >> 8))
	buf.WriteByte(0x00) // packed fields: no global color table
	buf.WriteByte(0x00) // background color index
	buf.WriteByte(0x00) // pixel aspect ratio
	return buf.Bytes()
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
		t.Errorf("png bounds = %v, want 4x4", b)
	}

	gifImg, err := decodeImage([]byte(encodeGIF(t, 4, 4, color.RGBA{0, 255, 0, 255})))
	if err != nil {
		t.Fatal(err)
	}
	if b := gifImg.Bounds(); b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("gif bounds = %v, want 4x4", b)
	}

	jpegImg, err := decodeImage([]byte(encodeJPEG(t, 4, 4, color.RGBA{0, 0, 255, 255})))
	if err != nil {
		t.Fatal(err)
	}
	if b := jpegImg.Bounds(); b.Dx() != 4 || b.Dy() != 4 {
		t.Errorf("jpeg bounds = %v, want 4x4", b)
	}

	if _, err := decodeImage([]byte("not an image at all")); err == nil {
		t.Error("garbage should fail to decode")
	}
}

// TestDecodeImageRejectsHugeDimensions guards against a small crafted file
// declaring huge dimensions (a classic decompression-bomb shape): without
// a cap, image.Decode would allocate the full pixel buffer for whatever
// the header claims. The hand-built GIF header here declares 60000x60000
// (3.6 billion pixels) but is only 13 bytes long, so if decodeImage ever
// fell through to a full image.Decode, it would either allocate ~3.6 GB
// or error out slowly — this test asserts it is rejected immediately via
// image.DecodeConfig instead.
func TestDecodeImageRejectsHugeDimensions(t *testing.T) {
	huge := buildGIFHeader(t, 60000, 60000)

	start := time.Now()
	_, err := decodeImage(huge)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("decodeImage should reject a 60000x60000 image")
	}
	if !errors.Is(err, errImageTooLarge) {
		t.Errorf("decodeImage error = %v, want it to wrap errImageTooLarge", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("decodeImage took %v; should reject via DecodeConfig before attempting a full decode", elapsed)
	}

	// A normal small image must still decode fine.
	if _, err := decodeImage([]byte(encodePNG(t, 4, 4, color.RGBA{0, 255, 0, 255}))); err != nil {
		t.Errorf("small image should still decode: %v", err)
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

func TestOpenImageShowsHalfBlocks(t *testing.T) {
	m := testModel(t, map[string]string{"pic.png": encodePNG(t, 8, 8, color.RGBA{200, 30, 30, 255}), "a.md": "# A"})
	m.profile = termenv.TrueColor
	m.openFile(filepath.Join(m.root.Path, "pic.png"))
	if len(m.tabs) != 1 {
		t.Fatalf("tabs = %d, want 1", len(m.tabs))
	}
	tb := m.tabs[0]
	if tb.img == nil {
		t.Fatal("tab should be an image tab")
	}
	view := tb.vp.View()
	if !strings.Contains(view, "▀") {
		t.Error("viewport should contain half-block art")
	}
	if !strings.Contains(view, "pic.png") || !strings.Contains(view, "8×8") {
		t.Errorf("viewport should contain the caption, got %q", view)
	}
}

func TestCorruptImageFallsBackToPlaceholder(t *testing.T) {
	m := testModel(t, map[string]string{"bad.png": "\x89PNG\r\n\x1a\n\x00broken"})
	m.profile = termenv.TrueColor
	m.openFile(filepath.Join(m.root.Path, "bad.png"))
	tb := m.tabs[0]
	if tb.img != nil {
		t.Fatal("corrupt png must not become an image tab")
	}
	if !tb.binary || !strings.Contains(tb.vp.View(), "binary file") {
		t.Error("corrupt png should fall back to the binary placeholder")
	}
}

func TestLowColorProfileFallsBack(t *testing.T) {
	m := testModel(t, map[string]string{"pic.png": encodePNG(t, 8, 8, color.RGBA{10, 10, 10, 255})})
	m.profile = termenv.Ascii
	m.openFile(filepath.Join(m.root.Path, "pic.png"))
	if m.tabs[0].img != nil {
		t.Error("Ascii profile should not create image tabs")
	}
	if !strings.Contains(m.tabs[0].vp.View(), "binary file") {
		t.Error("Ascii profile should show the placeholder")
	}
}

// TestImageTabRerendersAfterHeightOnlyResize reproduces a stale-art bug:
// an inactive image tab must re-render when the pane height changes even
// though its width (the old cache key) stayed the same.
func TestImageTabRerendersAfterHeightOnlyResize(t *testing.T) {
	m := testModel(t, map[string]string{
		// Tall, narrow image so the render is height-bound (not
		// width-bound) at both the original and resized heights.
		"pic.png": encodePNG(t, 4, 200, color.RGBA{200, 30, 30, 255}),
		"a.md":    "# A",
	})
	m.profile = termenv.TrueColor
	m.openFile(filepath.Join(m.root.Path, "pic.png"))
	imgIdx := m.active
	before := strings.Count(m.tabs[imgIdx].vp.View(), "▀")
	if before == 0 {
		t.Fatal("expected half-block rows before resize")
	}

	m.openFile(filepath.Join(m.root.Path, "a.md")) // makes the image tab inactive
	if m.active == imgIdx {
		t.Fatal("second tab should now be active")
	}

	// Height-only resize: width is unchanged from testModel's initial
	// WindowSizeMsg{Width: 100, Height: 30}.
	resized, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	m = resized.(Model)

	m.switchTab(-1) // back to the image tab
	if m.active != imgIdx {
		t.Fatalf("active = %d, want %d", m.active, imgIdx)
	}
	after := strings.Count(m.tabs[imgIdx].vp.View(), "▀")

	if after == before {
		t.Errorf("image tab did not re-render for the new height: before=%d after=%d rows of ▀", before, after)
	}
}
