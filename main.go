// mado is a TUI markdown viewer: a file-tree sidebar, tabs for multiple
// open files, mouse support, and TOML-configurable keys and themes.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"

	tea "charm.land/bubbletea/v2"

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
	help := helpEN
	if japaneseLocale(os.Getenv) {
		help = helpJA
	}
	opts := registerFlags(flag.CommandLine, help, config.DefaultPath())
	configPath, style, watchFiles, remoteCmd, showVersion := opts.configPath, opts.style, opts.watch, opts.remote, opts.showVersion
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
	m = m.WithConfigPath(*configPath)
	os.Exit(run(m))
}

// run serves remote commands and runs the program until it ends,
// returning the exit status. From the moment the socket exists every
// way out goes through here, so the socket is removed however mado
// stops — short of SIGKILL, which the next instance tidies up after
// (remote.Prune).
func run(m ui.Model) int {
	remote.Prune(remote.DefaultDir())
	srv, err := remote.Listen(remote.DefaultPath())
	if err != nil {
		warn(fmt.Errorf("remote commands disabled: %w", err))
	} else {
		defer srv.Close()
		m = m.Serve(srv)
	}

	p := tea.NewProgram(m)
	// Closing the terminal or the pane sends SIGHUP, which Bubble Tea
	// does not handle and which would otherwise end the process on
	// the spot. There is no terminal left to restore, so stop at once.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	hungUp := make(chan struct{})
	go func() {
		if _, ok := <-hup; ok {
			close(hungUp)
			p.Kill()
		}
	}()

	if _, err := p.Run(); err != nil {
		select {
		case <-hungUp:
			return 0
		default:
		}
		warn(err)
		return 1
	}
	return 0
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
