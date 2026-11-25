package ui

import (
	"charm.land/bubbles/v2/key"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/i18n"
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
	ToggleHidden   key.Binding
	ToggleLineNums key.Binding
	CopyPath       key.Binding
	Links          key.Binding
	Search         key.Binding
	SearchContent  key.Binding
	Settings       key.Binding
	Help           key.Binding
}

func newKeyMap(k config.Keys, t *i18n.Text) keyMap {
	bind := func(keys []string, help string) key.Binding {
		keys = keyNames(keys)
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(joinKeys(keys), help))
	}
	return keyMap{
		Quit:           bind(k.Quit, t.KeyQuit),
		Up:             bind(k.Up, t.KeyUp),
		Down:           bind(k.Down, t.KeyDown),
		Open:           bind(k.Open, t.KeyOpen),
		Back:           bind(k.Back, t.KeyBack),
		CloseTab:       bind(k.CloseTab, t.KeyCloseTab),
		NextTab:        bind(k.NextTab, t.KeyNextTab),
		PrevTab:        bind(k.PrevTab, t.KeyPrevTab),
		ToggleSidebar:  bind(k.ToggleSidebar, t.KeyToggleSidebar),
		HalfPageDown:   bind(k.HalfPageDown, t.KeyHalfPageDown),
		HalfPageUp:     bind(k.HalfPageUp, t.KeyHalfPageUp),
		Top:            bind(k.Top, t.KeyTop),
		Bottom:         bind(k.Bottom, t.KeyBottom),
		Reload:         bind(k.Reload, t.KeyReload),
		ToggleMode:     bind(k.ToggleMode, t.KeyToggleMode),
		ToggleAllFiles: bind(k.ToggleAllFiles, t.KeyToggleAllFiles),
		ToggleHidden:   bind(k.ToggleHidden, t.KeyToggleHidden),
		ToggleLineNums: bind(k.ToggleLineNums, t.KeyToggleLineNo),
		CopyPath:       bind(k.CopyPath, t.KeyCopyPath),
		Links:          bind(k.Links, t.KeyLinks),
		Search:         bind(k.Search, t.KeySearch),
		SearchContent:  bind(k.SearchContent, t.KeySearchContent),
		Settings:       bind(k.Settings, t.KeySettings),
		Help:           bind(k.Help, t.KeyHelp),
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
