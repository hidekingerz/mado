package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// field finds one setting by table and key.
func field(t *testing.T, table, key string) Field {
	t.Helper()
	for _, f := range Fields() {
		if f.Table == table && f.Key == key {
			return f
		}
	}
	t.Fatalf("no field %s.%s", table, key)
	return Field{}
}

func TestFieldsCoverEveryScalarKey(t *testing.T) {
	want := []string{
		"general.watch",
		"theme.style", "theme.default_mode", "theme.source_style",
		"theme.accent_color", "theme.border_color", "theme.dir_color", "theme.file_color",
		"theme.selection_fg", "theme.selection_bg", "theme.status_fg", "theme.status_bg",
		"sidebar.width", "sidebar.show_all_files", "sidebar.show_hidden",
	}
	have := map[string]bool{}
	for _, f := range Fields() {
		have[f.Table+"."+f.Key] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("Fields() lacks %s", w)
		}
	}
}

func TestFieldsAreGroupedByTable(t *testing.T) {
	seen := map[string]bool{}
	last := ""
	for _, f := range Fields() {
		if f.Table != last {
			if seen[f.Table] {
				t.Errorf("table %s appears twice, so the panel would show two headings for it", f.Table)
			}
			seen[f.Table] = true
			last = f.Table
		}
	}
}

func TestFieldDefaultsMatchDefault(t *testing.T) {
	c := Default()
	for _, f := range Fields() {
		if got, want := f.Get(&c), f.Default(); got != want {
			t.Errorf("%s.%s: Get on Default() = %q, Default() = %q", f.Table, f.Key, got, want)
		}
	}
}

func TestBoolFieldRoundTrip(t *testing.T) {
	c := Default()
	f := field(t, "general", "watch")
	if f.Kind != KindBool {
		t.Fatalf("kind = %v, want KindBool", f.Kind)
	}
	if err := f.Set(&c, "true"); err != nil {
		t.Fatal(err)
	}
	if !c.General.Watch || f.Get(&c) != "true" {
		t.Errorf("after Set true: Watch = %v, Get = %q", c.General.Watch, f.Get(&c))
	}
	if v, ok := f.Value(&c).(bool); !ok || !v {
		t.Errorf("Value = %#v, want true", f.Value(&c))
	}
	if err := f.Set(&c, "maybe"); err == nil {
		t.Error("maybe should be rejected")
	}
	if !c.General.Watch {
		t.Error("a rejected value must not change the config")
	}
}

