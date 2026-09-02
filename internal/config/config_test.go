package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	def := Default()
	if cfg.Theme.Style != def.Theme.Style || cfg.Sidebar.Width != def.Sidebar.Width {
		t.Errorf("expected defaults, got %+v", cfg)
	}
}

func TestLoadMergesOverDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := `
[theme]
style = "dracula"

[sidebar]
width = 40

[keys]
quit = ["Q"]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Theme.Style != "dracula" {
		t.Errorf("style = %q, want dracula", cfg.Theme.Style)
	}
	if cfg.Sidebar.Width != 40 {
		t.Errorf("width = %d, want 40", cfg.Sidebar.Width)
	}
	if len(cfg.Keys.Quit) != 1 || cfg.Keys.Quit[0] != "Q" {
		t.Errorf("quit = %v, want [Q]", cfg.Keys.Quit)
	}
	// Untouched values keep their defaults.
	if cfg.Theme.AccentColor != Default().Theme.AccentColor {
		t.Errorf("accent = %q, want default", cfg.Theme.AccentColor)
	}
	if len(cfg.Keys.Up) == 0 {
		t.Error("up keys should keep defaults")
	}
}

func TestLoadBadTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("not [valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestSourceStyleKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[theme]\nsource_style = \"monokai\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.SourceStyle != "monokai" {
		t.Errorf("SourceStyle = %q, want monokai", cfg.Theme.SourceStyle)
	}
	if Default().Theme.SourceStyle != "" {
		t.Errorf("default SourceStyle should be empty (= automatic)")
	}
}

func TestToggleAllFilesKey(t *testing.T) {
	if got := Default().Keys.ToggleAllFiles; len(got) != 1 || got[0] != "a" {
		t.Errorf("default ToggleAllFiles = %v, want [a]", got)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[keys]\ntoggle_all_files = [\"F\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Keys.ToggleAllFiles) != 1 || cfg.Keys.ToggleAllFiles[0] != "F" {
		t.Errorf("ToggleAllFiles = %v, want [F]", cfg.Keys.ToggleAllFiles)
	}
}

func TestToggleLineNumbersKey(t *testing.T) {
	if got := Default().Keys.ToggleLineNums; len(got) != 1 || got[0] != "n" {
		t.Errorf("default ToggleLineNums = %v, want [n]", got)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[keys]\ntoggle_line_numbers = [\"N\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Keys.ToggleLineNums) != 1 || cfg.Keys.ToggleLineNums[0] != "N" {
		t.Errorf("ToggleLineNums = %v, want [N]", cfg.Keys.ToggleLineNums)
	}
}

func TestDirColorKey(t *testing.T) {
	if got := Default().Theme.DirColor; got != "#89B4FA" {
		t.Errorf("default DirColor = %q, want #89B4FA", got)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[theme]\ndir_color = \"#FF0000\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.DirColor != "#FF0000" {
		t.Errorf("DirColor = %q, want #FF0000", cfg.Theme.DirColor)
	}
}

func TestFileColorKey(t *testing.T) {
	if got := Default().Theme.FileColor; got != "#FFFFFF" {
		t.Errorf("default FileColor = %q, want #FFFFFF", got)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[theme]\nfile_color = \"#AABBCC\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.FileColor != "#AABBCC" {
		t.Errorf("FileColor = %q, want #AABBCC", cfg.Theme.FileColor)
	}
}

func TestLoadGeneralWatch(t *testing.T) {
	if Default().General.Watch {
		t.Error("watch should be off by default")
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[general]\nwatch = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.General.Watch {
		t.Error("watch = true was not picked up from the config")
	}
}

func TestSearchKeys(t *testing.T) {
	def := Default()
	if got := def.Keys.Search; len(got) != 1 || got[0] != "/" {
		t.Errorf("default Search = %v, want [/]", got)
	}
	if got := def.Keys.SearchContent; len(got) != 1 || got[0] != "ctrl+f" {
		t.Errorf("default SearchContent = %v, want [ctrl+f]", got)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[keys]\nsearch = [\"f\"]\nsearch_content = [\"F\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Keys.Search) != 1 || cfg.Keys.Search[0] != "f" {
		t.Errorf("Search = %v, want [f]", cfg.Keys.Search)
	}
	if len(cfg.Keys.SearchContent) != 1 || cfg.Keys.SearchContent[0] != "F" {
		t.Errorf("SearchContent = %v, want [F]", cfg.Keys.SearchContent)
	}
}

func TestSearchExclude(t *testing.T) {
	def := Default().Search.Exclude
	has := func(list []string, s string) bool {
		for _, x := range list {
			if x == s {
				return true
			}
		}
		return false
	}
	if !has(def, "node_modules") || !has(def, ".git") {
		t.Errorf("default exclude = %v, want node_modules and .git among them", def)
	}

	// Absent key keeps the defaults.
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[search]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !has(cfg.Search.Exclude, "node_modules") {
		t.Errorf("absent key: exclude = %v, want defaults", cfg.Search.Exclude)
	}

	// A list replaces the defaults rather than adding to them.
	if err := os.WriteFile(path, []byte("[search]\nexclude = [\"tmp\", \"*.bak\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Search.Exclude) != 2 || cfg.Search.Exclude[0] != "tmp" || cfg.Search.Exclude[1] != "*.bak" {
		t.Errorf("exclude = %v, want [tmp *.bak]", cfg.Search.Exclude)
	}

	// An explicit empty list searches everything.
	if err := os.WriteFile(path, []byte("[search]\nexclude = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Search.Exclude) != 0 {
		t.Errorf("exclude = %v, want empty", cfg.Search.Exclude)
	}
}

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
