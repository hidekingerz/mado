package ui

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sirupsen/logrus"

	"github.com/hidekingerz/mado/internal/config"
)

const flowchart = "```mermaid\nflowchart TD\n    A[Start] --> B[End]\n```"

func TestRenderMermaidBlocksReplacesFenceWithDiagram(t *testing.T) {
	md := "# Title\n\nbefore\n\n" + flowchart + "\n\nafter\n"
	got := renderMermaidBlocks(md, 80)
	if strings.Contains(got, "```mermaid") {
		t.Errorf("the mermaid fence should be replaced:\n%s", got)
	}
	if !strings.Contains(got, "```text\n") || !strings.Contains(got, "┌") || !strings.Contains(got, "Start") {
		t.Errorf("expected a text fence holding the drawn diagram:\n%s", got)
	}
	if !strings.HasPrefix(got, "# Title\n\nbefore\n\n") || !strings.HasSuffix(got, "\n\nafter\n") {
		t.Errorf("the surrounding text must be untouched:\n%s", got)
	}
}

func TestRenderMermaidBlocksLeavesADiagramItCannotDraw(t *testing.T) {
	md := "```mermaid\nthis is not a diagram\n```\n"
	if got := renderMermaidBlocks(md, 80); got != md {
		t.Errorf("an undrawable block should stay as it is:\n%s", got)
	}
}

func TestRenderMermaidBlocksLeavesADiagramWiderThanThePane(t *testing.T) {
	md := flowchart + "\n"
	if got := renderMermaidBlocks(md, 8); got != md {
		t.Errorf("a diagram wider than the pane should stay as source:\n%s", got)
	}
}

func TestRenderMermaidBlocksHandlesSeveralFences(t *testing.T) {
	md := flowchart + "\n\n```go\nfmt.Println()\n```\n\n" + flowchart + "\n\n```mermaid\nflowchart TD\n    X --> Y\n"
	got := renderMermaidBlocks(md, 80)
	if n := strings.Count(got, "```text\n"); n != 2 {
		t.Errorf("both closed mermaid fences should be drawn, got %d text fences:\n%s", n, got)
	}
	if !strings.Contains(got, "```go\nfmt.Println()\n```") {
		t.Errorf("other fences must be untouched:\n%s", got)
	}
	if !strings.HasSuffix(got, "```mermaid\nflowchart TD\n    X --> Y\n") {
		t.Errorf("an unclosed fence must be left alone:\n%s", got)
	}
}

func TestRenderMermaidBlocksAcceptsTrailingSpaceOnTheFence(t *testing.T) {
	md := "```mermaid  \nflowchart TD\n    A --> B\n```  \n"
	if got := renderMermaidBlocks(md, 80); strings.Contains(got, "```mermaid") {
		t.Errorf("a fence with trailing spaces is still a mermaid fence:\n%s", got)
	}
}

const mermaidDoc = "# Doc\n\n" + flowchart + "\n"

func TestReaderModeDrawsMermaidBlocks(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": mermaidDoc}, "a.md")
	if c := m.tabs[0].content; !strings.Contains(c, "┌") || strings.Contains(c, "flowchart TD") {
		t.Errorf("reader mode should show the drawing, not the source:\n%s", c)
	}
	m = update(t, m, keyRune('m'))
	if c := m.tabs[0].content; strings.Contains(c, "┌") || !strings.Contains(c, "flowchart TD") {
		t.Errorf("source mode shows the source as written:\n%s", c)
	}
}

func TestMermaidOffKeepsTheSource(t *testing.T) {
	cfg := config.Default()
	cfg.Theme.Style = "notty"
	cfg.General.Mermaid = false
	m := testModelWithConfig(t, cfg, map[string]string{"a.md": mermaidDoc}, "a.md")
	if c := m.tabs[0].content; strings.Contains(c, "┌") || !strings.Contains(c, "flowchart TD") {
		t.Errorf("with mermaid off the block stays a code block:\n%s", c)
	}
	next := m.cfg
	next.General.Mermaid = true
	m.applyConfig(next)
	if c := m.tabs[0].content; !strings.Contains(c, "┌") {
		t.Errorf("turning mermaid on re-renders the open tab:\n%s", c)
	}
}

