# Settings Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A settings panel opened with `,` that lists every config key, edits it in place, applies it to the running window at once, and rewrites just that key in `config.toml`.

**Architecture:** `internal/config` gains a declarative list of settings (`Fields`, with typed `Get`/`Set`/`Value` per field and validation) and an in-place TOML editor (`Update`) that rewrites one key and leaves comments and ordering alone. `internal/ui` gains `applyConfig`, which re-derives from a `config.Config` everything `New` derives at startup, and `settings.go`, a panel modelled on the search panel that owns the keyboard while open and dispatches edits by field kind.

**Tech Stack:** Go 1.24, Bubble Tea, lipgloss, BurntSushi/toml (already a dependency, used only to validate the rewritten file). No new dependencies.

**Spec:** `docs/superpowers/specs/2026-09-03-settings-panel-design.md`

## Global Constraints

- No new module dependencies.
- The running program must not query the terminal: `termenv.HasDarkBackground()` is called once in `New` and pinned into `Model.darkBG`, like `profile` and `formatter` already are.
- Everything shown that came from disk (the config path, a style path, a source_style name) goes through `termsafe.String` before rendering, as every other string from disk does.
- Inside the panel the keys are fixed; only `ctrl+c` quits. `q` and `/` are inert.
- A rejected value never changes the config; a failed save never undoes the on-screen change.
- Work on branch `claude/settings-panel`. Commit messages follow the repo's `feat:` / `docs:` prefixes and end with the trailer block used by earlier commits on this branch.
- Before every commit run `gofmt -l .` (must print nothing), `go vet ./...`, and `go test -race ./...`.

---

### Task 1: the `settings` key

**Files:**
- Modify: `internal/config/config.go` (Keys struct, `Default`, `merge`)
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/view.go` (`renderHelp` rows)
- Test: `internal/config/config_test.go`, `internal/ui/model_test.go`

**Interfaces:**
- Produces: `config.Keys.Settings []string` (default `[","]`), `keyMap.Settings key.Binding` (help text `"settings"`).

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/config_test.go`:

```go
func TestSettingsKey(t *testing.T) {
	if got := Default().Keys.Settings; len(got) != 1 || got[0] != "," {
		t.Errorf("default settings key = %v, want [,]", got)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[keys]\nsettings = [\"ctrl+s\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Keys.Settings) != 1 || cfg.Keys.Settings[0] != "ctrl+s" {
		t.Errorf("settings = %v, want [ctrl+s]", cfg.Keys.Settings)
	}
}
```

Append to `internal/ui/model_test.go`:

```go
func TestHelpListsSettingsKey(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune('?'))
	if v := m.View(); !strings.Contains(v, "settings") {
		t.Errorf("help should list the settings key:\n%s", v)
	}
}
```

