# Inline Image Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** PNG/JPEG/GIF files open as half-block art (`▀` cells) fitted to the content pane, with a caption, falling back to the existing binary placeholder on any failure.

**Architecture:** A new `internal/ui/image.go` holds the pure pieces: extension check, stdlib decode, and a `renderImage` that scales with nearest-neighbor sampling and emits termenv-styled `▀` rows for an explicit color profile (deterministic, testable). The model pins the terminal profile once at `New`, `setContent` tries image decode before the binary check, and `ensureRendered` centers art + caption. Sidebar classification moves png/jpg/jpeg/gif from "dimmed binary" to normal displayable files.

**Tech Stack:** Go stdlib (`image`, `image/png`, `image/jpeg`, `image/gif`), termenv, lipgloss — no new dependencies.

**Spec:** `docs/superpowers/specs/2026-08-11-image-preview-design.md`

## Global Constraints

- No new module dependencies; decoders are the three stdlib ones.
- Display must never break: decode failure, pane too small, or profile below 256 colors → the existing looksBinary/placeholder path runs exactly as today.
- The running program must not query the terminal — `termenv.ColorProfile()` is called once in `New` and pinned into `Model.profile` (same pattern as `formatter`).
- Tests must not depend on the test terminal: they set `m.profile` explicitly and call `openFile` after construction; `renderImage` takes the profile as a parameter.
- GIF: first frame only (that is what `image.Decode` returns). No animation, zoom, pan, or new config keys.
- Run `gofmt -l .` (must print nothing), `go vet ./...`, and `go test -race ./...` before each commit.

---

### Task 1: image primitives (`hasImageExt`, `decodeImage`, `renderImage`)

**Files:**
- Create: `internal/ui/image.go`
- Create: `internal/ui/image_test.go`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces (used by Task 2):
  - `func hasImageExt(path string) bool` — true for .png/.jpg/.jpeg/.gif (case-insensitive).
  - `func decodeImage(data []byte) (image.Image, error)`
  - `func renderImage(img image.Image, maxW, maxH int, profile termenv.Profile) string` — half-block art of at most maxW columns × maxH rows; `""` when maxW < 2 or maxH < 1 or the image has no pixels.

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/image_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run 'TestHasImageExt|TestDecodeImage|TestRenderImage' -v`
Expected: FAIL to build with "undefined: hasImageExt" etc.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/image.go`:

```go
package ui

import (
	"bytes"
	"image"
	_ "image/gif"  // register stdlib decoders for image.Decode
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
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/ui/ -run 'TestHasImageExt|TestDecodeImage|TestRenderImage' -v`
Expected: PASS. Then `go test -race ./...` — all packages PASS.

- [ ] **Step 5: Verify formatting and vet, then commit**

Run: `gofmt -l . && go vet ./...`
Expected: no output from gofmt, no vet errors.

```bash
git add internal/ui/image.go internal/ui/image_test.go
git commit -m "Add half-block image rendering primitives"
```

---

### Task 2: wire image tabs into the model and sidebar

**Files:**
- Modify: `internal/ui/model.go` (tab struct + `setContent` ~line 55-85, Model struct ~line 90, `New` ~line 140-155, `openFile` ~line 380, `reload` ~line 440, `ensureRendered` ~line 470)
- Modify: `internal/ui/binary.go` (`binaryExts` map)
- Modify: `internal/ui/binary_test.go` (`TestHasBinaryExt`)
- Modify: `README.md` (Features list)
- Test: `internal/ui/image_test.go` (append the model-level tests to the same file)

**Interfaces:**
- Consumes: `hasImageExt`, `decodeImage`, `renderImage` (Task 1).
- Produces: `tab.img image.Image` field; `Model.profile termenv.Profile` field; `setContent` signature becomes `func (t *tab) setContent(data []byte, profile termenv.Profile)`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/image_test.go` (reuses `encodePNG` from Task 1):

```go
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
```

Add `"path/filepath"` to the test file imports.

In `internal/ui/binary_test.go`, update `TestHasBinaryExt`: png/jpg are no longer binary extensions. Replace the two lists with:

```go
	binary := []string{"a.zip", "b.pdf", "c.woff2", "d.exe", "e.webp"}
	...
	text := []string{"a.md", "b.go", "c.svg", "d.toml", "Makefile", "e.png", "f.JPG", "g.gif"}
