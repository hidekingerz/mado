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

func rowsOf(s string) []string { return strings.Split(s, "\n") }

func TestWrapTextFillsTheRowBeforeBreakingJapanese(t *testing.T) {
	// After a space, a run of Japanese must not move to the next row
	// whole: it can break between any two characters.
	in := "- **設定パネル** — `,` を押すと、すべての設定をその場で編集できるパネルが開きます。変更は即座に反映されます"
	rows := rowsOf(wrapText(in, 40))
	if w := lipgloss.Width(rows[0]); w < 38 {
		t.Errorf("first row should be filled to the width, got %d columns: %q", w, rows[0])
	}
	for i, r := range rows {
		if w := lipgloss.Width(r); w > 40 {
			t.Errorf("row %d is %d wide: %q", i, w, r)
		}
		if strings.HasPrefix(r, "。") || strings.HasPrefix(r, "、") {
			t.Errorf("row %d starts with punctuation that belongs to the previous row: %q", i, r)
		}
	}
	if squash(strings.Join(rows, "")) != squash(in) {
		t.Errorf("text changed:\n%s", strings.Join(rows, "\n"))
	}
}

func TestWrapTextKeepsLatinWordsWhole(t *testing.T) {
	got := rowsOf(wrapText("the quick brown fox jumps", 10))
	want := []string{"the quick", "brown fox", "jumps"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

func TestWrapTextSplitsAnOverlongWord(t *testing.T) {
	got := rowsOf(wrapText("abcdefghijkl", 5))
	want := []string{"abcde", "fghij", "kl"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

func TestWrapTextKeepsEscapeSequencesWhole(t *testing.T) {
	in := "\x1b[31m" + jpSentence + "\x1b[0m"
	out := wrapText(in, 20)
	if !strings.Contains(out, "\x1b[31m") || !strings.Contains(out, "\x1b[0m") {
		t.Errorf("escape sequences must survive: %q", out)
	}
	for i, r := range rowsOf(out) {
		if w := lipgloss.Width(r); w > 20 {
			t.Errorf("row %d is %d wide: %q", i, w, ansi.Strip(r))
		}
	}
	if squash(out) != squash(in) {
		t.Errorf("text changed:\n%s", ansi.Strip(out))
	}
}

func TestWrapTextKeepsBlankLinesAndIndent(t *testing.T) {
	got := wrapText("  indented line\n\nnext", 40)
	if got != "  indented line\n\nnext" {
		t.Errorf("short lines pass through unchanged, got %q", got)
	}
}

func TestReaderModeFillsRowsBeforeBreakingJapanese(t *testing.T) {
	md := "- **設定パネル** — `,` を押すと、すべての設定をその場で編集できるパネルが開きます。変更は即座に反映され、コメントはそのままに設定ファイルへ1キーずつ書き戻されます\n"
	m := testModel(t, map[string]string{"a.md": md}, "a.md")
	w := m.contentInnerWidth()
	for _, r := range rowsOf(m.tabs[0].content) {
		if strings.Contains(r, "設定パネル") {
			if got := lipgloss.Width(r); got < w-2 {
				t.Errorf("the row holding the bullet should be filled to about %d columns, got %d: %q", w, got, ansi.Strip(r))
			}
			return
		}
	}
	t.Fatalf("bullet not rendered:\n%s", m.tabs[0].content)
}

func TestWrapTextDropsTrailingSpacesThatDoNotFit(t *testing.T) {
	// glamour pads table cells with spaces; invisible padding must not
	// push a row past the pane.
	in := "abc" + strings.Repeat(" ", 20)
	got := hardWrapLines(in, 10)
	if strings.Count(got, "\n") != 0 || lipgloss.Width(got) > 10 {
		t.Errorf("padding should be dropped, not folded: %q", got)
	}
	if !strings.HasPrefix(got, "abc") {
		t.Errorf("text lost: %q", got)
	}
}

func TestWrapTextExpandsTabsToTerminalColumns(t *testing.T) {
	// A terminal draws a tab as spaces up to the next multiple of 8,
	// so the width of a tabbed line is counted that way too.
	got := wrapText("\tif x {", 40)
	if got != "        if x {" {
		t.Errorf("leading tab = %q", got)
	}
	got = wrapText("ab\tc", 40)
	if got != "ab      c" {
		t.Errorf("mid-line tab = %q", got)
	}
	// 8 + 34 wide: must fold, since the terminal would show 42 columns.
	rows := rowsOf(wrapText("\t"+strings.Repeat("x", 34), 40))
	if len(rows) != 2 {
		t.Errorf("a tabbed line wider than the pane must fold, got %q", rows)
	}
}

func TestWrapTextKeepsAStyledWordWhole(t *testing.T) {
	// Highlighting puts escape sequences inside a word; they must not
	// become a place to break.
	rows := rowsOf(wrapText("xx abc\x1b[31mdef\x1b[0m", 7))
	if len(rows) != 2 || ansi.Strip(rows[0]) != "xx" || ansi.Strip(rows[1]) != "abcdef" {
		t.Errorf("rows = %q", rows)
	}
	rows = rowsOf(wrapText("\x1b[31mabcdefghijkl\x1b[0m", 5))
	var vis []string
	for _, r := range rows {
		vis = append(vis, ansi.Strip(r))
	}
	if strings.Join(vis, "|") != "abcde|fghij|kl" {
		t.Errorf("an overlong styled word splits by visible columns, got %q", vis)
	}
}