(`keyRune` is defined in `internal/ui/search_test.go`, same package.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config -run TestSettingsKey; go test ./internal/ui -run TestHelpListsSettingsKey`
Expected: compile error `cfg.Keys.Settings undefined` in config; the ui test fails with "help should list the settings key".

- [ ] **Step 3: Add the key**

In `internal/config/config.go`, add to the `Keys` struct after `SearchContent`:

```go
	Settings       []string `toml:"settings"`
```

In `Default()`, after `SearchContent: []string{"ctrl+f"},`:

```go
			Settings:       []string{","},
```

In `merge`, after `mergeKeys(&dst.Keys.SearchContent, src.Keys.SearchContent)`:

```go
	mergeKeys(&dst.Keys.Settings, src.Keys.Settings)
```

In `internal/ui/keys.go`, add `Settings key.Binding` to `keyMap` after `SearchContent`, and in `newKeyMap` after the `SearchContent:` line:

```go
		Settings:       bind(k.Settings, "settings"),
```

In `internal/ui/view.go` `renderHelp`, add a row after the `SearchContent` row:

```go
		{k.Settings.Help().Key, k.Settings.Help().Desc},
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config ./internal/ui`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./... && go test -race ./...
git add internal/config/config.go internal/config/config_test.go internal/ui/keys.go internal/ui/view.go internal/ui/model_test.go
git commit -m "feat: add the settings key binding"
```

---

### Task 2: `config.Fields` — scalar settings

**Files:**
- Create: `internal/config/fields.go`
- Create: `internal/config/fields_test.go`

**Interfaces:**
- Produces:
  - `type Kind int` with `KindBool, KindEnum, KindText, KindInt, KindList, KindKeys`.
  - `const CustomOption = "custom…"` — the last option of the `style` enum.
  - `type Field struct { Table, Key string; Kind Kind; Desc string; Options []string; Get func(*Config) string; Set func(*Config, string) error; keys func(*Config) *[]string }`
  - `func (f Field) Default() string`, `func (f Field) Value(c *Config) any` (bool / int / string / []string).
  - `func Fields() []Field` — in panel order. This task adds `general.watch`, `theme.*` (style, default_mode, source_style, eight colors), `sidebar.*` (width, show_all_files, show_hidden). Task 3 appends `search.exclude` and `keys.*`.

- [ ] **Step 1: Write the failing tests**

Create `internal/config/fields_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config`
Expected: compile errors — `Fields`, `Field`, `KindBool` undefined.

- [ ] **Step 3: Implement the scalar fields**

Create `internal/config/fields.go`:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Kind says how a field is edited and how its value is encoded.
type Kind int

const (
	KindBool Kind = iota // true / false
	KindEnum             // one of Options
	KindText             // free text, validated by Set
	KindInt              // a whole number
	KindList             // strings separated by spaces
	KindKeys             // key names bound to an action
)

// CustomOption is the last option of an enum that also accepts a value
// of the user's own; choosing it asks for that value as text.
const CustomOption = "custom…"

// Field is one configurable value: where it lives in the TOML file,
// how it is edited, and how to read and write it on a Config. Get and
// Set speak the string the user sees; Set validates and leaves the
// config alone when it rejects a value.
type Field struct {
	Table, Key string
	Kind       Kind
	Desc       string
	Options    []string // KindEnum: the choices, in order
	Get        func(*Config) string
	Set        func(*Config, string) error
	keys       func(*Config) *[]string // KindKeys only
}

// Default is the value the field has in Default(), as Get shows it.
func (f Field) Default() string {
	c := Default()
	return f.Get(&c)
}

// Value returns the field's value on c as the TOML writer wants it:
// bool, int, string or []string.
func (f Field) Value(c *Config) any {
	switch f.Kind {
	case KindBool:
		return f.Get(c) == "true"
	case KindInt:
		n, _ := strconv.Atoi(f.Get(c))
		return n
	case KindList:
		return strings.Fields(f.Get(c))
	case KindKeys:
		return append([]string(nil), (*f.keys(c))...)
	default:
		return f.Get(c)
	}
}

// Fields lists every setting in the order the settings panel shows
// them, grouped by table.
func Fields() []Field {
	fs := []Field{
		boolField("general", "watch", "Reload open files and the tree when they change on disk",
			func(c *Config) *bool { return &c.General.Watch }),
		{
			Table: "theme", Key: "style", Kind: KindEnum,
			Desc:    "Glamour markdown style, or a path to a style JSON file",
			Options: []string{"auto", "dark", "light", "dracula", "notty", "ascii", CustomOption},
			Get:     func(c *Config) string { return c.Theme.Style },
			Set:     func(c *Config, v string) error { return setStyle(&c.Theme.Style, v) },
		},
		{
			Table: "theme", Key: "default_mode", Kind: KindEnum,
			Desc:    "View mode files open in",
			Options: []string{"reader", "source"},
			Get:     func(c *Config) string { return c.Theme.DefaultMode },
			Set: func(c *Config, v string) error {
				v = strings.TrimSpace(v)
				if v != "reader" && v != "source" {
					return fmt.Errorf("default_mode must be reader or source, not %q", v)
				}
				c.Theme.DefaultMode = v
				return nil
			},
		},
		{
			Table: "theme", Key: "source_style", Kind: KindText,
			Desc: "Chroma style for source mode; empty picks one from style",
			Get:  func(c *Config) string { return c.Theme.SourceStyle },
			Set: func(c *Config, v string) error {
				c.Theme.SourceStyle = strings.TrimSpace(v)
				return nil
			},
		},
		colorField("accent_color", "Focused borders, active tab, selection", func(c *Config) *string { return &c.Theme.AccentColor }),
		colorField("border_color", "Unfocused pane borders", func(c *Config) *string { return &c.Theme.BorderColor }),
		colorField("dir_color", "Directory names in the sidebar", func(c *Config) *string { return &c.Theme.DirColor }),
		colorField("file_color", "Text files in the sidebar", func(c *Config) *string { return &c.Theme.FileColor }),
		colorField("selection_fg", "Selected row text", func(c *Config) *string { return &c.Theme.SelectionFg }),
		colorField("selection_bg", "Selected row background", func(c *Config) *string { return &c.Theme.SelectionBg }),
		colorField("status_fg", "Status bar text", func(c *Config) *string { return &c.Theme.StatusFg }),
		colorField("status_bg", "Status bar background", func(c *Config) *string { return &c.Theme.StatusBg }),
		{
			Table: "sidebar", Key: "width", Kind: KindInt,
			Desc: "Sidebar width in columns, at least 16",
			Get:  func(c *Config) string { return strconv.Itoa(c.Sidebar.Width) },
			Set: func(c *Config, v string) error {
				n, err := strconv.Atoi(strings.TrimSpace(v))
				if err != nil || n < 16 {
					return fmt.Errorf("width must be a whole number of at least 16, not %q", v)
				}
				c.Sidebar.Width = n
				return nil
			},
		},
		boolField("sidebar", "show_all_files", "List every file, not only markdown",
			func(c *Config) *bool { return &c.Sidebar.ShowAllFiles }),
		boolField("sidebar", "show_hidden", "List dotfiles and dot-directories",
			func(c *Config) *bool { return &c.Sidebar.ShowHidden }),
	}
	return fs
}

func boolField(table, key, desc string, ptr func(*Config) *bool) Field {
	return Field{
		Table: table, Key: key, Kind: KindBool, Desc: desc,
		Get: func(c *Config) string { return strconv.FormatBool(*ptr(c)) },
		Set: func(c *Config, v string) error {
			b, err := strconv.ParseBool(strings.TrimSpace(v))
			if err != nil {
				return fmt.Errorf("%s must be true or false, not %q", key, v)
			}
			*ptr(c) = b
			return nil
		},
	}
}

func colorField(key, desc string, ptr func(*Config) *string) Field {
	return Field{
		Table: "theme", Key: key, Kind: KindText, Desc: desc + " (#RRGGBB or 0-255)",
		Get: func(c *Config) string { return *ptr(c) },
		Set: func(c *Config, v string) error {
			v = strings.TrimSpace(v)
			if !validColor(v) {
				return fmt.Errorf("%s must be #RRGGBB or an ANSI index 0-255, not %q", key, v)
			}
			*ptr(c) = v
			return nil
		},
	}
}

// validColor accepts what lipgloss.Color understands here: a #RRGGBB
// hex triplet or an ANSI 256-color index.
func validColor(v string) bool {
	if strings.HasPrefix(v, "#") {
		if len(v) != 7 {
			return false
		}
		_, err := strconv.ParseUint(v[1:], 16, 32)
		return err == nil
	}
	n, err := strconv.Atoi(v)
	return err == nil && n >= 0 && n <= 255
}

// setStyle accepts a built-in glamour style name or the path of a
// style JSON file that exists. The renderer treats anything ending in
// .json as a path, so that is the shape a custom style must have.
func setStyle(dst *string, v string) error {
	v = strings.TrimSpace(v)
	switch v {
	case "auto", "dark", "light", "dracula", "notty", "ascii":
		*dst = v
		return nil
	case "", CustomOption:
		return fmt.Errorf("style must be a built-in name or a path to a style JSON file")
	}
	if !strings.HasSuffix(v, ".json") {
		return fmt.Errorf("style %q is not a built-in name; a custom style is a path ending in .json", v)
	}
	if _, err := os.Stat(v); err != nil {
		return fmt.Errorf("style file %s: %w", v, err)
	}
	*dst = v
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./... && go test -race ./...
git add internal/config/fields.go internal/config/fields_test.go
git commit -m "feat: describe the scalar settings as fields"
```

---

### Task 3: `config.Fields` — the exclude list and the key bindings

**Files:**
- Modify: `internal/config/fields.go`
- Test: `internal/config/fields_test.go`

**Interfaces:**
- Consumes: `Field`, `Fields`, `KindList`, `KindKeys` from Task 2.
- Produces:
  - `Fields()` now also returns `search.exclude` (KindList) and one KindKeys field per action in `[keys]`, file order, `settings` before `help`.
  - `func (f Field) Keys(c *Config) []string`
  - `func (f Field) AddKey(c *Config, k string) error` — refuses a key bound to any action (naming it) and a space.
  - `func (f Field) RemoveLastKey(c *Config) error` — refuses to empty the list.

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/fields_test.go` (add `"reflect"` and `"strings"` to the imports):

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config`
Expected: compile errors — `f.AddKey`, `f.Keys`, `f.RemoveLastKey` undefined.

- [ ] **Step 3: Implement the list and key fields**

In `internal/config/fields.go`, replace the `return fs` at the end of `Fields()` with:

```go
	fs = append(fs, Field{
		Table: "search", Key: "exclude", Kind: KindList,
		Desc: "Patterns the search skips, separated by spaces; empty searches everything",
		Get:  func(c *Config) string { return strings.Join(c.Search.Exclude, " ") },
		Set: func(c *Config, v string) error {
			list := strings.Fields(v)
			if list == nil {
				list = []string{} // an explicit "exclude = []", not an absent key
			}
			c.Search.Exclude = list
			return nil
		},
	})
	for _, a := range keyActions() {
		fs = append(fs, keysField(a.key, a.desc, a.ptr))
	}
	return fs
```

Then append to the file:

```go
type keyAction struct {
	key, desc string
	ptr       func(*Config) *[]string
}

// keyActions lists the [keys] table in the order config.example.toml
// has it.
func keyActions() []keyAction {
	return []keyAction{
		{"quit", "Quit", func(c *Config) *[]string { return &c.Keys.Quit }},
		{"up", "Move up / scroll up", func(c *Config) *[]string { return &c.Keys.Up }},
		{"down", "Move down / scroll down", func(c *Config) *[]string { return &c.Keys.Down }},
		{"open", "Open file / expand directory", func(c *Config) *[]string { return &c.Keys.Open }},
		{"back", "Focus the sidebar", func(c *Config) *[]string { return &c.Keys.Back }},
		{"close_tab", "Close tab", func(c *Config) *[]string { return &c.Keys.CloseTab }},
		{"next_tab", "Next tab", func(c *Config) *[]string { return &c.Keys.NextTab }},
		{"prev_tab", "Previous tab", func(c *Config) *[]string { return &c.Keys.PrevTab }},
		{"toggle_sidebar", "Show / hide the sidebar", func(c *Config) *[]string { return &c.Keys.ToggleSidebar }},
		{"half_page_down", "Half page down", func(c *Config) *[]string { return &c.Keys.HalfPageDown }},
		{"half_page_up", "Half page up", func(c *Config) *[]string { return &c.Keys.HalfPageUp }},
		{"top", "Go to top", func(c *Config) *[]string { return &c.Keys.Top }},
		{"bottom", "Go to bottom", func(c *Config) *[]string { return &c.Keys.Bottom }},
		{"reload", "Reload tree and file", func(c *Config) *[]string { return &c.Keys.Reload }},
		{"toggle_mode", "Reader / source mode", func(c *Config) *[]string { return &c.Keys.ToggleMode }},
		{"toggle_all_files", "All files / markdown only", func(c *Config) *[]string { return &c.Keys.ToggleAllFiles }},
		{"toggle_line_numbers", "Line numbers in source view", func(c *Config) *[]string { return &c.Keys.ToggleLineNums }},
		{"search", "Search file names", func(c *Config) *[]string { return &c.Keys.Search }},
		{"search_content", "Search file contents", func(c *Config) *[]string { return &c.Keys.SearchContent }},
		{"settings", "Open this settings panel", func(c *Config) *[]string { return &c.Keys.Settings }},
		{"help", "Help", func(c *Config) *[]string { return &c.Keys.Help }},
	}
}

func keysField(key, desc string, ptr func(*Config) *[]string) Field {
	return Field{
		Table: "keys", Key: key, Kind: KindKeys, Desc: desc,
		Get: func(c *Config) string { return strings.Join(*ptr(c), ", ") },
		Set: func(c *Config, v string) error {
			var keys []string
			for _, k := range strings.Split(v, ",") {
				if k = strings.TrimSpace(k); k != "" {
					keys = append(keys, k)
				}
			}
			if len(keys) == 0 {
				return fmt.Errorf("%s needs at least one key", key)
			}
			for _, k := range keys {
				if owner := keyOwner(c, k); owner != "" && owner != key {
					return fmt.Errorf("%s is already bound to %s", k, owner)
				}
			}
			*ptr(c) = keys
			return nil
		},
		keys: ptr,
	}
}

// Keys returns the action's key list. Nil for fields that are not
// key bindings.
func (f Field) Keys(c *Config) []string {
	if f.keys == nil {
		return nil
	}
	return *f.keys(c)
}

// AddKey binds one more key to the action. A key that is already
// bound — to this action or another — is refused, naming the owner;
// so is a space, which cannot be typed as a key name.
func (f Field) AddKey(c *Config, k string) error {
	if f.keys == nil {
		return fmt.Errorf("%s is not a key binding", f.Key)
	}
	if strings.TrimSpace(k) == "" {
		return fmt.Errorf("space cannot be bound")
	}
	if owner := keyOwner(c, k); owner != "" {
		return fmt.Errorf("%s is already bound to %s", k, owner)
	}
	p := f.keys(c)
	*p = append(append([]string(nil), (*p)...), k)
	return nil
}

// RemoveLastKey unbinds the action's most recently added key. Every
// action keeps at least one.
func (f Field) RemoveLastKey(c *Config) error {
	if f.keys == nil {
		return fmt.Errorf("%s is not a key binding", f.Key)
	}
	p := f.keys(c)
	if len(*p) <= 1 {
		return fmt.Errorf("%s needs at least one key", f.Key)
	}
	*p = append([]string(nil), (*p)[:len(*p)-1]...)
	return nil
}

// keyOwner names the action k is bound to, or "" when k is free.
func keyOwner(c *Config, k string) string {
	for _, a := range keyActions() {
		for _, bound := range *a.ptr(c) {
			if bound == k {
				return a.key
			}
		}
	}
	return ""
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config`
Expected: PASS (including `TestFieldDefaultsMatchDefault` over the new fields).

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./... && go test -race ./...
git add internal/config/fields.go internal/config/fields_test.go
git commit -m "feat: describe the exclude list and key bindings as fields"
```

---

### Task 4: in-place TOML editing (`replaceKey`, `encodeValue`)

**Files:**
- Create: `internal/config/update.go`
- Create: `internal/config/update_test.go`

**Interfaces:**
- Produces (used by Task 5):
  - `func encodeValue(v any) (string, error)` — string, bool, int, []string to TOML text.
  - `func replaceKey(src, table, key, encoded string) string` — pure; rewrites one key, keeps everything else.

- [ ] **Step 1: Write the failing tests**

Create `internal/config/update_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config`
Expected: compile errors — `encodeValue`, `replaceKey` undefined.

- [ ] **Step 3: Implement the editor**

Create `internal/config/update.go`:

```go
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// encodeValue renders v as a TOML value: a basic string, a boolean,
// an integer, or a one-line array of strings.
func encodeValue(v any) (string, error) {
	switch v := v.(type) {
	case string:
		return quoteTOML(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case int:
		return strconv.Itoa(v), nil
	case []string:
		parts := make([]string, len(v))
		for i, s := range v {
			parts[i] = quoteTOML(s)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	default:
		return "", fmt.Errorf("cannot encode %T as TOML", v)
	}
}

// quoteTOML writes s as a TOML basic string. Go's strconv.Quote is
// close but uses escapes TOML does not know (\x, \a), so this spells
// out the few TOML has.
func quoteTOML(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// replaceKey returns src with table.key set to encoded and nothing
// else touched: comments, blank lines, ordering and every other key
// stay as they were. A key missing from its table is added after the
// table's last key; a table missing from the file is appended.
func replaceKey(src, table, key, encoded string) string {
	lines := strings.Split(src, "\n")
	inTable, tableSeen := false, false
	insertAt := -1 // a missing key goes after this line
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if name, ok := tableHeader(t); ok {
			if inTable {
				break // the target table ended
			}
			inTable = name == table
			if inTable {
				tableSeen = true
				insertAt = i
			}
			continue
		}
		if !inTable || t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		name, eq := keyName(lines[i])
		if eq < 0 {
			continue
		}
		end := valueEnd(lines, i, eq+1)
		if name == key {
			lines[i] = lines[i][:eq+1] + " " + encoded + trailingComment(lines[end])
			lines = append(lines[:i+1], lines[end+1:]...)
			return strings.Join(lines, "\n")
		}
		insertAt = end
		i = end
	}
	if !tableSeen {
		out := src
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if out != "" {
			out += "\n"
		}
		return out + "[" + table + "]\n" + key + " = " + encoded + "\n"
	}
	line := key + " = " + encoded
	lines = append(lines[:insertAt+1], append([]string{line}, lines[insertAt+1:]...)...)
	return strings.Join(lines, "\n")
}

// tableHeader reports the name on a "[table]" line. Arrays of tables
// ("[[x]]") are not something mado's config has and are left alone.
func tableHeader(t string) (string, bool) {
	if !strings.HasPrefix(t, "[") || strings.HasPrefix(t, "[[") {
		return "", false
	}
	end := strings.Index(t, "]")
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(t[1:end]), true
}

// keyName returns the bare key at the start of line and the index of
// its "=", or -1 when the line is not a plain "key = value". Quoted
// and dotted keys are not mado's; they are left for the parser to
// judge after the edit.
func keyName(line string) (string, int) {
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", -1
	}
	name := strings.TrimSpace(line[:eq])
	if name == "" || strings.ContainsAny(name, "\"' .") {
		return "", -1
	}
	return name, eq
}

// valueEnd returns the index of the line on which the value starting
// at lines[i][from:] ends: the same line unless an array is left
// open, in which case brackets outside quoted strings are counted
// until it closes.
func valueEnd(lines []string, i, from int) int {
	depth := bracketDepth(lines[i][from:], 0)
	for depth > 0 && i+1 < len(lines) {
		i++
		depth = bracketDepth(lines[i], depth)
	}
	return i
}

// outsideStrings calls fn for every rune of line that is outside a
// quoted string, stopping at a comment; it returns the index where
// the comment starts, or len(line) when there is none.
func outsideStrings(line string, fn func(r rune)) int {
	var quote rune
	escaped := false
	for i, r := range line {
		if quote != 0 {
			switch {
			case escaped:
				escaped = false
			case r == '\\' && quote == '"':
				escaped = true
			case r == quote:
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'':
			quote = r
		case '#':
			return i
		default:
			fn(r)
		}
	}
	return len(line)
}

// bracketDepth adds the brackets on line that are outside strings and
// comments to depth.
func bracketDepth(line string, depth int) int {
	outsideStrings(line, func(r rune) {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
		}
	})
	return depth
}

// trailingComment returns the comment at the end of line together
// with the whitespace before it, or "" when there is none.
func trailingComment(line string) string {
	i := outsideStrings(line, func(rune) {})
	if i == len(line) {
		return ""
	}
	start := i
	for start > 0 && (line[start-1] == ' ' || line[start-1] == '\t') {
		start--
	}
	if start == i {
		return " " + line[i:]
	}
	return line[start:]
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./... && go test -race ./...
git add internal/config/update.go internal/config/update_test.go
git commit -m "feat: rewrite one key of a TOML file in place"
```

---

### Task 5: `config.Update` — validate, write atomically

**Files:**
- Modify: `internal/config/update.go`
- Test: `internal/config/update_test.go`

**Interfaces:**
- Consumes: `encodeValue`, `replaceKey` from Task 4.
- Produces: `func Update(path, table, key string, value any) error` — used by the UI (Task 8).

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/update_test.go` (add `"os"`, `"path/filepath"`, `"runtime"`, `"strings"` to the imports):

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config`
Expected: compile error — `Update` undefined.

- [ ] **Step 3: Implement `Update`**

In `internal/config/update.go`, add `"os"`, `"path/filepath"` and `"github.com/BurntSushi/toml"` to the imports, and append:

```go
// Update sets table.key to value in the TOML file at path, leaving
// the rest of the file as it was. The result is parsed before it is
// written, so a file that would no longer load is refused rather than
// saved; the write goes through a temporary file in the same
// directory and a rename, so an interruption leaves the old file
// whole. A missing file, and its directory, are created.
func Update(path, table, key string, value any) error {
	encoded, err := encodeValue(value)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	out := replaceKey(string(src), table, key, encoded)
	var check Config
	if err := toml.Unmarshal([]byte(out), &check); err != nil {
		return fmt.Errorf("refusing to write %s: the result would not load: %w", path, err)
	}
	return writeAtomic(path, []byte(out))
}

// writeAtomic replaces the file at path with data by writing a
// sibling temporary file and renaming it over, keeping the mode of
// the file it replaces.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // gone already once renamed
	if info, err := os.Stat(path); err == nil {
		_ = tmp.Chmod(info.Mode().Perm())
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./... && go test -race ./...
git add internal/config/update.go internal/config/update_test.go
git commit -m "feat: save one config key, validated and written atomically"
```

---

### Task 6: `applyConfig` — put a config on screen at runtime

**Files:**
- Create: `internal/ui/apply.go`
- Create: `internal/ui/apply_test.go`
- Modify: `internal/ui/model.go` (`Model` fields, `New`, `resolveStyle`)

**Interfaces:**
- Consumes: `config.Config`; existing `newKeyMap`, `chromaStyleName`, `reloadTree`, `layoutTabs`, `ensureRendered`, `syncWatch`, `waitForChange`.
- Produces:
  - `Model.darkBG bool`, `Model.configPath string`.
  - `func (m Model) WithConfigPath(path string) Model`.
  - `func newStyles(th config.Theme) styles`.
  - `func (m *Model) applyConfig(next config.Config) tea.Cmd` — the command is non-nil only when a watcher was just started.
  - `resolveStyle(style string, darkBG bool) string` (signature change; the only caller is `New`).

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/apply_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestApplyConfigThemeRebuildsStylesAndRerenders(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	next := m.cfg
	next.Theme.AccentColor = "#FF0000"
	if cmd := m.applyConfig(next); cmd != nil {
		t.Error("a theme change has nothing to wait on")
	}
	if m.cfg.Theme.AccentColor != "#FF0000" {
		t.Errorf("cfg not replaced: %q", m.cfg.Theme.AccentColor)
	}
	if got := m.styles.accent.GetForeground(); got != lipgloss.Color("#FF0000") {
		t.Errorf("accent style = %v, want #FF0000", got)
	}
	if m.tabs[0].rendered != 0 {
		t.Error("background tabs should be marked dirty")
	}
	if m.tabs[1].rendered == 0 {
		t.Error("the active tab should be re-rendered at once")
	}
}

func TestApplyConfigStyleChangesTheRenderer(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	next := m.cfg
	next.Theme.Style = "ascii"
	m.applyConfig(next)
	if m.style != "ascii" || m.sourceStyle != "" {
		t.Errorf("style = %q sourceStyle = %q, want ascii and no highlighting", m.style, m.sourceStyle)
	}
	next.Theme.Style = "dracula"
	m.applyConfig(next)
	if m.style != "dracula" || m.sourceStyle != "dracula" {
		t.Errorf("style = %q sourceStyle = %q, want dracula twice", m.style, m.sourceStyle)
	}
}

func TestApplyConfigDefaultModeSwitchesTheView(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	next := m.cfg
	next.Theme.DefaultMode = "source"
	m.applyConfig(next)
	if m.mode != modeSource {
		t.Fatalf("mode = %v, want source", m.mode)
	}
	if !strings.Contains(m.tabs[0].content, "# A") {
		t.Error("source mode should show the raw heading marker")
	}
}

func TestApplyConfigKeysTakeEffectImmediately(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	next := m.cfg
	next.Keys.NextTab = []string{"ctrl+n"}
	m.applyConfig(next)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.active != 1 {
		t.Error("tab should no longer switch tabs")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.active != 0 {
		t.Error("ctrl+n should switch tabs now")
	}
}

func TestApplyConfigSidebarReloadsTheTree(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "notes.txt": "hi"})
	if strings.Contains(m.View(), "notes.txt") {
		t.Fatal("txt files are hidden by default")
	}
	next := m.cfg
	next.Sidebar.ShowAllFiles = true
	next.Sidebar.Width = 40
	m.applyConfig(next)
	if !m.treeOpts.ShowAllFiles {
		t.Error("treeOpts should follow the config")
	}
	if !strings.Contains(m.View(), "notes.txt") {
		t.Error("the tree should reload with all files")
	}
	if m.sidebarWidth() != 40 {
		t.Errorf("sidebar width = %d, want 40", m.sidebarWidth())
	}
}

func TestApplyConfigStartsAndStopsTheWatcher(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"}, "a.md")
	t.Cleanup(func() {
		if m.watcher != nil {
			m.watcher.Close()
		}
	})
	next := m.cfg
	next.General.Watch = true
	cmd := m.applyConfig(next)
	if m.watcher == nil {
		t.Fatal("watch on should start a watcher")
	}
	if cmd == nil {
		t.Error("starting the watcher should hand back a command that waits on it")
	}
	if !strings.Contains(m.View(), "[WATCH]") {
		t.Error("status bar should show watching")
	}
	next.General.Watch = false
	if cmd := m.applyConfig(next); cmd != nil {
		t.Error("stopping the watcher has nothing to wait on")
	}
	if m.watcher != nil {
		t.Error("watch off should close the watcher")
	}
	if strings.Contains(m.View(), "[WATCH]") {
		t.Error("status bar should stop showing watching")
	}
}

func TestWithConfigPath(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	if m.configPath != "" {
		t.Fatalf("configPath starts empty, got %q", m.configPath)
	}
	m = m.WithConfigPath("/tmp/x.toml")
	if m.configPath != "/tmp/x.toml" {
		t.Errorf("configPath = %q", m.configPath)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui`
Expected: compile errors — `m.applyConfig`, `m.configPath`, `WithConfigPath` undefined.

- [ ] **Step 3: Pin the background and extract the styles in `model.go`**

In the `Model` struct, after `profile termenv.Profile ...`:

```go
	darkBG       bool            // terminal background, pinned at startup like profile
	configPath   string          // where the settings panel saves; "" = nowhere
```

Replace `resolveStyle`:

```go
// resolveStyle pins "auto" to dark or light. darkBG is the terminal
// background as read once at startup, so renders inside the running
// program never query the terminal.
func resolveStyle(style string, darkBG bool) string {
	if style == "" || style == "auto" {
		if darkBG {
			return "dark"
		}
		return "light"
	}
	return style
}
```

In `New`, replace the block from `accent := lipgloss.Color(cfg.Theme.AccentColor)` through the closing `}` of the `m := Model{` literal (the line after the `styles:` literal's `},`) with:

```go
	darkBG := termenv.HasDarkBackground()
	style := resolveStyle(cfg.Theme.Style, darkBG)
	m := Model{
		cfg:         cfg,
		keys:        newKeyMap(cfg.Keys),
		root:        root,
		treeOpts:    opts,
		sidebar:     true,
		focus:       focusSidebar,
		mode:        mode,
		style:       style,
		sourceStyle: chromaStyleName(style, cfg.Theme.SourceStyle, darkBG),
		formatter:   chromaFormatterName(),
		profile:     termenv.ColorProfile(),
		darkBG:      darkBG,
		styles:      newStyles(cfg.Theme),
	}
```

- [ ] **Step 4: Create `apply.go`**

```go
package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/filetree"
	"github.com/hidekingerz/mado/internal/watch"
)

// WithConfigPath tells the model where the settings panel saves. An
// empty path keeps changes for the session only.
func (m Model) WithConfigPath(path string) Model {
	m.configPath = path
	return m
}

// newStyles derives every lipgloss style the views use from the theme.
func newStyles(th config.Theme) styles {
	accent := lipgloss.Color(th.AccentColor)
	return styles{
		accent:    lipgloss.NewStyle().Foreground(accent),
		border:    lipgloss.NewStyle().Foreground(lipgloss.Color(th.BorderColor)),
		selection: lipgloss.NewStyle().Foreground(lipgloss.Color(th.SelectionFg)).Background(lipgloss.Color(th.SelectionBg)).Bold(true),
		cursor:    lipgloss.NewStyle().Foreground(lipgloss.Color(th.FileColor)).Bold(true),
		dir:       lipgloss.NewStyle().Foreground(lipgloss.Color(th.DirColor)).Bold(true),
		file:      lipgloss.NewStyle().Foreground(lipgloss.Color(th.FileColor)),
		dimmed:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		status:    lipgloss.NewStyle().Foreground(lipgloss.Color(th.StatusFg)).Background(lipgloss.Color(th.StatusBg)),
		tabActive: lipgloss.NewStyle().Foreground(lipgloss.Color(th.SelectionFg)).Background(accent).Bold(true),
		tabInactive: lipgloss.NewStyle().Foreground(lipgloss.Color(th.StatusFg)).
			Background(lipgloss.Color(th.StatusBg)),
	}
}

// applyConfig makes next the running configuration, re-deriving what
// New derived at startup from whichever parts changed: theme fields
// rebuild the styles and re-render every tab, sidebar fields reload
// the tree and the layout, watch starts or stops the watcher, and the
// key map is always rebuilt. The command, if any, waits on a watcher
// that was just started.
func (m *Model) applyConfig(next config.Config) tea.Cmd {
	prev := m.cfg
	m.cfg = next
	m.keys = newKeyMap(next.Keys)

	if next.Theme != prev.Theme {
		m.styles = newStyles(next.Theme)
		m.style = resolveStyle(next.Theme.Style, m.darkBG)
		m.sourceStyle = chromaStyleName(m.style, next.Theme.SourceStyle, m.darkBG)
		if next.Theme.DefaultMode != prev.Theme.DefaultMode {
			m.mode = modeReader
			if next.Theme.DefaultMode == "source" {
				m.mode = modeSource
			}
		}
		for _, t := range m.tabs {
			t.rendered = 0
		}
		m.ensureRendered(m.activeTab())
	}
	if next.Sidebar != prev.Sidebar {
		m.treeOpts = filetree.Options{
			ShowAllFiles: next.Sidebar.ShowAllFiles,
			ShowHidden:   next.Sidebar.ShowHidden,
		}
		m.reloadTree()
		m.layoutTabs()
	}
	var cmd tea.Cmd
	switch {
	case next.General.Watch && m.watcher == nil:
		w, err := watch.New(watch.DefaultDebounce)
		if err != nil {
			m.statusMsg = "watch disabled: " + err.Error()
		} else {
			m.watcher = w
			m.syncWatch()
			cmd = waitForChange(w)
		}
	case !next.General.Watch && m.watcher != nil:
		m.watcher.Close()
		m.watcher = nil
	}
	return cmd
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/ui`
Expected: PASS, including every pre-existing test (the `New` refactor must change nothing).

- [ ] **Step 6: Commit**

```bash
gofmt -l . && go vet ./... && go test -race ./...
git add internal/ui/apply.go internal/ui/apply_test.go internal/ui/model.go
git commit -m "feat: apply a changed config to the running window"
```

---

### Task 7: the settings panel — open, close, move, render

**Files:**
- Create: `internal/ui/settings.go`
- Create: `internal/ui/settings_test.go`
- Modify: `internal/ui/model.go` (`Model` field, `handleKey`, `handleMouse`, `handleRemote`, `layoutTabs`)
- Modify: `internal/ui/search.go` (`closeSearch` uses the shared `restoreFocus`)
- Modify: `internal/ui/view.go` (`renderSidebar`, `renderContent`, `renderStatusBar`)

**Interfaces:**
- Consumes: `config.Fields`, `config.Field`, `Kind*` (Tasks 2–3); `keyMap.Settings` (Task 1); `configPath` (Task 6).
- Produces:
  - `Model.settings settingsState`; `settingsRow{heading string; field int}`; `settingsEdit` with `editNone, editText, editCapture`.
  - `func (m *Model) openSettings()`, `closeSettings()`, `restoreFocus(prev focusArea)`, `selectedSetting() (config.Field, bool)`, `moveSettingsCursor(delta int)`, `scrollSettings(delta int)`, `clampSettings()`, `settingsListHeight() int`.
  - `func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)` — navigation and close only; Task 8 adds the edit dispatch.
  - `func (m Model) handleSettingsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd)`.
  - `func (m Model) renderSettings(w, h int) string`, `settingsHint() string`.

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/settings_test.go`:

```go
package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// settingsModel is testModel with a config file to save into.
func settingsModel(t *testing.T, files map[string]string, open ...string) Model {
	t.Helper()
	m := testModel(t, files, open...)
	return m.WithConfigPath(filepath.Join(t.TempDir(), "config.toml"))
}

// selectedField is the key of the field under the settings cursor.
func selectedField(m Model) string {
	f, ok := m.selectedSetting()
	if !ok {
		return ""
	}
	return f.Key
}

// moveTo puts the settings cursor on the named field.
func moveTo(t *testing.T, m Model, key string) Model {
	t.Helper()
	m = update(t, m, tea.KeyMsg{Type: tea.KeyHome})
	for range m.settings.rows {
		if selectedField(m) == key {
			return m
		}
		m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	t.Fatalf("no field %q in the panel", key)
	return m
}

func esc() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyEsc} }
func enter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

func TestSettingsKeyOpensThePanel(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m = update(t, m, keyRune(','))
	if !m.settings.open {
		t.Fatal("',' should open the settings panel")
	}
	if m.settings.prevFocus != focusContent {
		t.Errorf("prevFocus = %v, want content", m.settings.prevFocus)
	}
	v := m.View()
	for _, want := range []string{"Settings", "[general]", "watch", "[theme]", "accent_color", "[sidebar]", "config.toml"} {
		if !strings.Contains(v, want) {
			t.Errorf("panel should show %q:\n%s", want, v)
		}
	}
	if !strings.Contains(m.renderStatusBar(), "[SETTINGS]") {
		t.Errorf("status bar = %q, want [SETTINGS]", m.renderStatusBar())
	}
	if selectedField(m) != "watch" {
		t.Errorf("cursor starts on the first field, got %q", selectedField(m))
	}
	if !strings.Contains(v, "default: false") {
		t.Errorf("footer should describe the selected field:\n%s", v)
	}
}

func TestSettingsEscClosesAndRestoresFocus(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, esc())
	if m.settings.open || m.focus != focusSidebar {
		t.Errorf("open = %v focus = %v, want closed and sidebar", m.settings.open, m.focus)
	}
	m = update(t, m, keyRune(','))
	m = update(t, m, keyRune(','))
	if m.settings.open {
		t.Error("the settings key should close the panel too")
	}
}

func TestSettingsCursorSkipsHeadings(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if got := selectedField(m); got != "style" {
		t.Errorf("down from watch should skip [theme] and land on style, got %q", got)
	}
	m = update(t, m, keyRune('k'))
	if got := selectedField(m); got != "watch" {
		t.Errorf("k should move back up to watch, got %q", got)
	}
	m = update(t, m, keyRune('k'))
	if got := selectedField(m); got != "watch" {
		t.Errorf("up at the top stays put, got %q", got)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnd})
	if got := selectedField(m); got != "help" {
		t.Errorf("end should reach the last field, got %q", got)
	}
	if !strings.Contains(m.View(), "[keys]") {
		t.Error("the list should scroll to keep the cursor in view")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyHome})
	if got := selectedField(m); got != "watch" {
		t.Errorf("home should reach the first field, got %q", got)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	if selectedField(m) == "watch" {
		t.Error("pgdown should move")
	}
}

func TestSettingsTypedKeysDoNotAct(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m = update(t, m, keyRune(','))
	m = update(t, m, keyRune('q'))
	if !m.settings.open {
		t.Fatal("q must not quit or close the panel")
	}
	m = update(t, m, keyRune('/'))
	if m.search.open {
		t.Error("/ must not open the search over the panel")
	}
	m = update(t, m, keyRune('x'))
	if len(m.tabs) != 1 {
		t.Error("x must not close the tab")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c should produce tea.Quit")
	}
}

func TestSettingsOpensOverHelpNotOverSearch(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	// While the search prompt has the keyboard a typed "," is query
	// text, as every typed character is.
	m = update(t, m, keyRune('/'))
	m = update(t, m, keyRune(','))
	if m.settings.open || !m.search.open || m.search.query != "," {
		t.Errorf("settings = %v search = %v query = %q, want the comma typed into the query", m.settings.open, m.search.open, m.search.query)
	}
	m = update(t, m, esc())
	m = update(t, m, keyRune('?'))
	m = update(t, m, keyRune(','))
	if m.showHelp || !m.settings.open {
		t.Errorf("help = %v settings = %v, want settings over help", m.showHelp, m.settings.open)
	}
}

func TestSettingsMouseSelectsAndScrolls(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	x := m.sidebarWidth() + 2
	// List rows start below the tab bar (1), the border (1) and the
	// header (2). Rows: [general], watch, [theme], style.
	top := tabBarHeight + 1 + settingsHeaderRows
	m = update(t, m, tea.MouseMsg{X: x, Y: top + 3, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if got := selectedField(m); got != "style" {
		t.Errorf("click on row 3 should select style, got %q", got)
	}
	m = update(t, m, tea.MouseMsg{X: x, Y: top + 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if got := selectedField(m); got != "style" {
		t.Errorf("clicking a heading changes nothing, got %q", got)
	}
	m = update(t, m, tea.MouseMsg{X: x, Y: top, Button: tea.MouseButtonWheelDown})
	if m.settings.off != wheelStep {
		t.Errorf("wheel should scroll the list, off = %d", m.settings.off)
	}
}

func TestSettingsRowsFitThePane(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = update(t, m, tea.WindowSizeMsg{Width: 44, Height: 12})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnd})
	for _, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > 44 {
			t.Errorf("line is %d wide, pane is 44: %q", w, line)
		}
	}
}

func TestSettingsRemoteOpenClosesThePanel(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A", "b.md": "# B"})
	m, srv := serveModel(t, m)
	m = update(t, m, keyRune(','))
	m, err := sendRemote(t, m, srv, "open", filepath.Join(m.root.Path, "b.md"))
	if err != nil {
		t.Fatal(err)
	}
	if m.settings.open {
		t.Error("a remote open should put the file in front, closing the panel")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui`
Expected: compile errors — `m.settings`, `selectedSetting`, `settingsHeaderRows` undefined.

- [ ] **Step 3: Create `settings.go`**

```go
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/termsafe"
)

// settingsHeaderRows is the title and the blank line above the list;
// settingsFooterRows the blank line and the description below it.
const (
	settingsHeaderRows = 2
	settingsFooterRows = 2
)

type settingsEdit int

const (
	editNone    settingsEdit = iota
	editText                 // an inline prompt holds the value
	editCapture              // the next key press is the value
)

// settingsRow is one line of the panel: a table heading or a field.
type settingsRow struct {
	heading string // "[theme]"; "" for a field row
	field   int    // index into settingsState.fields; -1 for a heading
}

// settingsState is the settings panel: every config field listed
// under its table, a cursor on one of them, and an edit in progress.
type settingsState struct {
	open      bool
	fields    []config.Field
	rows      []settingsRow
	cursor    int // index into rows; always a field row
	off       int
	editing   settingsEdit
	input     string // the text prompt's contents
	prevFocus focusArea
}

// openSettings shows the panel over whatever the content pane held,
// search and help included.
func (m *Model) openSettings() {
	if m.settings.open {
		return
	}
	m.closeSearch()
	m.showHelp = false
	s := &m.settings
	s.open = true
	s.prevFocus = m.focus
	s.editing = editNone
	s.fields = config.Fields()
	s.rows = s.rows[:0]
	table := ""
	for i, f := range s.fields {
		if f.Table != table {
			table = f.Table
			s.rows = append(s.rows, settingsRow{heading: "[" + table + "]", field: -1})
		}
		s.rows = append(s.rows, settingsRow{field: i})
	}
	s.cursor, s.off = 0, 0
	m.moveSettingsCursor(1) // off the first heading, onto the first field
	m.statusMsg = ""
}

// closeSettings hides the panel and returns the focus to where it was.
func (m *Model) closeSettings() {
	if !m.settings.open {
		return
	}
	m.settings.open = false
	m.settings.editing = editNone
	m.restoreFocus(m.settings.prevFocus)
}

// restoreFocus puts the focus back on prev after a panel closes,
// unless that pane is gone: a hidden sidebar hands to the content,
// an empty content pane hands to the sidebar.
func (m *Model) restoreFocus(prev focusArea) {
	m.focus = prev
	if m.focus == focusSidebar && !m.sidebar {
		m.focus = focusContent
	}
	if m.focus == focusContent && len(m.tabs) == 0 && m.sidebar {
		m.focus = focusSidebar
	}
}

// selectedSetting returns the field under the cursor.
func (m *Model) selectedSetting() (config.Field, bool) {
	s := m.settings
	if !s.open || s.cursor < 0 || s.cursor >= len(s.rows) || s.rows[s.cursor].field < 0 {
		return config.Field{}, false
	}
	return s.fields[s.rows[s.cursor].field], true
}

func (m *Model) settingsListHeight() int {
	return maxInt(m.contentInnerHeight()-settingsHeaderRows-settingsFooterRows, 0)
}

// moveSettingsCursor moves delta field rows, stepping over headings,
// and keeps the cursor on screen.
func (m *Model) moveSettingsCursor(delta int) {
	s := &m.settings
	step := 1
	if delta < 0 {
		step, delta = -1, -delta
	}
	for ; delta > 0; delta-- {
		next := s.cursor
		for {
			next += step
			if next < 0 || next >= len(s.rows) || s.rows[next].field >= 0 {
				break
			}
		}
		if next < 0 || next >= len(s.rows) {
			break
		}
		s.cursor = next
	}
	m.clampSettings()
}

// scrollSettings moves the list without moving the cursor.
func (m *Model) scrollSettings(delta int) {
	s := &m.settings
	s.off += delta
	if max := maxInt(len(s.rows)-m.settingsListHeight(), 0); s.off > max {
		s.off = max
	}
	if s.off < 0 {
		s.off = 0
	}
}

// clampSettings keeps the cursor in range and on screen.
func (m *Model) clampSettings() {
	s := &m.settings
	n := len(s.rows)
	if s.cursor >= n {
		s.cursor = n - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	h := m.settingsListHeight()
	if max := maxInt(n-h, 0); s.off > max {
		s.off = max
	}
	if s.off < 0 {
		s.off = 0
	}
	if h <= 0 {
		return
	}
	if s.cursor < s.off {
		s.off = s.cursor
	}
	if s.cursor >= s.off+h {
		s.off = s.cursor - h + 1
	}
}

// handleSettingsKey runs the panel's fixed keys. The keys that edit a
// field are dispatched by its kind.
func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := &m.settings
	if key.Matches(msg, m.keys.Settings) {
		m.closeSettings()
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.closeSettings()
	case tea.KeyUp, tea.KeyCtrlP:
		m.moveSettingsCursor(-1)
	case tea.KeyDown, tea.KeyCtrlN:
		m.moveSettingsCursor(1)
	case tea.KeyPgUp:
		m.moveSettingsCursor(-maxInt(m.settingsListHeight(), 1))
	case tea.KeyPgDown:
		m.moveSettingsCursor(maxInt(m.settingsListHeight(), 1))
	case tea.KeyHome:
		m.moveSettingsCursor(-len(s.rows))
	case tea.KeyEnd:
		m.moveSettingsCursor(len(s.rows))
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "k":
			m.moveSettingsCursor(-1)
		case "j":
			m.moveSettingsCursor(1)
		}
	}
	return m, nil
}

// handleSettingsMouse selects the row under a click and scrolls with
// the wheel. Clicks outside the panel close it and act as they would
// without it.
func (m Model) handleSettingsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	inSidebar := m.sidebar && msg.X < m.sidebarWidth()
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inSidebar {
			m.scrollTree(-wheelStep)
		} else {
			m.scrollSettings(-wheelStep)
		}
	case tea.MouseButtonWheelDown:
		if inSidebar {
			m.scrollTree(wheelStep)
		} else {
			m.scrollSettings(wheelStep)
		}
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		switch {
		case msg.Y < tabBarHeight:
			m.closeSettings()
			m.clickTabBar(msg.X)
		case inSidebar:
			m.closeSettings()
			m.clickSidebar(msg.X, msg.Y)
		default:
			row := msg.Y - tabBarHeight - 1 - settingsHeaderRows
			idx := m.settings.off + row
			if row >= 0 && row < m.settingsListHeight() && idx < len(m.settings.rows) && m.settings.rows[idx].field >= 0 {
				m.settings.editing = editNone
				m.settings.cursor = idx
			}
		}
	}
	return m, nil
}

// ── rendering ───────────────────────────────────────────────────────

func (m Model) renderSettings(w, h int) string {
	s := m.settings
	rows := make([]string, 0, h)

	title := m.styles.accent.Bold(true).Render("Settings")
	where := "not saved: no config path"
	if m.configPath != "" {
		// The path came from a flag or the environment: show it, do
		// not obey it.
		where = termsafe.String(m.configPath)
	}
	// A long path is cut from the left so its file name stays readable.
	where = truncateLeft(where, maxInt(w-1-lipgloss.Width(title)-2, 0))
	gap := maxInt(w-1-lipgloss.Width(title)-lipgloss.Width(where), 1)
	rows = append(rows, clip(" "+title+strings.Repeat(" ", gap)+m.styles.dimmed.Render(where), w))
	rows = append(rows, "")

	keyW := 0
	for _, f := range s.fields {
		if lw := lipgloss.Width(f.Key); lw > keyW {
			keyW = lw
		}
	}
	listH := maxInt(h-settingsHeaderRows-settingsFooterRows, 0)
	for i := 0; i < listH; i++ {
		idx := s.off + i
		if idx >= len(s.rows) {
			rows = append(rows, "")
			continue
		}
		rows = append(rows, m.renderSettingsRow(s.rows[idx], keyW, w, idx == s.cursor))
	}
	if len(rows)+settingsFooterRows <= h {
		rows = append(rows, "", clip(m.renderSettingsFooter(), w))
	}
	return strings.Join(rows, "\n")
}

// renderSettingsRow lays out one row in w columns: the key, then the
// value — in the accent color when it is not the default, in the
// selection style under the cursor, and as a prompt or a capture
// notice while that field is being edited.
func (m Model) renderSettingsRow(r settingsRow, keyW, w int, selected bool) string {
	if r.field < 0 {
		return clip(m.styles.accent.Bold(true).Render(r.heading), w)
	}
	f := m.settings.fields[r.field]
	// Style paths and chroma names are strings from the config file.
	value := termsafe.String(f.Get(&m.cfg))
	name := " " + f.Key + strings.Repeat(" ", keyW-lipgloss.Width(f.Key)) + "  "
	valueW := maxInt(w-lipgloss.Width(name), 0)
	switch {
	case selected && m.settings.editing == editText:
		text := truncateLeft(termsafe.String(m.settings.input), maxInt(valueW-1, 0))
		return name + text + m.styles.selection.Render(" ")
	case selected && m.settings.editing == editCapture:
		return m.styles.selection.Render(padLine(truncate(name+value+"  press a key…", w), w))
	case selected:
		return m.styles.selection.Render(padLine(truncate(name+value, w), w))
	case value != f.Default():
		return name + m.styles.accent.Render(truncate(value, valueW))
	default:
		return name + m.styles.file.Render(truncate(value, valueW))
	}
}

// renderSettingsFooter describes the selected field and its default.
func (m Model) renderSettingsFooter() string {
	f, ok := m.selectedSetting()
	if !ok {
		return ""
	}
	def := f.Default()
	if def == "" {
		def = "(empty)"
	}
	return m.styles.dimmed.Render(" " + f.Desc + "  ·  default: " + def)
}

// settingsHint is the status bar's right side while the panel is
// open: what the keys do for the selected field.
func (m Model) settingsHint() string {
	switch m.settings.editing {
	case editText:
		return "enter apply │ esc cancel "
	case editCapture:
		return "press a key │ ctrl+c quit "
	}
	f, ok := m.selectedSetting()
	if !ok {
		return "esc close "
	}
	switch f.Kind {
	case config.KindBool:
		return "enter toggle │ esc close "
	case config.KindEnum:
		return "←/→ change │ esc close "
	case config.KindKeys:
		return "enter capture │ backspace remove │ esc close "
	default:
		return "enter edit │ esc close "
	}
}
```

- [ ] **Step 4: Wire the panel into `model.go`**

Add to the `Model` struct, after the `search searchState` field:

```go
	// settings is the panel that edits the config in place.
	settings settingsState
```

In `handleKey`, before the `if m.search.open && ...` guard:

```go
	// The settings panel owns the keyboard while open, so a key being
	// captured can be anything — esc included. Only ctrl+c still quits.
	if m.settings.open && msg.Type != tea.KeyCtrlC {
		return m.handleSettingsKey(msg)
	}
```

In the first `switch` of `handleKey`, after the `k.SearchContent` case:

```go
	case key.Matches(msg, k.Settings):
		m.openSettings()
		return m, nil
```

In `handleMouse`, before `if m.search.open {`:

```go
	if m.settings.open {
		return m.handleSettingsMouse(msg)
	}
```

In `handleRemote`, add `m.closeSettings()` right after each `m.closeSearch()` (both the `CmdOpen` and `CmdFocus` branches).

In `layoutTabs`, after `m.clampSearch()`:

```go
	m.clampSettings()
```

In `search.go`, replace the body of `closeSearch` after `m.cancelSearch()` with `m.restoreFocus(m.search.prevFocus)` (dropping the two `if` fallbacks, which `restoreFocus` now holds).

- [ ] **Step 5: Wire the panel into `view.go`**

In `renderSidebar`, the selection condition and the border condition both gain `&& !m.settings.open`:

```go
		case idx == m.cursor && m.focus == focusSidebar && !m.search.open && !m.settings.open:
```
```go
	if m.focus == focusSidebar && !m.search.open && !m.settings.open {
```

In `renderContent`, add a first case to the `switch` and extend the border condition:

```go
	case m.settings.open:
		body = m.renderSettings(innerW, innerH)
```
```go
	if m.focus == focusContent || m.search.open || m.settings.open {
```

In `renderStatusBar`, make the settings panel the first branch of the left side:

```go
	if m.settings.open {
		left += "[SETTINGS]  "
		if m.configPath != "" {
			left += m.configPath
		} else {
			left += "not saved"
		}
	} else if m.search.open {
```

and, after the `if m.search.open { right = ... }` block:

```go
	if m.settings.open {
		right = m.settingsHint()
	}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/ui`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
gofmt -l . && go vet ./... && go test -race ./...
git add internal/ui/settings.go internal/ui/settings_test.go internal/ui/model.go internal/ui/search.go internal/ui/view.go
git commit -m "feat: settings panel lists every config key"
```

---

### Task 8: editing — toggles, choices, text, and saving

**Files:**
- Modify: `internal/ui/settings.go`
- Test: `internal/ui/settings_test.go`

**Interfaces:**
- Consumes: `applyConfig` (Task 6), `config.Update`, `Field.Value` (Tasks 2, 5), panel state (Task 7).
- Produces:
  - `func (m Model) editSetting(msg tea.KeyMsg) (tea.Model, tea.Cmd)` — bool / enum / text / int / list; Task 9 adds keys.
  - `func (m Model) handleSettingsTextKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)`.
  - `func (m *Model) commitSetting(f config.Field, change func(*config.Config) error) (tea.Cmd, bool)`.
  - `func (m *Model) saveSetting(f config.Field)`, `startSettingsText(value string, options []string)`, `cycleOption(options []string, cur string, step int) string`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/settings_test.go` (add `"os"` and `"reflect"` to the imports):

