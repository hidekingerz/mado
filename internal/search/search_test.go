package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hidekingerz/mado/internal/filetree"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func rels(ms []Match) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Rel)
	}
	return out
}

func TestNamesMatchRelativePathIgnoringCase(t *testing.T) {
	root := writeTree(t, map[string]string{
		"README.md":        "",
		"docs/Plan.md":     "",
		"docs/notes.md":    "",
		"docs/sub/plan.md": "",
	})
	res := Run(context.Background(), root, Names, "plan", Options{})
	got := strings.Join(rels(res.Matches), " ")
	if got != "docs/Plan.md docs/sub/plan.md" {
		t.Errorf("names = %q", got)
	}
	// A directory segment matches too, so a query can narrow by folder.
	res = Run(context.Background(), root, Names, "sub/", Options{})
	if got := strings.Join(rels(res.Matches), " "); got != "docs/sub/plan.md" {
		t.Errorf("dir query = %q", got)
	}
}

func TestNamesSmartCase(t *testing.T) {
	root := writeTree(t, map[string]string{"Plan.md": "", "plan.md": "", "x/PLAN.md": ""})
	res := Run(context.Background(), root, Names, "Plan", Options{})
	if got := strings.Join(rels(res.Matches), " "); got != "Plan.md" {
		t.Errorf("upper-case query should be exact: %q", got)
	}
}

func TestNamesRankFileNameHitsFirst(t *testing.T) {
	root := writeTree(t, map[string]string{
		"plans/todo.md":   "",
		"a/b/plan.md":     "",
		"plan.md":         "",
		"plans/other.md":  "",
		"zz/planning.md":  "",
		"plans/x/deep.md": "",
	})
	res := Run(context.Background(), root, Names, "plan", Options{})
	got := strings.Join(rels(res.Matches), " ")
	want := "plan.md zz/planning.md a/b/plan.md plans/other.md plans/todo.md plans/x/deep.md"
	if got != want {
		t.Errorf("order = %q\nwant    %q", got, want)
	}
}

func TestEmptyQueryMatchesNothing(t *testing.T) {
	root := writeTree(t, map[string]string{"a.md": "x"})
	if res := Run(context.Background(), root, Names, "", Options{}); len(res.Matches) != 0 {
		t.Errorf("names: %v", res.Matches)
	}
	if res := Run(context.Background(), root, Contents, "", Options{}); len(res.Matches) != 0 {
		t.Errorf("contents: %v", res.Matches)
	}
}

func TestTreeFilterMatchesTheSidebar(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a.md":         "needle",
		"a.txt":        "needle",
		".hidden/b.md": "needle",
		".c.md":        "needle",
	})
	res := Run(context.Background(), root, Contents, "needle", Options{})
	if got := strings.Join(rels(res.Matches), " "); got != "a.md" {
		t.Errorf("default filter = %q, want only a.md", got)
	}
	res = Run(context.Background(), root, Contents, "needle", Options{
		Tree: filetree.Options{ShowAllFiles: true, ShowHidden: true},
	})
	if got := strings.Join(rels(res.Matches), " "); got != ".c.md .hidden/b.md a.md a.txt" {
		t.Errorf("all files = %q", got)
	}
}

func TestContentsReportLineNumbersAndText(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a.md": "one\ntwo Needle\r\nthree\nneedle again\n",
		"b.md": "nothing here\n",
	})
	res := Run(context.Background(), root, Contents, "needle", Options{})
	if len(res.Matches) != 2 {
		t.Fatalf("matches = %+v, want 2", res.Matches)
	}
	if m := res.Matches[0]; m.Rel != "a.md" || m.Line != 2 || m.Text != "two Needle" {
		t.Errorf("first = %+v", m)
	}
	if m := res.Matches[1]; m.Line != 4 || m.Text != "needle again" {
		t.Errorf("second = %+v", m)
	}
	if !filepath.IsAbs(res.Matches[0].Path) {
		t.Errorf("Path = %q, want absolute", res.Matches[0].Path)
	}
}

func TestContentsSkipBinaryFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"bin.md": "needle\x00needle",
		"ok.md":  "needle",
	})
	res := Run(context.Background(), root, Contents, "needle", Options{})
	if got := strings.Join(rels(res.Matches), " "); got != "ok.md" {
		t.Errorf("matches = %q", got)
	}
}

