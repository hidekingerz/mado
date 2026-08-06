// Package config loads mado's TOML configuration file.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the root of mado's configuration.
type Config struct {
	Theme   Theme   `toml:"theme"`
	Sidebar Sidebar `toml:"sidebar"`
	Keys    Keys    `toml:"keys"`
}

// Theme controls markdown rendering style and UI colors.
type Theme struct {
	// Style is a glamour style: "auto", "dark", "light", "dracula",
	// "notty", "ascii", or a path to a glamour style JSON file.
	Style string `toml:"style"`
	// DefaultMode is the view mode files open in: "reader" renders
	// markdown as a clean document, "source" shows the raw syntax.
	DefaultMode string `toml:"default_mode"`
	// SourceStyle is the chroma style used to highlight source-mode
	// text and non-markdown files (e.g. "monokai", "github").
	// Empty = pick automatically from Style.
	SourceStyle string `toml:"source_style"`
	AccentColor string `toml:"accent_color"`
	BorderColor string `toml:"border_color"`
	SelectionFg string `toml:"selection_fg"`
	SelectionBg string `toml:"selection_bg"`
	StatusFg    string `toml:"status_fg"`
	StatusBg    string `toml:"status_bg"`
}

// Sidebar controls the file tree pane.
type Sidebar struct {
	Width        int  `toml:"width"`
	ShowAllFiles bool `toml:"show_all_files"`
	ShowHidden   bool `toml:"show_hidden"`
}

// Keys maps actions to one or more key names (bubbletea key syntax,
// e.g. "ctrl+w", "shift+tab", "enter", "q").
type Keys struct {
	Quit          []string `toml:"quit"`
	Up            []string `toml:"up"`
	Down          []string `toml:"down"`
	Open          []string `toml:"open"`
	Back          []string `toml:"back"`
	CloseTab      []string `toml:"close_tab"`
	NextTab       []string `toml:"next_tab"`
	PrevTab       []string `toml:"prev_tab"`
	ToggleSidebar []string `toml:"toggle_sidebar"`
	HalfPageDown  []string `toml:"half_page_down"`
	HalfPageUp    []string `toml:"half_page_up"`
	Top           []string `toml:"top"`
	Bottom        []string `toml:"bottom"`
	Reload        []string `toml:"reload"`
	ToggleMode    []string `toml:"toggle_mode"`
	Help          []string `toml:"help"`
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		Theme: Theme{
			Style:       "auto",
			DefaultMode: "reader",
			AccentColor: "#7C6AEF",
			BorderColor: "#585B70",
			SelectionFg: "#1E1E2E",
			SelectionBg: "#7C6AEF",
			StatusFg:    "#CDD6F4",
			StatusBg:    "#313244",
		},
		Sidebar: Sidebar{
			Width:        32,
			ShowAllFiles: false,
			ShowHidden:   false,
		},
		Keys: Keys{
			Quit:          []string{"q", "ctrl+c"},
			Up:            []string{"up", "k"},
			Down:          []string{"down", "j"},
			Open:          []string{"enter", "l"},
			Back:          []string{"esc", "h"},
			CloseTab:      []string{"x", "ctrl+w"},
			NextTab:       []string{"tab", "]"},
			PrevTab:       []string{"shift+tab", "["},
			ToggleSidebar: []string{"b", "ctrl+b"},
			HalfPageDown:  []string{"ctrl+d", "pgdown"},
			HalfPageUp:    []string{"ctrl+u", "pgup"},
			Top:           []string{"g", "home"},
			Bottom:        []string{"G", "end"},
			Reload:        []string{"r", "f5"},
			ToggleMode:    []string{"m"},
			Help:          []string{"?"},
		},
	}
}

// DefaultPath returns the standard location of the config file:
// $XDG_CONFIG_HOME/mado/config.toml (falling back to ~/.config).
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "mado", "config.toml")
}

// Load reads the TOML file at path and merges it over the defaults.
// A missing file is not an error; the defaults are returned as-is.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	var user Config
	if err := toml.Unmarshal(data, &user); err != nil {
		return cfg, err
	}
	merge(&cfg, user)
	return cfg, nil
}

func merge(dst *Config, src Config) {
	mergeStr(&dst.Theme.Style, src.Theme.Style)
	mergeStr(&dst.Theme.DefaultMode, src.Theme.DefaultMode)
	mergeStr(&dst.Theme.SourceStyle, src.Theme.SourceStyle)
	mergeStr(&dst.Theme.AccentColor, src.Theme.AccentColor)
	mergeStr(&dst.Theme.BorderColor, src.Theme.BorderColor)
	mergeStr(&dst.Theme.SelectionFg, src.Theme.SelectionFg)
	mergeStr(&dst.Theme.SelectionBg, src.Theme.SelectionBg)
	mergeStr(&dst.Theme.StatusFg, src.Theme.StatusFg)
	mergeStr(&dst.Theme.StatusBg, src.Theme.StatusBg)

	if src.Sidebar.Width > 0 {
		dst.Sidebar.Width = src.Sidebar.Width
	}
	dst.Sidebar.ShowAllFiles = dst.Sidebar.ShowAllFiles || src.Sidebar.ShowAllFiles
	dst.Sidebar.ShowHidden = dst.Sidebar.ShowHidden || src.Sidebar.ShowHidden

	mergeKeys(&dst.Keys.Quit, src.Keys.Quit)
	mergeKeys(&dst.Keys.Up, src.Keys.Up)
	mergeKeys(&dst.Keys.Down, src.Keys.Down)
	mergeKeys(&dst.Keys.Open, src.Keys.Open)
	mergeKeys(&dst.Keys.Back, src.Keys.Back)
	mergeKeys(&dst.Keys.CloseTab, src.Keys.CloseTab)
	mergeKeys(&dst.Keys.NextTab, src.Keys.NextTab)
	mergeKeys(&dst.Keys.PrevTab, src.Keys.PrevTab)
	mergeKeys(&dst.Keys.ToggleSidebar, src.Keys.ToggleSidebar)
	mergeKeys(&dst.Keys.HalfPageDown, src.Keys.HalfPageDown)
	mergeKeys(&dst.Keys.HalfPageUp, src.Keys.HalfPageUp)
	mergeKeys(&dst.Keys.Top, src.Keys.Top)
	mergeKeys(&dst.Keys.Bottom, src.Keys.Bottom)
	mergeKeys(&dst.Keys.Reload, src.Keys.Reload)
	mergeKeys(&dst.Keys.ToggleMode, src.Keys.ToggleMode)
	mergeKeys(&dst.Keys.Help, src.Keys.Help)
}

func mergeStr(dst *string, src string) {
	if src != "" {
		*dst = src
	}
}

func mergeKeys(dst *[]string, src []string) {
	if len(src) > 0 {
		*dst = src
	}
}
