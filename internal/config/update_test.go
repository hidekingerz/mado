package config

import (
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
