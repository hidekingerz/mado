package ui

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const (
	noFileToCopyMsg = "no file open to copy"
	copiedPathMsg   = "✓ copied path"
)

// copyToClipboard puts s on the system clipboard two ways: OSC 52,
// which the terminal carries to its own clipboard even over ssh, and
// the platform's copy command, for terminals (or a tmux left at its
// default set-clipboard) that drop OSC 52. Either alone may silently
// do nothing; together they cover the common setups. It is a variable
// so tests can see what was copied.
var copyToClipboard = func(s string) tea.Cmd {
	return tea.Batch(tea.SetClipboard(s), nativeCopy(s))
}

// nativeCopy runs the platform's copy command with s on its input,
// or returns nil when there is none to run.
func nativeCopy(s string) tea.Cmd {
	argv := clipboardCommand(runtime.GOOS, os.Getenv, exec.LookPath)
	if argv == nil {
		return nil
	}
	return func() tea.Msg {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader(s)
		// A failure is not reported: OSC 52 was sent as well, and
		// whether that reached the clipboard cannot be known either.
		_ = cmd.Run()
		return nil
	}
}

// clipboardCommand picks the command that writes its input to the
// clipboard of the machine the user sits at. Over ssh that is not
// this machine, so there is none and OSC 52 has to do.
func clipboardCommand(goos string, getenv func(string) string, lookPath func(string) (string, error)) []string {
	if getenv("SSH_CONNECTION") != "" || getenv("SSH_TTY") != "" {
		return nil
	}
	var candidates [][]string
	switch goos {
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	case "windows":
		candidates = [][]string{{"clip"}}
	default:
		if getenv("WAYLAND_DISPLAY") != "" {
			candidates = append(candidates, []string{"wl-copy"})
		}
		if getenv("DISPLAY") != "" {
			candidates = append(candidates,
				[]string{"xclip", "-selection", "clipboard"},
				[]string{"xsel", "--clipboard", "--input"})
		}
		// WSL reaches the Windows clipboard through clip.exe.
		candidates = append(candidates, []string{"clip.exe"})
	}
	for _, c := range candidates {
		if _, err := lookPath(c[0]); err == nil {
			return c
		}
	}
	return nil
}