```go
// typeText types s into the settings prompt.
func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		if r == ' ' {
			m = update(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		} else {
			m = update(t, m, keyRune(r))
		}
	}
	return m
}

func savedConfig(t *testing.T, m Model) string {
	t.Helper()
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		t.Fatalf("config not saved: %v", err)
	}
	return string(data)
}

func TestSettingsToggleBoolAppliesAndSaves(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A", "notes.txt": "n"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "show_all_files")
	m = update(t, m, enter())
	if !m.cfg.Sidebar.ShowAllFiles {
		t.Fatal("enter should toggle the value on")
	}
	if !strings.Contains(m.View(), "notes.txt") {
		t.Error("the tree should reload with all files")
	}
	if got := savedConfig(t, m); got != "[sidebar]\nshow_all_files = true\n" {
		t.Errorf("saved:\n%s", got)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeySpace})
	if m.cfg.Sidebar.ShowAllFiles {
		t.Fatal("space should toggle the value off")
	}
	if got := savedConfig(t, m); got != "[sidebar]\nshow_all_files = false\n" {
		t.Errorf("saved:\n%s", got)
	}
}

func TestSettingsWatchToggleWaitsOnTheWatcher(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"}, "a.md")
	t.Cleanup(func() {
		if m.watcher != nil {
			m.watcher.Close()
		}
	})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "watch")
	var cmd tea.Cmd
	m, cmd = updateCmd(t, m, enter())
	if m.watcher == nil || cmd == nil {
		t.Fatalf("watch on: watcher = %v cmd = %v", m.watcher != nil, cmd != nil)
	}
	m = update(t, m, enter())
	if m.watcher != nil {
		t.Error("watch off should stop the watcher")
	}
}

func TestSettingsEnumCyclesWithArrows(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "default_mode")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.cfg.Theme.DefaultMode != "source" || m.mode != modeSource {
		t.Fatalf("right: default_mode = %q mode = %v", m.cfg.Theme.DefaultMode, m.mode)
	}
	if !strings.Contains(savedConfig(t, m), "default_mode = \"source\"") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.cfg.Theme.DefaultMode != "reader" {
		t.Error("right past the last option wraps to the first")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.cfg.Theme.DefaultMode != "source" {
		t.Error("left cycles backwards")
	}
	m = update(t, m, enter())
	if m.cfg.Theme.DefaultMode != "reader" {
		t.Error("enter goes forward")
	}
}

func TestSettingsStyleCustomOpensAPrompt(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "style")
	// The test config starts at notty; ascii is next, then custom….
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.cfg.Theme.Style != "ascii" {
		t.Fatalf("style = %q, want ascii", m.cfg.Theme.Style)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.settings.editing != editText || m.settings.input != "" {
		t.Fatalf("custom… should open an empty prompt: editing = %v input = %q", m.settings.editing, m.settings.input)
	}
	if m.cfg.Theme.Style != "ascii" {
		t.Error("nothing applies until a path is entered")
	}
	path := filepath.Join(t.TempDir(), "my.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = typeText(t, m, path)
	m = update(t, m, enter())
	if m.cfg.Theme.Style != path || m.settings.editing != editNone {
		t.Fatalf("style = %q editing = %v", m.cfg.Theme.Style, m.settings.editing)
	}
	if !strings.Contains(savedConfig(t, m), "style = \""+path+"\"") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.cfg.Theme.Style != "ascii" {
		t.Errorf("left from a custom path goes to the option before custom…, got %q", m.cfg.Theme.Style)
	}
}

func TestSettingsTextPromptValidatesAndKeepsInput(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "accent_color")
	m = update(t, m, enter())
	if m.settings.editing != editText || m.settings.input != "#7C6AEF" {
		t.Fatalf("the prompt should start with the current value: editing = %v input = %q", m.settings.editing, m.settings.input)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "#12")
	m = update(t, m, enter())
	if m.settings.editing != editText {
		t.Error("a rejected value keeps the prompt open")
	}
	if !strings.Contains(m.renderStatusBar(), "accent_color") {
		t.Errorf("status bar should say why: %q", m.renderStatusBar())
	}
	if m.cfg.Theme.AccentColor != "#7C6AEF" {
		t.Error("a rejected value must not apply")
	}
	m = typeText(t, m, "3456")
	m = update(t, m, enter())
	if m.cfg.Theme.AccentColor != "#123456" || m.settings.editing != editNone {
		t.Fatalf("accent = %q editing = %v", m.cfg.Theme.AccentColor, m.settings.editing)
	}
	if got := m.styles.accent.GetForeground(); got != lipgloss.Color("#123456") {
		t.Errorf("accent style = %v, want #123456", got)
	}
	if !strings.Contains(savedConfig(t, m), "accent_color = \"#123456\"") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, enter())
	m = typeText(t, m, "zzz")
	m = update(t, m, esc())
	if m.settings.editing != editNone || m.cfg.Theme.AccentColor != "#123456" {
		t.Error("esc cancels the prompt and leaves the value")
	}
}

func TestSettingsPromptEditingKeys(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "exclude")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "dist *.log")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.settings.input != "dist *.lo" {
		t.Errorf("backspace: input = %q", m.settings.input)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlW})
	if m.settings.input != "dist " {
		t.Errorf("ctrl+w: input = %q", m.settings.input)
	}
}

func TestSettingsWidthAndExclude(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "width")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "40")
	m = update(t, m, enter())
	if m.sidebarWidth() != 40 {
		t.Errorf("sidebar width = %d, want 40", m.sidebarWidth())
	}
	if !strings.Contains(savedConfig(t, m), "width = 40") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}

	m = moveTo(t, m, "exclude")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "dist *.log")
	m = update(t, m, enter())
	if !reflect.DeepEqual(m.cfg.Search.Exclude, []string{"dist", "*.log"}) {
		t.Errorf("exclude = %v", m.cfg.Search.Exclude)
	}
	if !strings.Contains(savedConfig(t, m), "exclude = [\"dist\", \"*.log\"]") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = update(t, m, enter())
	if len(m.cfg.Search.Exclude) != 0 {
		t.Errorf("an empty prompt clears the list, got %v", m.cfg.Search.Exclude)
	}
	if !strings.Contains(savedConfig(t, m), "exclude = []") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
}

func TestSettingsSaveFailureKeepsTheChange(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m.configPath = t.TempDir() // a directory: reading it as a file fails
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "show_hidden")
	m = update(t, m, enter())
	if !m.cfg.Sidebar.ShowHidden {
		t.Error("the change should stay applied")
	}
	if !strings.Contains(m.renderStatusBar(), "save failed") {
		t.Errorf("status bar = %q, want a save failure", m.renderStatusBar())
	}
}

func TestSettingsWithoutConfigPathWarns(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	if !strings.Contains(m.View(), "not saved") {
		t.Error("the panel should say changes are not saved")
	}
	m = moveTo(t, m, "show_hidden")
	m = update(t, m, enter())
	if !m.cfg.Sidebar.ShowHidden {
		t.Error("the change should still apply for the session")
	}
	if !strings.Contains(m.renderStatusBar(), "not saved") {
		t.Errorf("status bar = %q", m.renderStatusBar())
	}
}

func TestSettingsPreservesTheRestOfTheFile(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	src := "# mine\n[theme]\nstyle = \"notty\" # keep\n\n[sidebar]\nwidth = 32\n"
	if err := os.WriteFile(m.configPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "width")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
	m = typeText(t, m, "48")
	m = update(t, m, enter())
	want := "# mine\n[theme]\nstyle = \"notty\" # keep\n\n[sidebar]\nwidth = 48\n"
	if got := savedConfig(t, m); got != want {
		t.Errorf("saved:\n%s\nwant:\n%s", got, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui`