func TestContentsStopAtMaxResults(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString("needle\n")
	}
	root := writeTree(t, map[string]string{"a.md": b.String(), "b.md": b.String()})
	res := Run(context.Background(), root, Contents, "needle", Options{MaxResults: 5})
	if len(res.Matches) != 5 || !res.Truncated {
		t.Errorf("matches = %d truncated = %v, want 5/true", len(res.Matches), res.Truncated)
	}
	res = Run(context.Background(), root, Contents, "needle", Options{})
	if len(res.Matches) != 40 || res.Truncated {
		t.Errorf("matches = %d truncated = %v, want 40/false", len(res.Matches), res.Truncated)
	}
}

func TestNamesTruncateAfterRanking(t *testing.T) {
	root := writeTree(t, map[string]string{
		"z/deep/note.md": "", "y/note.md": "", "note.md": "", "a/note.md": "",
	})
	res := Run(context.Background(), root, Names, "note", Options{MaxResults: 2})
	if got := strings.Join(rels(res.Matches), " "); got != "note.md a/note.md" || !res.Truncated {
		t.Errorf("matches = %q truncated = %v", got, res.Truncated)
	}
}

func TestExcludePatterns(t *testing.T) {
	root := writeTree(t, map[string]string{
		"README.md":                 "needle",
		"node_modules/pkg/x.md":     "needle",
		"docs/node_modules/y.md":    "needle",
		"docs/drafts/z.md":          "needle",
		"docs/keep.md":              "needle",
		"drafts/not-excluded.md":    "needle",
		"logs/today.log.md":         "needle",
		"vendor/lib/README.md":      "needle",
		"build/out.md":              "needle",
		"src/build/instructions.md": "needle",
	})
	opts := Options{Exclude: []string{"node_modules", "docs/drafts", "*.log.md", "vendor/", "src/build"}}
	res := Run(context.Background(), root, Contents, "needle", opts)
	got := strings.Join(rels(res.Matches), " ")
	want := "README.md build/out.md docs/keep.md drafts/not-excluded.md"
	if got != want {
		t.Errorf("matches = %q\nwant      %q", got, want)
	}
}

func TestExcluded(t *testing.T) {
	cases := []struct {
		rel  string
		pats []string
		want bool
	}{
		{"node_modules", []string{"node_modules"}, true},
		{"a/node_modules", []string{"node_modules"}, true},
		{"node_modules_x", []string{"node_modules"}, false},
		{"docs/drafts", []string{"docs/drafts"}, true},
		{"x/docs/drafts", []string{"docs/drafts"}, false},
		{"a.log", []string{"*.log"}, true},
		{"deep/a.log", []string{"*.log"}, true},
		{"deep/a.md", []string{"*.log"}, false},
		{"target", []string{"target/"}, true},
		{"[bad", []string{"[bad"}, true}, // malformed glob matches literally
		{"x", []string{"", "  "}, false},
	}
	for _, c := range cases {
		if got := Excluded(c.rel, c.pats); got != c.want {
			t.Errorf("Excluded(%q, %v) = %v, want %v", c.rel, c.pats, got, c.want)
		}
	}
}

func TestCancelStopsTheWalk(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 50; i++ {
		files[filepath.Join("d", strings.Repeat("x", i%7)+".md")] = "needle"
	}
	root := writeTree(t, files)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := Run(ctx, root, Contents, "needle", Options{})
	if len(res.Matches) != 0 {
		t.Errorf("a cancelled search returned %d matches", len(res.Matches))
	}
}

func TestFindSmartCaseAndUnicode(t *testing.T) {
	cases := []struct {
		s, q       string
		start, end int
	}{
		{"Hello World", "world", 6, 11},
		{"Hello World", "World", 6, 11},
		{"Hello World", "WORLD", -1, -1},
		{"hello world", "World", -1, -1},
		{"日本語のテキスト", "テキスト", len("日本語の"), len("日本語のテキスト")},
		{"xÄBCy", "äbc", 1, 1 + len("ÄBC")},
		{"abc", "", -1, -1},
		{"", "a", -1, -1},
		// The Kelvin sign folds to k but is three bytes long: the range
		// must cover the original bytes, not the query's length.
		{"a\u212Ab", "kb", 1, len("a\u212Ab")},
	}
	for _, c := range cases {
		start, end := Find(c.s, c.q)
		if start != c.start || end != c.end {
			t.Errorf("Find(%q, %q) = (%d, %d), want (%d, %d)", c.s, c.q, start, end, c.start, c.end)
		}
		if got := Index(c.s, c.q); got != c.start {
			t.Errorf("Index(%q, %q) = %d, want %d", c.s, c.q, got, c.start)
		}
	}
}
