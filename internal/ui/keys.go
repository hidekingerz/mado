package ui

import (
	"github.com/charmbracelet/bubbles/key"

	"github.com/hidekingerz/mado/internal/config"
)

type keyMap struct {
	Quit          key.Binding
	Up            key.Binding
	Down          key.Binding
	Open          key.Binding
	Back          key.Binding
	CloseTab      key.Binding
	NextTab       key.Binding
	PrevTab       key.Binding
	ToggleSidebar key.Binding
	HalfPageDown  key.Binding
	HalfPageUp    key.Binding
	Top           key.Binding
	Bottom        key.Binding
	Reload        key.Binding
	Help          key.Binding
}

func newKeyMap(k config.Keys) keyMap {
	bind := func(keys []string, help string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(joinKeys(keys), help))
	}
	return keyMap{
		Quit:          bind(k.Quit, "quit"),
		Up:            bind(k.Up, "up"),
		Down:          bind(k.Down, "down"),
		Open:          bind(k.Open, "open / expand"),
		Back:          bind(k.Back, "focus sidebar"),
		CloseTab:      bind(k.CloseTab, "close tab"),
		NextTab:       bind(k.NextTab, "next tab"),
		PrevTab:       bind(k.PrevTab, "previous tab"),
		ToggleSidebar: bind(k.ToggleSidebar, "toggle sidebar"),
		HalfPageDown:  bind(k.HalfPageDown, "half page down"),
		HalfPageUp:    bind(k.HalfPageUp, "half page up"),
		Top:           bind(k.Top, "go to top"),
		Bottom:        bind(k.Bottom, "go to bottom"),
		Reload:        bind(k.Reload, "reload"),
		Help:          bind(k.Help, "toggle help"),
	}
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