Expected: the new tests fail — `enter` on a field does nothing yet, so `TestSettingsToggleBoolAppliesAndSaves` reports "enter should toggle the value on", and the others fail in kind.

- [ ] **Step 3: Implement editing and saving**

In `internal/ui/settings.go`, add `"strconv"`, `"unicode"` and `"unicode/utf8"` to the imports.

In `handleSettingsKey`, make the edit states the first thing checked, and add the edit dispatch to the `switch`:

```go
func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := &m.settings
	if s.editing == editText {
		return m.handleSettingsTextKey(msg)
	}
	if key.Matches(msg, m.keys.Settings) {
```

and, as a new case in that `switch msg.Type`:

```go
	case tea.KeyEnter, tea.KeySpace, tea.KeyLeft, tea.KeyRight, tea.KeyBackspace:
		return m.editSetting(msg)
```

Then append:

```go
// ── editing ─────────────────────────────────────────────────────────

// editSetting starts or performs the edit msg asks for on the
// selected field, by its kind.
func (m Model) editSetting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f, ok := m.selectedSetting()
	if !ok {
		return m, nil
	}
	cur := f.Get(&m.cfg)
	switch f.Kind {
	case config.KindBool:
		if msg.Type == tea.KeyEnter || msg.Type == tea.KeySpace {
			cmd, _ := m.commitSetting(f, func(c *config.Config) error {
				return f.Set(c, strconv.FormatBool(cur != "true"))
			})
			return m, cmd
		}
	case config.KindEnum:
		var step int
		switch msg.Type {
		case tea.KeyEnter, tea.KeyRight:
			step = 1
		case tea.KeyLeft:
			step = -1
		default:
			return m, nil
		}
		next := cycleOption(f.Options, cur, step)
		if next == config.CustomOption {
			m.startSettingsText(cur, f.Options)
			return m, nil
		}
		cmd, _ := m.commitSetting(f, func(c *config.Config) error { return f.Set(c, next) })
		return m, cmd
	case config.KindText, config.KindInt, config.KindList:
		if msg.Type == tea.KeyEnter {
			m.startSettingsText(cur, nil)
		}
	}
	return m, nil
}

// cycleOption returns the option step places from cur, wrapping. A
// value that is not an option (a custom style path) counts as the
// custom slot, which is the last option.
func cycleOption(options []string, cur string, step int) string {
	i := len(options) - 1
	for j, o := range options {
		if o == cur {
			i = j
			break
		}
	}
	n := len(options)
	return options[((i+step)%n+n)%n]
}

// startSettingsText opens the inline prompt holding value. A value
// that is one of options (a built-in style, not a path) is not worth
// editing, so the prompt starts empty then.
func (m *Model) startSettingsText(value string, options []string) {
	for _, o := range options {
		if o == value {
			value = ""
			break
		}
	}
	m.settings.editing = editText
	m.settings.input = value
}

// handleSettingsTextKey edits the prompt. enter commits, esc cancels;
// the editing keys are the search prompt's.
func (m Model) handleSettingsTextKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := &m.settings
	switch msg.Type {
	case tea.KeyEsc:
		s.editing = editNone
	case tea.KeyEnter:
		f, ok := m.selectedSetting()
		if !ok {
			s.editing = editNone
			return m, nil
		}
		input := s.input
		cmd, ok := m.commitSetting(f, func(c *config.Config) error { return f.Set(c, input) })
		if ok {
			s.editing = editNone
		}
		return m, cmd
	case tea.KeyBackspace:
		if s.input != "" {
			_, size := utf8.DecodeLastRuneInString(s.input)
			s.input = s.input[:len(s.input)-size]
		}
	case tea.KeyCtrlW:
		q := strings.TrimRightFunc(s.input, unicode.IsSpace)
		if i := strings.LastIndexFunc(q, unicode.IsSpace); i >= 0 {
			q = q[:i+1]
		} else {
			q = ""
		}
		s.input = q
	case tea.KeyCtrlU:
		s.input = ""
	case tea.KeySpace:
		s.input += " "
	case tea.KeyRunes:
		if msg.Alt {
			return m, nil
		}
		for _, r := range msg.Runes {
			// A pasted newline or escape has no place in a value.
			if !unicode.IsControl(r) {
				s.input += string(r)
			}
		}
	}
	return m, nil
}

// commitSetting applies change to a copy of the config, then puts the
// result on screen and saves it. A rejected change is reported in the
// status bar and nothing else happens. The command, if any, waits on
// a watcher the change started.
func (m *Model) commitSetting(f config.Field, change func(*config.Config) error) (tea.Cmd, bool) {
	next := m.cfg
	if err := change(&next); err != nil {
		m.statusMsg = err.Error()
		return nil, false
	}
	m.statusMsg = ""
	cmd := m.applyConfig(next)
	m.saveSetting(f)
	return cmd, true
}

// saveSetting writes the field's current value to the config file.
// Failing to save does not undo the change on screen; the status bar
// says what happened.
func (m *Model) saveSetting(f config.Field) {
	if m.configPath == "" {
		m.statusMsg = "not saved: no config path"
		return
	}
	if err := config.Update(m.configPath, f.Table, f.Key, f.Value(&m.cfg)); err != nil {
		m.statusMsg = "save failed: " + err.Error()
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ui`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./... && go test -race ./...
git add internal/ui/settings.go internal/ui/settings_test.go
git commit -m "feat: edit settings in the panel, apply at once, save the key"
```

---

### Task 9: key bindings — capture and remove

**Files:**
- Modify: `internal/ui/settings.go`
- Test: `internal/ui/settings_test.go`

**Interfaces:**
- Consumes: `Field.AddKey`, `Field.RemoveLastKey` (Task 3); `commitSetting`, `editSetting` (Task 8).
- Produces: `func (m Model) handleSettingsCaptureKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)`; `editSetting` handles `KindKeys`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/settings_test.go`:

