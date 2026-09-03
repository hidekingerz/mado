package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hidekingerz/mado/internal/config"
)

// A sentence with no spaces to break at: wordwrap alone cannot fold it.
const jpSentence = "窓はディレクトリ内のマークダウンを読むためのターミナルリーダーで、ファイルツリーとタブと検索と図で読みやすくします。"

// squash strips styling and whitespace so wrapped text can be compared
// with what went in.
func squash(s string) string {
	return strings.Join(strings.Fields(ansi.Strip(s)), "")
}

func assertFolded(t *testing.T, content string, width int, want string) {
	t.Helper()
	for _, l := range strings.Split(content, "\n") {
		if w := lipgloss.Width(l); w > width {
			t.Errorf("line is %d columns, pane is %d: %q", w, width, ansi.Strip(l))
		}
	}
	if !strings.Contains(squash(content), squash(want)) {
		t.Errorf("text was lost in wrapping:\n%s", ansi.Strip(content))
	}
}

func TestReaderModeFoldsTextWithoutSpaces(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# 見出し\n\n" + jpSentence + "\n"}, "a.md")
	assertFolded(t, m.tabs[0].content, m.contentInnerWidth(), jpSentence)
}

func TestSourceModeFoldsTextWithoutSpaces(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.Style = "notty"
	cfg.Theme.DefaultMode = "source"
	m := testModelWithConfig(t, cfg, map[string]string{"a.md": jpSentence + "\n"}, "a.md")
	assertFolded(t, m.tabs[0].content, m.contentInnerWidth(), jpSentence)
	m = update(t, m, keyRune('n'))
	assertFolded(t, m.tabs[0].content, m.contentInnerWidth(), jpSentence)
	if !strings.Contains(m.tabs[0].content, "1 ") || strings.Count(m.tabs[0].content, "\n") < 1 {
		t.Errorf("line numbers should number the folded line once and continue it on blank-gutter rows:\n%s", m.tabs[0].content)
	}
}

func TestSearchJumpCountsFoldedRowsOfTextWithoutSpaces(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.Style = "notty"
	cfg.Theme.DefaultMode = "source"
	raw := jpSentence + "\ntarget line\n"
	m := testModelWithConfig(t, cfg, map[string]string{"a.md": raw}, "a.md")
	rows := strings.Split(m.tabs[0].content, "\n")
	want := -1
	for i, r := range rows {
		if strings.Contains(r, "target line") {
			want = i
			break
		}
	}
	if want < 0 {
		t.Fatalf("target line not rendered:\n%s", m.tabs[0].content)
	}
	if got := sourceRow(raw, 2, m.tabs[0].vp.Width-2, false); got != want {
		t.Errorf("sourceRow = %d, but the line is rendered on row %d", got, want)
	}
}

func TestHardWrapLinesKeepsEveryLineOnce(t *testing.T) {
	in := jpSentence + "\nsecond\nthird"
	got := hardWrapLines(in, 30)
	rows := strings.Split(got, "\n")
	if rows[len(rows)-1] != "third" || rows[len(rows)-2] != "second" {
		t.Errorf("the lines after a folded one must follow it once each:\n%s", got)
	}
	if strings.Count(got, "second") != 1 || squash(got) != squash(in) {
		t.Errorf("folding must not repeat or drop text:\n%s", got)
	}
	for _, r := range rows {
		if w := lipgloss.Width(r); w > 30 {
			t.Errorf("row is %d wide: %q", w, r)
		}
	}
}

func TestHardWrapLinesKeepsAnIndentSetBehindEscapes(t *testing.T) {
	// glamour writes its margin after style sequences, as in
	// "\x1b[38;5;252m\x1b[0m  text\x1b[0m".
	in := "\x1b[38;5;252m\x1b[0m  " + jpSentence + "\x1b[0m"
	rows := strings.Split(hardWrapLines(in, 30), "\n")
	if len(rows) < 3 {
		t.Fatalf("expected the sentence to fold into several rows, got %d", len(rows))
	}
	for i, r := range rows {
		if w := lipgloss.Width(r); w > 30 {
			t.Errorf("row %d is %d wide: %q", i, w, ansi.Strip(r))
		}
		if !strings.HasPrefix(ansi.Strip(r), "  ") {
			t.Errorf("row %d lost the two-column margin: %q", i, ansi.Strip(r))
		}
	}
}
