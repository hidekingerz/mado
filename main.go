// mado is a TUI markdown viewer: a file-tree sidebar, tabs for multiple
// open files, mouse support, and TOML-configurable keys and themes.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/ui"
)

func main() {
	configPath := flag.String("config", config.DefaultPath(), "path to config.toml")
	style := flag.String("style", "", "glamour style (overrides config): auto, dark, light, dracula, … or a style JSON path")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: mado [flags] [dir | file ...]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Opens a markdown viewer rooted at dir (default: current directory).\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Any file arguments are opened in tabs.\n\nflags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mado: bad config %s: %v\n", *configPath, err)
		os.Exit(1)
	}
	if *style != "" {
		cfg.Theme.Style = *style
	}

	rootDir := "."
	var files []string
	for _, arg := range flag.Args() {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mado: %v\n", err)
			os.Exit(1)
		}
		if info.IsDir() {
			rootDir = arg
		} else {
			files = append(files, arg)
		}
	}

	m, err := ui.New(cfg, rootDir, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mado: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mado: %v\n", err)
		os.Exit(1)
	}
}