```go
func TestSettingsCaptureBindsAKey(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A", "b.md": "# B"}, "a.md", "b.md")
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "next_tab")
	m = update(t, m, enter())
	if m.settings.editing != editCapture {
		t.Fatalf("enter should start a capture, editing = %v", m.settings.editing)
	}
	if !strings.Contains(m.View(), "press a key") || !strings.Contains(m.renderStatusBar(), "press a key") {
		t.Error("the row and the status bar should ask for a key")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.settings.editing != editNone {
		t.Error("one press ends the capture")
	}
	if !reflect.DeepEqual(m.cfg.Keys.NextTab, []string{"tab", "]", "ctrl+n"}) {
		t.Errorf("next_tab = %v", m.cfg.Keys.NextTab)
	}
	if !strings.Contains(savedConfig(t, m), "next_tab = [\"tab\", \"]\", \"ctrl+n\"]") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, esc())
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.active != 0 {
		t.Error("the new key should switch tabs as soon as the panel closes")
	}
}

func TestSettingsCaptureRefusesAKeyAnotherActionHas(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "reload")
	m = update(t, m, enter())
	m = update(t, m, keyRune('q'))
	if m.settings.editing != editNone {
		t.Error("a refused key still ends the capture")
	}
	if !strings.Contains(m.renderStatusBar(), "quit") {
		t.Errorf("status bar should name the owner: %q", m.renderStatusBar())
	}
	if !reflect.DeepEqual(m.cfg.Keys.Reload, []string{"r", "f5"}) {
		t.Errorf("reload = %v, want unchanged", m.cfg.Keys.Reload)
	}
	if _, err := os.Stat(m.configPath); !os.IsNotExist(err) {
		t.Error("nothing should be saved for a refused key")
	}
}

func TestSettingsCaptureTakesEscAsAKey(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "back")
	m = update(t, m, enter())
	m = update(t, m, esc())
	if !m.settings.open {
		t.Fatal("esc during a capture is the key, not a close")
	}
	if !strings.Contains(m.renderStatusBar(), "already bound to back") {
		t.Errorf("esc reached the binding and was refused as a duplicate: %q", m.renderStatusBar())
	}
}

func TestSettingsSpaceCannotBeBound(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "help")
	m = update(t, m, enter())
	m = update(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if !strings.Contains(m.renderStatusBar(), "space") {
		t.Errorf("status bar = %q", m.renderStatusBar())
	}
	if !reflect.DeepEqual(m.cfg.Keys.Help, []string{"?"}) {
		t.Errorf("help = %v", m.cfg.Keys.Help)
	}
}

func TestSettingsBackspaceRemovesTheLastKey(t *testing.T) {
	m := settingsModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune(','))
	m = moveTo(t, m, "reload")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !reflect.DeepEqual(m.cfg.Keys.Reload, []string{"r"}) {
		t.Errorf("reload = %v, want [r]", m.cfg.Keys.Reload)
	}
	if !strings.Contains(savedConfig(t, m), "reload = [\"r\"]") {
		t.Errorf("saved:\n%s", savedConfig(t, m))
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if !reflect.DeepEqual(m.cfg.Keys.Reload, []string{"r"}) {
		t.Error("the last key stays")
	}
	if !strings.Contains(m.renderStatusBar(), "at least one key") {
		t.Errorf("status bar = %q", m.renderStatusBar())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui`
