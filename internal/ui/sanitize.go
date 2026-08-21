package ui

import (
	"fmt"
	"strings"
)

// sanitizeText makes a string safe to print to the terminal by
// replacing the control characters a terminal acts on with a visible
// caret escape, keeping only the whitespace that rendering needs.
//
// File contents and file names are data to display, not commands. Left
// as they are, an escape sequence in a file mado is showing drives the
// terminal instead of appearing in it: OSC 52 writes the clipboard, a
// title sequence can be echoed back into the shell's input on some
// terminals, cursor movement lets one line overwrite another. Whoever
// writes the file chooses those bytes — an editor, a build, an agent —
// and with auto-reload they take effect with no keystroke from the
// reader.
func sanitizeText(s string) string {
	if !strings.ContainsFunc(s, isTerminalControl) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !isTerminalControl(r) {
			b.WriteRune(r)
			continue
		}
		switch {
		case r == 0x7f:
			b.WriteString("^?")
		case r < 0x20:
			// Caret notation, as cat -v and less show them: ESC is ^[.
			b.WriteByte('^')
			b.WriteByte(byte('@' + r))
		default:
			// C1 controls; 0x9b is a single-byte CSI on some terminals.
			fmt.Fprintf(&b, "<%02X>", r)
		}
	}
	return b.String()
}

// isTerminalControl reports whether r is a control character a terminal
// would act on. Newline and tab are how text is laid out, so they stay.
func isTerminalControl(r rune) bool {
	switch {
	case r == '\n' || r == '\t':
		return false
	case r < 0x20 || r == 0x7f:
		return true
	case r >= 0x80 && r <= 0x9f:
		return true
	default:
		return false
	}
}
