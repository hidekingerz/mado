package ui

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/muesli/termenv"
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

// chromaFormatterName picks a chroma formatter for the terminal's color
// capability. Queries the terminal — call once at startup only.
// "" disables highlighting.
func chromaFormatterName() string {
	switch termenv.ColorProfile() {
	case termenv.TrueColor:
		return "terminal16m"
	case termenv.ANSI256:
		return "terminal256"
	case termenv.ANSI:
		return "terminal16"
	default:
		return ""
	}
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