// drawnWidth is the widest line of the test flowchart once drawn.
func drawnWidth(t *testing.T) int {
	t.Helper()
	drawn, ok := drawMermaid("flowchart TD\n    A[Start] --> B[End]", 1000)
	if !ok {
		t.Fatal("the test flowchart should draw")
	}
	w := 0
	for _, l := range strings.Split(drawn, "\n") {
		if n := lipgloss.Width(l); n > w {
			w = n
		}
	}
	return w
}

func TestRenderMermaidBlocksFitBoundMatchesGlamour(t *testing.T) {
	// Glamour indents the document by 2 on each side and the code block
	// by 2 more, so a drawing needs width-6 columns to survive wrapping.
	d := drawnWidth(t)
	if got := renderMermaidBlocks(flowchart, d+6); !strings.Contains(got, "```text") {
		t.Errorf("a drawing of %d columns fits a wrap width of %d:\n%s", d, d+6, got)
	}
	if got := renderMermaidBlocks(flowchart, d+5); got != flowchart {
		t.Errorf("a drawing of %d columns does not fit a wrap width of %d and must stay source:\n%s", d, d+5, got)
	}
}

func TestReaderModeNeverWrapsADrawnDiagram(t *testing.T) {
	// A window exactly as wide as the drawing needs — the pane's two
	// border columns, glamour's wrap width two inside that, and the six
	// columns of margin and indent — so the boxes must come through
	// glamour intact, one drawn line per rendered line.
	d := drawnWidth(t)
	m := testModel(t, map[string]string{"a.md": mermaidDoc}, "a.md")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlB}) // hide the sidebar: content = full width
	m = update(t, m, tea.WindowSizeMsg{Width: d + 10, Height: 40})
	c := m.tabs[0].content
	if !strings.Contains(c, "┌") {
		t.Fatalf("the diagram should be drawn at width %d:\n%s", d+10, c)
	}
	drawn, _ := drawMermaid("flowchart TD\n    A[Start] --> B[End]", 1000)
	want := strings.Count(strings.TrimRight(drawn, "\n"), "\n") + 1
	got := 0
	for _, l := range strings.Split(c, "\n") {
		if strings.ContainsAny(l, "┌│└▼┐┘") {
			got++
		}
	}
	if got != want {
		t.Errorf("drawn lines in the rendered content = %d, want %d (a wrapped diagram adds lines):\n%s", got, want, c)
	}
	m = update(t, m, tea.WindowSizeMsg{Width: d + 9, Height: 40})
	if strings.Contains(m.tabs[0].content, "┌") {
		t.Errorf("one column narrower the drawing no longer fits and the source must show:\n%s", m.tabs[0].content)
	}
}

func TestMermaidRendererCannotWriteToTheTerminal(t *testing.T) {
	// The drawing library logs through logrus's global logger, which
	// would print straight to stderr underneath the TUI.
	if logrus.StandardLogger().Out != io.Discard {
		t.Error("logrus output should be discarded")
	}
}

func TestFlowchartWithJapaneseTextDraws(t *testing.T) {
	src := "```mermaid\nflowchart TD\n    A[開始] -->|はい| B[終了]\n```\n"
	got := renderMermaidBlocks(src, 200)
	if !strings.Contains(got, "```text") {
		t.Fatalf("a flowchart with Japanese text should draw:\n%s", got)
	}
	for _, want := range []string{"開始", "はい", "終了"} {
		if !strings.Contains(got, want) {
			t.Errorf("drawing should contain %q intact:\n%s", want, got)
		}
	}
	// Every row of the drawing is the same width: wide characters
	// were measured as two columns, not one.
	rows := strings.Split(strings.SplitN(strings.SplitN(got, "```text\n", 2)[1], "\n```", 2)[0], "\n")
	w := lipgloss.Width(rows[0])
	for i, r := range rows {
		if lipgloss.Width(r) != w {
			t.Errorf("row %d is %d wide, row 0 is %d: box drawing misaligned:\n%s", i, lipgloss.Width(r), w, got)
			break
		}
	}
}

func TestSequenceWithNonASCIITextStillDraws(t *testing.T) {
	src := "```mermaid\nsequenceDiagram\n    利用者->>mado: 起動\n    mado-->>利用者: 描画\n```\n"
	got := renderMermaidBlocks(src, 200)
	if !strings.Contains(got, "```text") || !strings.Contains(got, "利用者") || !strings.Contains(got, "起動") {
		t.Errorf("the sequence renderer handles wide text and should draw:\n%s", got)
	}
}
