package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEncodeValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"dracula", `"dracula"`},
		{`a"b\c`, `"a\"b\\c"`},
		{"tab\there", `"tab\there"`},
		{"ctrl+c", `"ctrl+c"`},
		{true, "true"},
		{false, "false"},
		{40, "40"},
		{[]string{"node_modules", ".git"}, `["node_modules", ".git"]`},
		{[]string{}, "[]"},
		{[]string(nil), "[]"},
	}
	for _, c := range cases {
		got, err := encodeValue(c.in)
		if err != nil {
			t.Errorf("encodeValue(%#v): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("encodeValue(%#v) = %s, want %s", c.in, got, c.want)
		}
	}
	if _, err := encodeValue(3.5); err == nil {
		t.Error("floats are not something mado writes; expected an error")
	}
}

func TestReplaceKeyKeepsCommentsAndOrder(t *testing.T) {
	src := "# mado config\n[theme]\n# the style\nstyle = \"auto\"   # or dracula\naccent_color = \"#7C6AEF\"\n\n[sidebar]\nwidth = 32\n"
	want := "# mado config\n[theme]\n# the style\nstyle = \"dracula\"   # or dracula\naccent_color = \"#7C6AEF\"\n\n[sidebar]\nwidth = 32\n"
	if got := replaceKey(src, "theme", "style", `"dracula"`); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplaceKeyOnlyTouchesTheNamedTable(t *testing.T) {
	src := "[theme]\nwidth = 1\n[sidebar]\nwidth = 32\n"
	want := "[theme]\nwidth = 1\n[sidebar]\nwidth = 40\n"
	if got := replaceKey(src, "sidebar", "width", "40"); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplaceKeyIgnoresCommentedOutKeys(t *testing.T) {
	src := "[theme]\n# source_style = \"monokai\"\nstyle = \"auto\"\n"
	want := "[theme]\n# source_style = \"monokai\"\nstyle = \"auto\"\nsource_style = \"github\"\n"
	if got := replaceKey(src, "theme", "source_style", `"github"`); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplaceKeyValueContainingHash(t *testing.T) {
	src := "[theme]\naccent_color = \"#7C6AEF\"  # focused\n"
	want := "[theme]\naccent_color = \"#FF0000\"  # focused\n"
	if got := replaceKey(src, "theme", "accent_color", `"#FF0000"`); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplaceKeyCollapsesMultiLineArray(t *testing.T) {
	src := "[search]\nexclude = [\n  \"node_modules\", # deps\n  \"[weird]\",\n]  # end\n\n[keys]\nquit = [\"q\"]\n"
	want := "[search]\nexclude = [\"dist\"]  # end\n\n[keys]\nquit = [\"q\"]\n"
	if got := replaceKey(src, "search", "exclude", `["dist"]`); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplaceKeyAddsMissingKeyAfterTheTablesLastKey(t *testing.T) {
	src := "[theme]\nstyle = \"auto\"\n\n# sidebar settings\n[sidebar]\nwidth = 32\n"
	want := "[theme]\nstyle = \"auto\"\ndefault_mode = \"source\"\n\n# sidebar settings\n[sidebar]\nwidth = 32\n"
	if got := replaceKey(src, "theme", "default_mode", `"source"`); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplaceKeyAddsKeyToEmptyTable(t *testing.T) {
	src := "[theme]\n\n[sidebar]\nwidth = 32\n"
	want := "[theme]\nstyle = \"dark\"\n\n[sidebar]\nwidth = 32\n"
	if got := replaceKey(src, "theme", "style", `"dark"`); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplaceKeyAppendsMissingTable(t *testing.T) {
	cases := []struct{ src, want string }{
		{"", "[general]\nwatch = true\n"},
		{"[theme]\nstyle = \"auto\"\n", "[theme]\nstyle = \"auto\"\n\n[general]\nwatch = true\n"},
		{"[theme]\nstyle = \"auto\"", "[theme]\nstyle = \"auto\"\n\n[general]\nwatch = true\n"},
	}
	for _, c := range cases {
		if got := replaceKey(c.src, "general", "watch", "true"); got != c.want {
			t.Errorf("replaceKey(%q):\ngot:\n%s\nwant:\n%s", c.src, got, c.want)
		}
	}
}

func TestUpdateRewritesOneKeyInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	src := "# mine\n[theme]\nstyle = \"auto\"  # glamour\n\n[sidebar]\nwidth = 32\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Update(path, "sidebar", "width", 40); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# mine\n[theme]\nstyle = \"auto\"  # glamour\n\n[sidebar]\nwidth = 40\n"
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sidebar.Width != 40 {
		t.Errorf("Load after Update: width = %d", cfg.Sidebar.Width)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("a temporary file was left behind: %v", entries)
	}
}

func TestUpdateCreatesFileAndDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mado", "config.toml")
	if err := Update(path, "general", "watch", true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "[general]\nwatch = true\n" {
		t.Errorf("got:\n%s", got)
	}
}

func TestUpdateRefusesAResultThatWouldNotLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	// A dotted key defines the theme table implicitly; a [theme] header
	// after it is a redefinition, which TOML forbids. (Should the parser
	// turn out to accept it, use "[theme]\nstyle = \"auto\"\n[theme]\n"
	// instead: a header repeated is refused by every parser.)
	src := "theme.style = \"auto\"\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Update(path, "theme", "style", "dark")
	if err == nil || !strings.Contains(err.Error(), "would not load") {
		t.Fatalf("expected a refusal, got %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != src {
		t.Errorf("the file must be untouched, got:\n%s", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("a temporary file was left behind: %v", entries)
	}
}

func TestUpdateKeepsFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not meaningful on windows")
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[sidebar]\nwidth = 32\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Update(path, "sidebar", "width", 40); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestUpdateRejectsUnencodableValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Update(path, "sidebar", "width", 1.5); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("nothing should be written for a bad value")
	}
}