Expected: `TestSettingsCaptureBindsAKey` fails with "enter should start a capture"; the others fail in kind.

- [ ] **Step 3: Implement capture and removal**

In `handleSettingsKey`, right after the `editText` check:

```go
	if s.editing == editCapture {
		return m.handleSettingsCaptureKey(msg)
	}
```

In `editSetting`, add a case to the `switch f.Kind`:

```go
	case config.KindKeys:
		switch msg.Type {
		case tea.KeyEnter:
			m.settings.editing = editCapture
		case tea.KeyBackspace:
			cmd, _ := m.commitSetting(f, f.RemoveLastKey)
			return m, cmd
		}
```

Append:

```go
// handleSettingsCaptureKey binds the pressed key to the selected
// action. Whatever the key is, it is the value — so esc can be bound
// — except ctrl+c, which handleKey turned into quit before this.
func (m Model) handleSettingsCaptureKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.settings.editing = editNone
	f, ok := m.selectedSetting()
	if !ok {
		return m, nil
	}
	k := msg.String()
	cmd, _ := m.commitSetting(f, func(c *config.Config) error { return f.AddKey(c, k) })
	return m, cmd
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ui`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./... && go test -race ./...
git add internal/ui/settings.go internal/ui/settings_test.go
git commit -m "feat: rebind keys from the settings panel"
```

---

### Task 10: wire `main.go`, document, check by hand

**Files:**
- Modify: `main.go`
- Modify: `README.md` (Features, key table, a Settings section, Configuration)
- Modify: `config.example.toml` (`[keys] settings`)

**Interfaces:**
- Consumes: `Model.WithConfigPath` (Task 6).

- [ ] **Step 1: Pass the config path to the model**

In `main.go`, after `m, err := ui.New(cfg, rootDir, files)` and its error check:

```go
	m = m.WithConfigPath(*configPath)
