// mado is a TUI markdown viewer: a file-tree sidebar, tabs for multiple
// open files, mouse support, and TOML-configurable keys and themes.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/remote"
	"github.com/hidekingerz/mado/internal/termsafe"
	"github.com/hidekingerz/mado/internal/ui"
)

// version is set at release time by GoReleaser via ldflags. For builds
// made with `go install`, the module version from build info is used
// instead.
var version = ""

func versionString() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "devel"
}

func main() {
	configPath := flag.String("config", config.DefaultPath(), "path to config.toml")
	style := flag.String("style", "", "glamour style (overrides config): auto, dark, light, dracula, … or a style JSON path")
	watchFiles := flag.Bool("watch", false, "reload open files and the tree automatically when they change on disk")
	remoteCmd := flag.String("remote", "", "hand the file arguments to a running mado instead of starting a new one: open or focus")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: mado [flags] [dir | file ...]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Opens a markdown viewer rooted at dir (default: current directory).\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Any file arguments are opened in tabs.\n\nflags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("mado " + versionString())
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fatal(fmt.Errorf("bad config %s: %w", *configPath, err))
	}
	if *style != "" {
		cfg.Theme.Style = *style
	}
	if *watchFiles {
		cfg.General.Watch = true
	}

	if *remoteCmd != "" {
		sent, err := sendRemote(*remoteCmd, flag.Args())
		if err != nil {
			fatal(err)
		}
		if sent {
			return
		}
		// No instance answered: fall through and be that instance.
	}

	rootDir := "."
	var files []string
	for _, arg := range flag.Args() {
		info, err := os.Stat(arg)
		if err != nil {
			fatal(err)
		}
		if info.IsDir() {
			rootDir = arg
		} else {
			files = append(files, arg)
		}
	}

	m, err := ui.New(cfg, rootDir, files)
	if err != nil {
		fatal(err)
	}
	if srv, err := remote.Listen(remote.DefaultPath()); err != nil {
		warn(fmt.Errorf("remote commands disabled: %w", err))
	} else {
		defer srv.Close()
		m = m.Serve(srv)
	}

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fatal(err)
	}
}

// fatal reports an error and exits. Diagnostics carry file names and
// paths, chosen by whoever created them rather than by the person
// reading them, so they are defused before reaching the terminal.
func fatal(err error) {
	warn(err)
	os.Exit(1)
}

func warn(err error) {
	fmt.Fprintln(os.Stderr, termsafe.String("mado: "+err.Error()))
}

// sendRemote hands the named files to a running instance. It reports
// false when no instance answered, which leaves the caller to start
// one. Whether a file can be opened is the instance's call, not ours:
// focus reaches a file that has since been deleted, and open reports
// back whatever reading it said.
func sendRemote(cmd string, files []string) (bool, error) {
	switch cmd {
	case remote.CmdOpen, remote.CmdFocus:
	default:
		return false, fmt.Errorf("unknown -remote command %q (want %s or %s)", cmd, remote.CmdOpen, remote.CmdFocus)
	}
	if len(files) == 0 {
		return false, fmt.Errorf("-remote %s needs at least one file", cmd)
	}

	dir := remote.DefaultDir()
	for i, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return false, err
		}
		if err := remote.Send(dir, cmd, abs); err != nil {
			// Nothing is listening. Report it only for the first
			// file: once a later one fails this way the instance
			// vanished mid-run, which is worth surfacing.
			if errors.Is(err, remote.ErrNoInstance) && i == 0 {
				return false, nil
			}
			return false, err
		}
	}
	return true, nil
}
