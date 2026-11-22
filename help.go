package main

import (
	"flag"
	"fmt"
	"strings"
)

// helpText is what -h prints, in one language.
type helpText struct {
	Usage   string // the synopsis and what mado does, before the flags
	Flags   string // heading over the flag list
	Default string // appended to a flag's description; %q is its default
	Config  string
	Style   string
	Watch   string
	Remote  string
	Version string
}

var helpEN = helpText{
	Usage: "usage: mado [flags] [dir | file ...]\n\n" +
		"Opens a markdown viewer rooted at dir (default: current directory).\n" +
		"Any file arguments are opened in tabs.\n",
	Flags:   "flags:",
	Default: " (default %q)",
	Config:  "path to config.toml",
	Style:   "glamour style (overrides config): auto, dark, light, dracula, … or a style JSON path",
	Watch:   "reload open files and the tree automatically when they change on disk",
	Remote:  "hand the file arguments to a running mado instead of starting a new one: open or focus",
	Version: "print version and exit",
}

var helpJA = helpText{
	Usage: "使い方: mado [フラグ] [ディレクトリ | ファイル ...]\n\n" +
		"ディレクトリ（省略時はカレントディレクトリ）をルートにして Markdown ビューアを開きます。\n" +
		"ファイルを渡すと、それぞれをタブで開きます。\n",
	Flags:   "フラグ:",
	Default: "（既定値: %q）",
	Config:  "config.toml のパス",
	Style:   "glamour のスタイル（設定より優先）: auto, dark, light, dracula, … またはスタイル JSON のパス",
	Watch:   "開いているファイルとツリーを、ディスク上で変更されたら自動で読み直す",
	Remote:  "新しく起動せず、実行中の mado にファイル引数を渡す: open または focus",
	Version: "バージョンを表示して終了",
}

// options holds the parsed command-line flags.
type options struct {
	configPath  *string
	style       *string
	watch       *bool
	remote      *string
	showVersion *bool
}

// registerFlags defines mado's flags on fs, described in m, and sets
// fs.Usage to print them.
func registerFlags(fs *flag.FlagSet, m helpText, defaultConfig string) options {
	o := options{
		configPath:  fs.String("config", defaultConfig, m.Config),
		style:       fs.String("style", "", m.Style),
		watch:       fs.Bool("watch", false, m.Watch),
		remote:      fs.String("remote", "", m.Remote),
		showVersion: fs.Bool("version", false, m.Version),
	}
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintln(out, m.Usage)
		fmt.Fprintln(out, m.Flags)
		printDefaults(fs, m)
	}
	return o
}

// printDefaults lists fs's flags the way flag.PrintDefaults does,
// but with the default value labeled in m's language.
func printDefaults(fs *flag.FlagSet, m helpText) {
	out := fs.Output()
	fs.VisitAll(func(f *flag.Flag) {
		name, usage := flag.UnquoteUsage(f)
		line := "  -" + f.Name
		if name != "" {
			line += " " + name
		}
		line += "\n    \t" + strings.ReplaceAll(usage, "\n", "\n    \t")
		if f.DefValue != "" && f.DefValue != "false" {
			line += fmt.Sprintf(m.Default, f.DefValue)
		}
		fmt.Fprintln(out, line)
	})
}