```

- [ ] **Step 2: Document the panel**

In `config.example.toml`, after the `search_content` line and its comment block, add:

```toml
settings = [","]             # open the settings panel
# The panel edits every key in this file in place and saves it here;
# comments and ordering are kept. Inside it the keys are fixed: ↑/↓
# (k/j) move, enter / space / ←/→ edit, backspace removes an action's
# last key, esc closes, ctrl+c quits.
```

In `README.md`:

- Features: add after the "Configurable keyboard shortcuts" bullet:
  `- **Settings panel** — \`,\` opens every setting for editing in place; changes apply at once and are written back to the config file, one key at a time, leaving comments alone`
- Key table: add a row `| \`,\` | settings |` before the `?` row.
- After the "### Search" section, add:

```markdown
### Settings

`,` opens a settings panel over the content pane listing every key
from `config.toml` under its table. `↑`/`↓` (or `k`/`j`) move between
them, and the footer describes the selected one with its default.
Editing depends on the kind of value: `enter` or `space` flips a
switch, `←`/`→` step through a choice, `enter` opens an inline prompt
for text, numbers and lists (`enter` applies, `esc` cancels), and for
a key binding `enter` captures the next key you press — whatever it
is, `esc` included — while `backspace` removes the action's last key.
`esc` or `,` closes the panel. `ctrl+c` still quits.

A change takes effect the moment it is made: colors repaint, a new
style re-renders the open tabs, the tree reloads, the watcher starts
or stops, and a rebound key works from the next press. It is also
written to the config file at once — only that key, so a hand-written
file keeps its comments and order. A value that will not do (a color
that is not `#RRGGBB` or `0`–`255`, a key another action already has)
is refused with a note in the status bar and nothing changes.
```

- Configuration section: add a sentence after the first paragraph: "The settings panel (`,`) edits this file in place."

- [ ] **Step 3: Verify everything**

Run:

```bash
gofmt -l . && go vet ./... && go test -race ./... && go build -o mado .
```

Expected: gofmt prints nothing; vet, tests and build succeed.

- [ ] **Step 4: Check by hand in tmux**

```bash
tmux new-session -d -s mado -x 120 -y 40 "./mado -config /tmp/mado-test.toml README.md"
sleep 1; tmux send-keys -t mado ","; sleep 0.5; tmux capture-pane -p -t mado
tmux send-keys -t mado "j" "j" "j" "j" "Enter"; sleep 0.3          # accent_color prompt
tmux send-keys -t mado "C-u"; tmux send-keys -t mado "#FF0000" "Enter"; sleep 0.5; tmux capture-pane -p -t mado
tmux send-keys -t mado "Escape" "q"; cat /tmp/mado-test.toml; tmux kill-session -t mado; rm /tmp/mado-test.toml
```

Expected: the panel lists the tables; after the second capture the borders and the selected row are red; the file holds `[theme]\naccent_color = "#FF0000"`.

Also run `./mado` against your real config once, change the sidebar width, confirm the file kept its comments, then put the width back.

- [ ] **Step 5: Commit**

```bash
git add main.go README.md config.example.toml
git commit -m "docs: describe the settings panel and wire the config path"
```

---

## Self-review notes

- Spec coverage: fields (Tasks 2–3), in-place writer with validation and atomic write (Tasks 4–5), applyConfig with every listed effect (Task 6), the panel with fixed keys, mouse, status bar, help entry (Tasks 1, 7), editing by kind with validation-keeps-prompt, save failure handling, no-config-path (Task 8), key capture with duplicate refusal, esc as a key, space refused, last-key kept (Task 9), main wiring and docs (Task 10).
- One deliberate refinement of the spec: a rejected text value keeps the prompt open with what was typed, so the user can fix it instead of retyping. The config still does not change, which is what the spec requires.
