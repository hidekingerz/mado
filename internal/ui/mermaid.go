package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pgavlin/mermaid-ascii/pkg/diagram"
	"github.com/pgavlin/mermaid-ascii/pkg/render"
)

// codeBlockMargin is the indent glamour's styles put on both sides of
// a code block, which a drawn diagram has to fit inside.
const codeBlockMargin = 2

// renderMermaidBlocks replaces every closed ```mermaid fence in md
// with a ```text fence holding the diagram drawn in box characters,
// so the reader view shows the picture instead of the source. A
// block that cannot be drawn, or whose drawing is wider than the
// width columns available to a code block, is left as it is — the
// source is still readable, a wrapped drawing is not.
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
		drawn, ok := drawMermaid(strings.Join(lines[i+1:end], "\n"), width-2*codeBlockMargin)
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
