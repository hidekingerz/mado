package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// numberLines prefixes each logical line of src with a right-aligned
// line number, like vi's :set nu. Continuation rows of a wrapped line
// get a blank gutter, so the numbers track the file rather than the
// fold. width is the total width available; when it is too narrow to
// fit a gutter plus any text, src is wrapped plain instead.
func numberLines(src string, width int, gutter lipgloss.Style) string {
	lines := strings.Split(src, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	numW := len(strconv.Itoa(len(lines)))
	textW := width - numW - 1
	if textW < 1 {
		return wrapText(src, width)
	}
	pad := strings.Repeat(" ", numW+1)
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		segs := strings.Split(wrapText(line, textW), "\n")
		b.WriteString(gutter.Render(fmt.Sprintf("%*d", numW, i+1)))
		b.WriteByte(' ')
		b.WriteString(segs[0])
		for _, seg := range segs[1:] {
			b.WriteByte('\n')
			b.WriteString(pad)
			b.WriteString(seg)
		}
	}
	return b.String()
}