func TestColorFieldValidates(t *testing.T) {
	c := Default()
	f := field(t, "theme", "accent_color")
	if f.Kind != KindText {
		t.Fatalf("kind = %v, want KindText", f.Kind)
	}
	for _, ok := range []string{"#ff00aa", "#FF00AA", "0", "255", " 42 "} {
		if err := f.Set(&c, ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	if f.Get(&c) != "42" {
		t.Errorf("Get = %q, want 42 (trimmed)", f.Get(&c))
	}
	for _, bad := range []string{"", "#ff00a", "#gg0000", "256", "-1", "red"} {
		if err := f.Set(&c, bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if f.Get(&c) != "42" {
		t.Error("a rejected color must not change the config")
	}
	if v, ok := f.Value(&c).(string); !ok || v != "42" {
		t.Errorf("Value = %#v, want \"42\"", f.Value(&c))
	}
}

func TestWidthFieldValidates(t *testing.T) {
	c := Default()
	f := field(t, "sidebar", "width")
	if f.Kind != KindInt {
		t.Fatalf("kind = %v, want KindInt", f.Kind)
	}
	if err := f.Set(&c, "40"); err != nil || c.Sidebar.Width != 40 {
		t.Fatalf("Set 40: err = %v, width = %d", err, c.Sidebar.Width)
	}
	if v, ok := f.Value(&c).(int); !ok || v != 40 {
		t.Errorf("Value = %#v, want 40", f.Value(&c))
	}
	for _, bad := range []string{"15", "x", "", "4.5"} {
		if err := f.Set(&c, bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if c.Sidebar.Width != 40 {
		t.Error("a rejected width must not change the config")
	}
}

func TestModeFieldValidates(t *testing.T) {
	c := Default()
	f := field(t, "theme", "default_mode")
	if f.Kind != KindEnum || len(f.Options) != 2 || f.Options[0] != "reader" || f.Options[1] != "source" {
		t.Fatalf("kind = %v options = %v, want enum reader/source", f.Kind, f.Options)
	}
	if err := f.Set(&c, "source"); err != nil || c.Theme.DefaultMode != "source" {
		t.Fatalf("Set source: err = %v, mode = %q", err, c.Theme.DefaultMode)
	}
	if err := f.Set(&c, "raw"); err == nil {
		t.Error("raw should be rejected")
	}
	if c.Theme.DefaultMode != "source" {
		t.Error("a rejected mode must not change the config")
	}
}

func TestStyleFieldAcceptsNamesAndJSONPaths(t *testing.T) {
	c := Default()
	f := field(t, "theme", "style")
	if f.Kind != KindEnum || f.Options[len(f.Options)-1] != CustomOption {
		t.Fatalf("kind = %v options = %v, want an enum ending in %q", f.Kind, f.Options, CustomOption)
	}
	for _, name := range []string{"auto", "dark", "light", "dracula", "notty", "ascii"} {
		if err := f.Set(&c, name); err != nil {
			t.Errorf("%q rejected: %v", name, err)
		}
	}
	path := filepath.Join(t.TempDir(), "my.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.Set(&c, path); err != nil || c.Theme.Style != path {
		t.Fatalf("Set %s: err = %v, style = %q", path, err, c.Theme.Style)
	}
	for _, bad := range []string{"", CustomOption, "neon", filepath.Join(t.TempDir(), "missing.json"), "notes.txt"} {
		if err := f.Set(&c, bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if c.Theme.Style != path {
		t.Error("a rejected style must not change the config")
	}
}

func TestSourceStyleFieldAcceptsEmpty(t *testing.T) {
	c := Default()
	f := field(t, "theme", "source_style")
	if err := f.Set(&c, "monokai"); err != nil || c.Theme.SourceStyle != "monokai" {
		t.Fatalf("Set monokai: err = %v, got %q", err, c.Theme.SourceStyle)
	}
	if err := f.Set(&c, ""); err != nil || c.Theme.SourceStyle != "" {
		t.Fatalf("Set empty: err = %v, got %q", err, c.Theme.SourceStyle)
	}
}

func TestExcludeFieldSplitsOnSpaces(t *testing.T) {
	c := Default()
	f := field(t, "search", "exclude")
	if f.Kind != KindList {
		t.Fatalf("kind = %v, want KindList", f.Kind)
	}
	if err := f.Set(&c, "  node_modules  *.log docs/drafts "); err != nil {
		t.Fatal(err)
	}
	want := []string{"node_modules", "*.log", "docs/drafts"}
	if !reflect.DeepEqual(c.Search.Exclude, want) {
		t.Errorf("Exclude = %v, want %v", c.Search.Exclude, want)
	}
	if f.Get(&c) != "node_modules *.log docs/drafts" {
		t.Errorf("Get = %q", f.Get(&c))
	}
	if err := f.Set(&c, ""); err != nil {
		t.Fatal(err)
	}
	if c.Search.Exclude == nil || len(c.Search.Exclude) != 0 {
		t.Errorf("empty must be an empty list (exclude = []), got %#v", c.Search.Exclude)
	}
	if v, ok := f.Value(&c).([]string); !ok || len(v) != 0 {
		t.Errorf("Value = %#v, want an empty []string", f.Value(&c))
	}
}

func TestKeysFieldsCoverEveryAction(t *testing.T) {
	want := []string{
		"quit", "up", "down", "open", "back", "close_tab", "next_tab", "prev_tab",
		"toggle_sidebar", "half_page_down", "half_page_up", "top", "bottom", "reload",
		"toggle_mode", "toggle_all_files", "toggle_line_numbers", "search", "search_content",
		"settings", "help",
	}
	var got []string
	for _, f := range Fields() {
		if f.Kind == KindKeys {
			if f.Table != "keys" {
				t.Errorf("%s: key fields live in [keys], not [%s]", f.Key, f.Table)
			}
			got = append(got, f.Key)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("key fields = %v\nwant %v", got, want)
	}
}

func TestKeysFieldAddAndRemove(t *testing.T) {
	c := Default()
	f := field(t, "keys", "reload")
	if got := f.Get(&c); got != "r, f5" {
		t.Errorf("Get = %q, want \"r, f5\"", got)
	}
	if err := f.AddKey(&c, "ctrl+r"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.Keys(&c), []string{"r", "f5", "ctrl+r"}) {
		t.Errorf("Keys = %v", f.Keys(&c))
	}
	if v, ok := f.Value(&c).([]string); !ok || len(v) != 3 {
		t.Errorf("Value = %#v", f.Value(&c))
	}
	if err := f.AddKey(&c, "q"); err == nil || !strings.Contains(err.Error(), "quit") {
		t.Errorf("q belongs to quit; err = %v", err)
	}
	if err := f.AddKey(&c, "r"); err == nil {
		t.Error("a key the action already has should be refused")
	}
	if err := f.AddKey(&c, " "); err == nil {
		t.Error("space cannot be bound")
	}
	if len(f.Keys(&c)) != 3 {
		t.Errorf("refused keys must not be added: %v", f.Keys(&c))
	}
	for i := 0; i < 2; i++ {
		if err := f.RemoveLastKey(&c); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.RemoveLastKey(&c); err == nil {
		t.Error("the last key must stay")
	}
	if !reflect.DeepEqual(f.Keys(&c), []string{"r"}) {
		t.Errorf("Keys = %v, want [r]", f.Keys(&c))
	}
}

func TestKeysFieldSetParsesCommaList(t *testing.T) {
	c := Default()
	f := field(t, "keys", "help")
	if err := f.Set(&c, "?, f1"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Keys.Help, []string{"?", "f1"}) {
		t.Errorf("Help = %v", c.Keys.Help)
	}
	if err := f.Set(&c, "q"); err == nil {
		t.Error("q belongs to quit")
	}
	if err := f.Set(&c, " , "); err == nil {
		t.Error("an empty list should be refused")
	}
	if err := f.Set(&c, "?, f1"); err != nil {
		t.Errorf("re-setting an action's own keys must pass: %v", err)
	}
}

func TestAddKeyDoesNotAliasAnotherConfig(t *testing.T) {
	a := Default()
	b := a
	f := field(t, "keys", "reload")
	if err := f.AddKey(&a, "ctrl+r"); err != nil {
		t.Fatal(err)
	}
	if err := f.RemoveLastKey(&a); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.Keys.Reload, []string{"r", "f5"}) {
		t.Errorf("a copy of the config must not see the edit: %v", b.Keys.Reload)
	}
}
