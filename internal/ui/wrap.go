package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
)

// wrapText folds s to width columns: at spaces where it can, and
// inside a word where it must. Word wrapping alone leaves a run with
// no space in it — a Japanese sentence, a long URL — on one line that
// the pane then cuts off; the hard wrap after it splits such a run
// at the width, counting wide characters as the two columns they
// take and leaving escape sequences whole.
func wrapText(s string, width int) string {
	if width < 1 {
		return s
	}
	return wrap.String(wordwrap.String(s, width), width)
}

// hardWrapLines folds every line of already laid-out text that is
// wider than width, keeping each folded line's indent on the rows it
// continues on, so a paragraph that glamour set inside a margin keeps
// that margin when a spaceless run has to be split. The indent is
// measured on the visible text: glamour writes its style sequences
// before the margin's spaces.
func hardWrapLines(s string, width int) string {
	if width < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if lipgloss.Width(line) <= width {
			out = append(out, line)
			continue
		}
		rows := strings.Split(wrap.String(line, width), "\n")
		visible := ansi.Strip(line)
		indent := len(visible) - len(strings.TrimLeft(visible, " "))
		if indent == 0 || indent >= width || len(rows) == 1 {
			out = append(out, rows...)
			continue
		}
		// The first row already carries the indent; re-fold the rest
		// to leave room for it on every following row.
		out = append(out, rows[0])
		pad := strings.Repeat(" ", indent)
		rest := strings.Join(rows[1:], "")
		for _, r := range strings.Split(wrap.String(rest, width-indent), "\n") {
			out = append(out, pad+r)
		}
	}
	return strings.Join(out, "\n")
}
