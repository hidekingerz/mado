package ui

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/colorprofile"
)

// chromaStyleName maps the resolved glamour style to a chroma style.
// override (theme.source_style) wins when set. "" disables highlighting.
func chromaStyleName(glamourStyle, override string, darkBG bool) string {
	if override != "" {
		return override
	}
	switch glamourStyle {
	case "dark":
		return "catppuccin-mocha"
	case "light":
		return "catppuccin-latte"
	case "dracula":
		return "dracula"
	case "notty", "ascii":
		return ""
	default: // custom style JSON or anything unrecognized
		if darkBG {
			return "catppuccin-mocha"
		}
		return "catppuccin-latte"
	}
}

// chromaFormatterFor picks a chroma formatter for the terminal's color
// profile. Bubble Tea converts every color to the profile as it draws,
// so full color is right for any terminal that shows color at all.
// "" disables highlighting.
func chromaFormatterFor(p colorprofile.Profile) string {
	if p == colorprofile.Ascii || p == colorprofile.NoTTY {
		return ""
	}
	return "terminal16m"
}

// highlightSource syntax-highlights src, detecting the language from
// path's filename. On any failure or unknown input it returns src
// unchanged so the display never breaks.
func highlightSource(src, path, styleName, formatterName string) (string, error) {
	if styleName == "" || formatterName == "" {
		return src, nil
	}
	lexer := lexers.Match(filepath.Base(path))
	if lexer == nil {
		return src, nil
	}
	lexer = chroma.Coalesce(lexer)
	formatter := formatters.Get(formatterName)
	if formatter == nil {
		return src, nil
	}
	it, err := lexer.Tokenise(nil, src)
	if err != nil {
		return src, err
	}
	var b strings.Builder
	if err := formatter.Format(&b, chromastyles.Get(styleName), it); err != nil {
		return src, err
	}
	return b.String(), nil
}
