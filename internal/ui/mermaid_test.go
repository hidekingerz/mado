package ui

import (
	"strings"
	"testing"

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
