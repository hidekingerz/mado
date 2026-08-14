package filetree

import (
	"os"
	"path/filepath"
	"testing"
)

func makeTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("README.md", "# hi")
	mustWrite("notes.txt", "plain")
	mustWrite(".hidden.md", "secret")
	mustWrite("docs/guide.md", "# guide")
	mustWrite("docs/inner/deep.markdown", "deep")
	return dir
}

func names(items []Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Node.Name)
	}
	return out
}

func TestNewFiltersMarkdownAndHidden(t *testing.T) {
	root, err := New(makeTree(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := names(Flatten(root))
	want := []string{"docs", "README.md"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestShowAllAndHidden(t *testing.T) {
	root, err := New(makeTree(t), Options{ShowAllFiles: true, ShowHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	got := names(Flatten(root))
	want := map[string]bool{"docs": true, ".hidden.md": true, "README.md": true, "notes.txt": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want keys of %v", got, want)
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected entry %q", n)
		}
	}
}

func TestToggleExpandsLazily(t *testing.T) {
	root, err := New(makeTree(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	items := Flatten(root)
	docs := items[0].Node
	if docs.Name != "docs" || docs.Expanded {
		t.Fatalf("expected collapsed docs first, got %+v", docs)
	}
	if err := docs.Toggle(Options{}); err != nil {
		t.Fatal(err)
	}
	got := names(Flatten(root))
	want := []string{"docs", "inner", "guide.md", "README.md"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("after expand got %v, want %v", got, want)
		}
	}
	// Depth of docs children is 1.
	if Flatten(root)[1].Depth != 1 {
		t.Errorf("depth = %d, want 1", Flatten(root)[1].Depth)
	}
	if err := docs.Toggle(Options{}); err != nil {
		t.Fatal(err)
	}
	if len(Flatten(root)) != 2 {
		t.Errorf("collapse failed: %v", names(Flatten(root)))
	}
}

func TestReloadPreservesExpansion(t *testing.T) {
	dir := makeTree(t)
	root, err := New(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	docs := Flatten(root)[0].Node
	if err := docs.Toggle(Options{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := root.Reload(Options{}); err != nil {
		t.Fatal(err)
	}
	got := names(Flatten(root))
	found := false
	for _, n := range got {
		if n == "new.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("new.md not picked up: %v", got)
	}
	if !Flatten(root)[0].Node.Expanded {
		t.Error("docs should stay expanded after reload")
	}
}

// Regression test for issue #9: expansion state two or more levels
// deep must survive a reload, not just the first level.
func TestReloadPreservesNestedExpansion(t *testing.T) {
	dir := makeTree(t) // contains docs/inner/deep.markdown
	root, err := New(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	docs := Flatten(root)[0].Node
	if err := docs.Toggle(Options{}); err != nil {
		t.Fatal(err)
	}
	inner := Flatten(root)[1].Node
	if inner.Name != "inner" {
		t.Fatalf("unexpected layout: %v", names(Flatten(root)))
	}
	if err := inner.Toggle(Options{}); err != nil {
		t.Fatal(err)
	}
	before := names(Flatten(root))

	if err := root.Reload(Options{}); err != nil {
		t.Fatal(err)
	}

	after := names(Flatten(root))
	if len(after) != len(before) {
		t.Fatalf("nested expansion lost: before %v, after %v", before, after)
	}
	for i := range before {
		if after[i] != before[i] {
			t.Fatalf("tree changed shape: before %v, after %v", before, after)
		}
	}
	if !Flatten(root)[1].Node.Expanded {
		t.Error("docs/inner should stay expanded after reload")
	}
}

// A directory that was collapsed before the reload must stay collapsed
// (and unloaded) afterwards.
func TestReloadKeepsCollapsedDirsLazy(t *testing.T) {
	dir := makeTree(t)
	root, err := New(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Reload(Options{}); err != nil {
		t.Fatal(err)
	}
	docs := Flatten(root)[0].Node
	if docs.Expanded {
		t.Error("collapsed docs should stay collapsed after reload")
	}
	if docs.loaded {
		t.Error("collapsed docs should stay lazy (not loaded) after reload")
	}
}

// State remembered inside a collapsed-but-previously-loaded directory
// must also survive a reload: expand docs and docs/inner, collapse
// docs, reload, re-expand docs -> inner is still expanded.
func TestReloadPreservesStateInsideCollapsedDir(t *testing.T) {
	dir := makeTree(t)
	root, err := New(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	docs := Flatten(root)[0].Node
	if err := docs.Toggle(Options{}); err != nil {
		t.Fatal(err)
	}
	inner := Flatten(root)[1].Node
	if err := inner.Toggle(Options{}); err != nil {
		t.Fatal(err)
	}
	if err := docs.Toggle(Options{}); err != nil { // collapse docs
		t.Fatal(err)
	}

	if err := root.Reload(Options{}); err != nil {
		t.Fatal(err)
	}

	docs = Flatten(root)[0].Node
	if err := docs.Toggle(Options{}); err != nil { // re-expand docs
		t.Fatal(err)
	}
	inner = Flatten(root)[1].Node
	if inner.Name != "inner" {
		t.Fatalf("unexpected layout: %v", names(Flatten(root)))
	}
	if !inner.Expanded {
		t.Error("inner should still be expanded after reload, as it would be without one")
	}
}

// A previously expanded subdirectory that can no longer be read must
// surface an error instead of silently collapsing.
func TestReloadReportsUnreadableSubdir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks do not apply to root")
	}
	dir := makeTree(t)
	root, err := New(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	docs := Flatten(root)[0].Node
	if err := docs.Toggle(Options{}); err != nil {
		t.Fatal(err)
	}
	docsPath := filepath.Join(dir, "docs")
	if err := os.Chmod(docsPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(docsPath, 0o755) })

	if err := root.Reload(Options{}); err == nil {
		t.Error("Reload should report the unreadable expanded subdirectory")
	}
}

func TestIsMarkdown(t *testing.T) {
	for path, want := range map[string]bool{
		"a.md": true, "b.MARKDOWN": true, "c.mkd": true, "d.txt": false, "e": false,
	} {
		if got := IsMarkdown(path); got != want {
			t.Errorf("IsMarkdown(%q) = %v, want %v", path, got, want)
		}
	}
}