```

(the loop bodies stay the same).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run 'TestOpenImage|TestCorruptImage|TestLowColor|TestHasBinaryExt' -v`
Expected: FAIL to build with "m.profile undefined" / "tb.img undefined"; `TestHasBinaryExt` fails on png once it builds.

- [ ] **Step 3: Implement**

1. `internal/ui/binary.go` — remove `".png"`, `".jpg"`, `".jpeg"`, `".gif"` from `binaryExts` (keep `.bmp`, `.webp`, `.ico` and the rest), and update the map comment to note that displayable image extensions live in `imageExts` (image.go).

2. `internal/ui/model.go`:

`tab` struct — after the `binary bool` field add:

```go
	img          image.Image // decoded image; nil = not an image tab
```

Add `"image"` to the imports.

`Model` struct — after `formatter string` add:

```go
	profile     termenv.Profile // terminal color capability, pinned at startup
```

In `New`, in the Model literal after `formatter:` add:

```go
		profile:     termenv.ColorProfile(),
```

`setContent` — new signature and image branch first:

```go
// setContent stores file data on the tab. Images become half-block
// art tabs; other binary data is replaced with a placeholder so raw
// control bytes never reach the terminal.
func (t *tab) setContent(data []byte, profile termenv.Profile) {
	t.img = nil
	if hasImageExt(t.path) && (profile == termenv.TrueColor || profile == termenv.ANSI256) {
		if img, err := decodeImage(data); err == nil {
			t.img = img
			t.binary = false
			t.raw = ""
			return
		}
	}
	if looksBinary(data) {
		t.binary = true
		t.raw = fmt.Sprintf("%s\n\nbinary file (%s) — not displayed", t.title, humanSize(len(data)))
		return
	}
	t.binary = false
	t.raw = string(data)
}
```

Update both call sites: `t.setContent(data, m.profile)` in `openFile` and in `reload`.

`ensureRendered` — insert the image branch before the binary branch:

```go
	content := t.raw
	if t.img != nil {
		ib := t.img.Bounds()
		art := renderImage(t.img, w-2, m.contentInnerHeight()-3, m.profile)
		caption := m.styles.dimmed.Render(fmt.Sprintf("%s  %d×%d", t.title, ib.Dx(), ib.Dy()))
		content = lipgloss.Place(w, m.contentInnerHeight(), lipgloss.Center, lipgloss.Center, art+"\n\n"+caption)
	} else if t.binary {
```

(`-3` leaves room for the blank line and caption; when `renderImage` returns `""` the pane still shows the centered caption, which is the graceful-degradation path for tiny panes.)

3. `README.md` — add a Features bullet after the reader/source one:

```markdown
- **Inline image preview** — PNG/JPEG/GIF files render as half-block pixel art fitted to the pane
```

- [ ] **Step 4: Run the full test suite**

Run: `go test -race ./...`
Expected: all packages PASS (including the untouched binary/placeholder tests).

- [ ] **Step 5: Verify formatting and vet, then commit**

Run: `gofmt -l . && go vet ./...`
Expected: no output from gofmt, no vet errors.

```bash
git add internal/ui/model.go internal/ui/binary.go internal/ui/binary_test.go internal/ui/image_test.go README.md
git commit -m "Render PNG/JPEG/GIF tabs as inline half-block art"
```

- [ ] **Step 6: Manual verification in tmux**

```bash
go build -o /tmp/mado-img .
tmux new-session -d -s madoimg -x 120 -y 40 '/tmp/mado-img'
sleep 1
tmux send-keys -t madoimg 'a'; sleep 0.4
tmux send-keys -t madoimg Enter; sleep 0.4      # expand docs/
tmux send-keys -t madoimg 'j'; sleep 0.3
tmux send-keys -t madoimg Enter; sleep 0.4      # expand assets/
tmux send-keys -t madoimg 'j'; sleep 0.3
tmux send-keys -t madoimg Enter; sleep 1        # open modes.png
tmux capture-pane -t madoimg -p | grep -c '▀'   # expect many half-block rows
tmux capture-pane -t madoimg -p | grep 'modes.png'   # caption with dimensions
tmux send-keys -t madoimg 'r'; sleep 0.5        # reload keeps the image
tmux send-keys -t madoimg 'q'
tmux kill-session -t madoimg 2>/dev/null
```

Expected: the pane shows colored half-block art with the `modes.png  W×H` caption, layout intact, quit clean. Note (macOS host): no `timeout` command; use the `sleep` calls as shown.
