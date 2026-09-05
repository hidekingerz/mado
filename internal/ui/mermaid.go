package ui

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pgavlin/mermaid-ascii/pkg/diagram"
	"github.com/pgavlin/mermaid-ascii/pkg/render"
	"github.com/sirupsen/logrus"
)

// Glamour lays a document out inside a margin of documentMargin
// columns on each side, and indents a code block's lines by a further
// codeBlockIndent before wrapping the whole to the requested width; a
// drawing must fit in what is left or it wraps and falls apart.
const (
	documentMargin  = 2
	codeBlockIndent = 2
)

// The drawing library reports layout trouble through logrus's global
// logger, which by default writes to stderr — straight onto the
// screen underneath the TUI. mado has nothing to say through logrus,
// so silence it.
func init() {
	logrus.SetOutput(io.Discard)
}

// renderMermaidBlocks replaces every closed ```mermaid fence in md
// with a ```text fence holding the diagram drawn in box characters,
// so the reader view shows the picture instead of the source. width
// is the wrap width the document will be rendered at. A block that
// cannot be drawn, or whose drawing would not fit a code block at
// that width, is left as it is — the source is still readable, a
// wrapped drawing is not.
func renderMermaidBlocks(md string, width int) string {
	lines := strings.Split(md, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		if !isMermaidFence(lines[i]) {
			out = append(out, lines[i])
			continue
		}
		end := closingFence(lines, i+1)
		if end < 0 {
			out = append(out, lines[i:]...)
			break
		}
		drawn, ok := drawMermaid(strings.Join(lines[i+1:end], "\n"), width-2*documentMargin-codeBlockIndent)
		if !ok {
			out = append(out, lines[i:end+1]...)
		} else {
			out = append(out, "```text", drawn, "```")
		}
		i = end
	}
	return strings.Join(out, "\n")
}

func isMermaidFence(line string) bool {
	return strings.TrimRight(line, " \t") == "```mermaid"
}

// closingFence returns the index of the first line at or after from
// that closes a fence, or -1.
func closingFence(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == "```" {
			return i
		}
	}
	return -1
}

// drawMermaid draws src, reporting false when it cannot be drawn or
// does not fit in maxWidth columns.
func drawMermaid(src string, maxWidth int) (string, bool) {
	if isFlowchart(src) && !isASCII(src) {
		// The flowchart renderer places text byte by byte, so a
		// multi-byte character comes out as mojibake. Until it
		// handles runes, such a diagram reads better as source.
		return "", false
	}
	drawn, err := render.Render(src, diagram.DefaultConfig())
	if err != nil {
		return "", false
	}
	drawn = strings.TrimRight(drawn, "\n")
	if drawn == "" {
		return "", false
	}
	for _, l := range strings.Split(drawn, "\n") {
		if lipgloss.Width(l) > maxWidth {
			return "", false
		}
	}
	return drawn, true
}

// isFlowchart reports whether src is a flowchart or graph, the
// diagram types drawn by the byte-wise renderer.
func isFlowchart(src string) bool {
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "%%") {
			continue
		}
		return strings.HasPrefix(l, "flowchart") || strings.HasPrefix(l, "graph")
	}
	return false
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
