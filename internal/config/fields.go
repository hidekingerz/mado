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
