package ui

import (
	"charm.land/bubbles/v2/key"

	"github.com/hidekingerz/mado/internal/config"
)

type keyMap struct {
	Quit           key.Binding
	Up             key.Binding
	Down           key.Binding
	Open           key.Binding
	Back           key.Binding
	CloseTab       key.Binding
	NextTab        key.Binding
	PrevTab        key.Binding
	ToggleSidebar  key.Binding
	HalfPageDown   key.Binding
	HalfPageUp     key.Binding
	Top            key.Binding
	Bottom         key.Binding
	Reload         key.Binding
	ToggleMode     key.Binding
	ToggleAllFiles key.Binding
	ToggleLineNums key.Binding
	Search         key.Binding
	SearchContent  key.Binding
	Settings       key.Binding
	Help           key.Binding
}

func newKeyMap(k config.Keys) keyMap {
	bind := func(keys []string, help string) key.Binding {
		keys = keyNames(keys)
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(joinKeys(keys), help))
	}
	return keyMap{
		Quit:           bind(k.Quit, "quit"),
		Up:             bind(k.Up, "up"),
		Down:           bind(k.Down, "down"),
		Open:           bind(k.Open, "open / expand"),
		Back:           bind(k.Back, "focus sidebar"),
		CloseTab:       bind(k.CloseTab, "close tab"),
		NextTab:        bind(k.NextTab, "next tab"),
		PrevTab:        bind(k.PrevTab, "previous tab"),
		ToggleSidebar:  bind(k.ToggleSidebar, "toggle sidebar"),
		HalfPageDown:   bind(k.HalfPageDown, "half page down"),
		HalfPageUp:     bind(k.HalfPageUp, "half page up"),
		Top:            bind(k.Top, "go to top"),
		Bottom:         bind(k.Bottom, "go to bottom"),
		Reload:         bind(k.Reload, "reload"),
		ToggleMode:     bind(k.ToggleMode, "reader/source mode"),
		ToggleAllFiles: bind(k.ToggleAllFiles, "all files / markdown only"),
		ToggleLineNums: bind(k.ToggleLineNums, "line numbers"),
		Search:         bind(k.Search, "search file names"),
		SearchContent:  bind(k.SearchContent, "search file contents"),
		Settings:       bind(k.Settings, "settings"),
		Help:           bind(k.Help, "toggle help"),
	}
}

// keyNames spells keys the way Bubble Tea v2 reports them. A config
// written for an earlier mado may name the space bar " ", which is
// how v1 spelled it; left alone it would never match again.
func keyNames(keys []string) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		if k == " " {
			k = "space"
		}
		out[i] = k
	}
	return out
}

func joinKeys(keys []string) string {
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += "/"
		}
		out += k
	}
	return out
}
