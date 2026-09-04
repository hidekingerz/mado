package ui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

// wrapText folds every line of s to width columns. A break goes at a
// space where one will do, between any two East Asian characters —
// Japanese has no spaces to offer — and inside a word only when the
// word alone is wider than the row. Wide characters count as the two
// columns they take, escape sequences take none and stay whole, and
// a closing mark such as 。 or 」 stays with the character before it
// rather than starting a row.
func wrapText(s string, width int) string {
	if width < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, foldLine(expandTabs(line), width, width, "")...)
	}
	return strings.Join(out, "\n")
}

// tabStop is how far a terminal draws a tab: to the next multiple of
// eight columns.
const tabStop = 8

// expandTabs replaces each tab with the spaces a terminal would draw
// for it, so the line's width can be counted. Escape sequences take
// no columns.
func expandTabs(line string) string {
	if !strings.Contains(line, "\t") {
		return line
	}
	var b strings.Builder
	col := 0
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\t':
			n := tabStop - col%tabStop
			b.WriteString(strings.Repeat(" ", n))
			col += n
		case r == 0x1b && i+1 < len(rs) && rs[i+1] == '[':
			j := csiEnd(rs, i)
			b.WriteString(string(rs[i:j]))
			i = j - 1
		default:
			b.WriteRune(r)
			col += runewidth.RuneWidth(r)
		}
	}
	return b.String()
}

// csiEnd returns the index just past the CSI sequence starting at
// rs[i], which is ESC '[' followed by parameter bytes and one final
// byte in the range 0x40-0x7e.
func csiEnd(rs []rune, i int) int {
	j := i + 2
	for j < len(rs) && (rs[j] < 0x40 || rs[j] > 0x7e) {
		j++
	}
	if j < len(rs) {
		j++
	}
	return j
}

// hardWrapLines folds every line of already laid-out text that is
// wider than width, keeping each folded line's indent on the rows it
// continues on, so a paragraph that glamour set inside a margin keeps
// that margin when it has to be split. The indent is measured on the
// visible text: glamour writes its style sequences before the
// margin's spaces.
func hardWrapLines(s string, width int) string {
	if width < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = expandTabs(line)
		if lipgloss.Width(line) <= width {
			out = append(out, line)
			continue
		}
		visible := ansi.Strip(line)
		indent := len(visible) - len(strings.TrimLeft(visible, " "))
		if width-indent < 4 {
			indent = 0 // too narrow to keep a margin and still show text
		}
		out = append(out, foldLine(line, width, width-indent, strings.Repeat(" ", indent))...)
	}
	return strings.Join(out, "\n")
}

// A token is the smallest piece foldLine moves as a whole: a run of
// spaces, a word in a script that uses them, one East Asian character
// with the marks that must not be parted from it, or an escape
// sequence, which takes no room.
type token struct {
	text  string
	width int
	space bool
}

// foldLine breaks one line into rows of at most first columns for the
// first row and rest for the ones after it, each of which is prefixed
// with pad. Spaces at a break are dropped.
func foldLine(line string, first, rest int, pad string) []string {
	var rows []string
	var row strings.Builder
	limit, used := first, 0
	var pending string // spaces seen since the last token, not yet placed

	flush := func() {
		rows = append(rows, row.String())
		row.Reset()
		row.WriteString(pad)
		limit, used = rest, 0
		pending = ""
	}
	place := func(t token) {
		if t.width == 0 {
			row.WriteString(t.text)
			return
		}
		if used+len(pending)+t.width > limit && used > 0 {
			flush()
		}
		if pending != "" {
			row.WriteString(pending)
			used += len(pending)
			pending = ""
		}
		// What does not fit in the rest of the row — a word wider than
		// a row, or one that follows a row's leading spaces — is cut
		// where the row ends rather than leaving the row blank.
		for used+t.width > limit {
			head, tail, w := splitAt(t.text, limit-used)
			if head == "" {
				if used == 0 {
					break // narrower than one character: let it overflow
				}
				flush()
				continue
			}
			row.WriteString(head)
			used += w
			flush()
			t.text, t.width = tail, t.width-w
		}
		row.WriteString(t.text)
		used += t.width
	}

	for _, t := range tokenize(line) {
		if t.space {
			pending += t.text
			continue
		}
		place(t)
	}
	// Trailing spaces are invisible; keep them only where they fit.
	if pending != "" && used+len(pending) <= limit {
		row.WriteString(pending)
	}
	rows = append(rows, row.String())
	return rows
}

// splitAt cuts text after as many leading runes as fit in cols,
// returning the head, the tail and the head's width.
func splitAt(text string, cols int) (string, string, int) {
	w := 0
	rs := []rune(text)
	for i := 0; i < len(rs); i++ {
		if rs[i] == 0x1b && i+1 < len(rs) && rs[i+1] == '[' {
			i = csiEnd(rs, i) - 1
			continue
		}
		rw := runewidth.RuneWidth(rs[i])
		if w+rw > cols {
			return string(rs[:i]), string(rs[i:]), w
		}
		w += rw
	}
	return text, "", w
}

// tokenize splits line into tokens. A CSI escape sequence is a token
// of width zero; a run of spaces is a space token; a wide character
// is a token of its own, together with any following marks that may
// not begin a row and, when it may not end one, the character after
// it; everything else is grouped into words.
func tokenize(line string) []token {
	var toks []token
	var word strings.Builder
	wordW := 0
	endWord := func() {
		if word.Len() > 0 {
			toks = append(toks, token{text: word.String(), width: wordW})
			word.Reset()
			wordW = 0
		}
	}
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == 0x1b && i+1 < len(rs) && rs[i+1] == '[':
			// A sequence inside a word — highlighting colors part of
			// it — stays in the word rather than becoming a place to
			// break.
			j := csiEnd(rs, i)
			if word.Len() > 0 {
				word.WriteString(string(rs[i:j]))
			} else {
				toks = append(toks, token{text: string(rs[i:j])})
			}
			i = j - 1
		case r == ' ' || r == '\t':
			endWord()
			j := i
			for j < len(rs) && (rs[j] == ' ' || rs[j] == '\t') {
				j++
			}
			toks = append(toks, token{text: string(rs[i:j]), width: j - i, space: true})
			i = j - 1
		case eastAsian(r):
			endWord()
			j := i + 1
			if noEnd(r) && j < len(rs) {
				j++
			}
			for j < len(rs) && noStart(rs[j]) {
				j++
			}
			toks = append(toks, token{text: string(rs[i:j]), width: runewidth.StringWidth(string(rs[i:j]))})
			i = j - 1
		default:
			word.WriteRune(r)
			wordW += runewidth.RuneWidth(r)
		}
	}
	endWord()
	return toks
}

// eastAsian reports whether r is a character a row may break beside
// without a space: a wide character, or a half-width form of one.
func eastAsian(r rune) bool {
	return runewidth.RuneWidth(r) == 2 || unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// noStart marks may not begin a row (行頭禁則).
func noStart(r rune) bool {
	return strings.ContainsRune("、。，．・：；？！ー〜～ゝゞ々」』）】〕｝〉》ぁぃぅぇぉっゃゅょゎァィゥェォッャュョヮ,.!?;:)]}", r)
}

// noEnd marks may not end a row (行末禁則).
func noEnd(r rune) bool {
	return strings.ContainsRune("「『（【〔｛〈《", r)
}
