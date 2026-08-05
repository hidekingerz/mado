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
