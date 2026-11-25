// Package i18n holds the text mado shows of its own — help, hints,
// panel titles, status messages — in each language it speaks. Names
// that are also config values or identifiers (reader, source, the
// [READER] tags, key names, setting keys) stay as they are in every
// language, so what the screen says matches what the config file and
// the README say.
package i18n

import "strings"

// Lang is a language mado has text for.
type Lang string

const (
	EN Lang = "en"
	JA Lang = "ja"
)

// Detect picks the language from the locale: the first of LC_ALL,
// LC_MESSAGES and LANG that is set decides, as it does for any program
// that reads the locale. Anything but Japanese is English.
func Detect(getenv func(string) string) Lang {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(k); v != "" {
			if strings.HasPrefix(v, "ja") {
				return JA
			}
			return EN
		}
	}
	return EN
}

// Resolve turns the general.language setting into a language: "en"
// and "ja" as they are, anything else ("auto", empty) from the locale.
func Resolve(setting string, getenv func(string) string) Lang {
	switch Lang(setting) {
	case EN, JA:
		return Lang(setting)
	}
	return Detect(getenv)
}

// For returns the text for l.
func For(l Lang) *Text {
	if l == JA {
		return &ja
	}
	return &en
}

// Text is every string mado shows of its own, in one language. A
// field ending in F is a fmt format; its comment names the arguments.
type Text struct {
	// Key descriptions, in the help screen and the welcome screen.
	KeyQuit, KeyUp, KeyDown, KeyOpen, KeyBack           string
	KeyCloseTab, KeyNextTab, KeyPrevTab                 string
	KeyToggleSidebar, KeyHalfPageDown, KeyHalfPageUp    string
	KeyTop, KeyBottom, KeyReload, KeyToggleMode         string
	KeyToggleAllFiles, KeyToggleHidden, KeyToggleLineNo string
	KeyCopyPath, KeySearch, KeySearchContent            string
	KeySettings, KeyHelp, KeyLinks                      string

	// Help screen.
	HelpTitle, HelpSearch, HelpMouse string

	// Welcome screen and the empty sidebar.
	Tagline, Welcome                              string
	ShowAllFiles, ShowDotfiles, Help              string
	EmptyNoMarkdown, EmptyNoFiles, EmptyNothing   string
	Loading                                       string
	BinaryFileF                                   string // name, size
	NoReaderView                                  string // followed by the mode tag
	WatchDisabled, SaveFailed                     string // followed by the error
	NoFileToCopy, CopiedPath, NotSavedNoPath      string
	NotSaved                                      string
	RenderErrorF                                  string // error
	BarHelp, BarQuit                              string // after the key in the status bar
	SearchBarHintF                                string // the other target
	SearchNames, SearchContents                   string // panel titles
	TargetNames, TargetContents                   string // what tab switches to
	SearchHintNames, SearchHintContents           string // under an empty query
	TabHintF                                      string // the other target
	Searching, NoMatches, OneMatch                string
	MatchesF, MatchesMoreF                        string // count
	SettingsTitle, PressAKey, Empty, DefaultLabel string
	LinksTitle, NoLinks, LinkMissing, OneLink     string
	LinksF                                        string // count
	LinksBarHint                                  string

	// Settings panel hints in the status bar.
	HintApply, HintCapturing, HintPick, HintClose, HintUndo string
	HintToggle, HintChoose, HintCapture, HintEdit           string

	// Fields describes the settings panel's rows by "table.key". Empty
	// in English: config.Field.Desc is the English text.
	Fields map[string]string
}

// FieldDesc is the description of the setting table.key, or fallback
// (the English one) when t has none.
func (t *Text) FieldDesc(table, key, fallback string) string {
	if d, ok := t.Fields[table+"."+key]; ok {
		return d
	}
	return fallback
}

var en = Text{
	KeyQuit: "quit", KeyUp: "up", KeyDown: "down", KeyOpen: "open / expand",
	KeyBack: "focus sidebar", KeyCloseTab: "close tab", KeyNextTab: "next tab",
	KeyPrevTab: "previous tab", KeyToggleSidebar: "toggle sidebar",
	KeyHalfPageDown: "half page down", KeyHalfPageUp: "half page up",
	KeyTop: "go to top", KeyBottom: "go to bottom", KeyReload: "reload",
	KeyToggleMode: "reader/source mode", KeyToggleAllFiles: "all files / markdown only",
	KeyToggleHidden: "show / hide dotfiles", KeyToggleLineNo: "line numbers",
	KeyCopyPath: "copy file path", KeySearch: "search file names",
	KeySearchContent: "search file contents", KeySettings: "settings", KeyHelp: "toggle help",
	KeyLinks: "open a link in the file",

	HelpTitle:  "Keyboard shortcuts",
	HelpSearch: "Search: type to filter, tab switches names/contents,\n↑/↓ select, enter opens, esc closes.",
	HelpMouse:  "Mouse: click to select/open, wheel to scroll,\nclick a tab to switch, ✕ to close.",

	Tagline:            " tui markdown viewer",
	Welcome:            "Select a markdown file in the sidebar,\nor click one with the mouse.",
	ShowAllFiles:       "show all files",
	ShowDotfiles:       "show dotfiles",
	Help:               "help",
	EmptyNoMarkdown:    "No markdown files here.",
	EmptyNoFiles:       "No files here.",
	EmptyNothing:       "Nothing to show here.",
	Loading:            "loading…",
	BinaryFileF:        "%s\n\nbinary file (%s) — not displayed",
	NoReaderView:       "no reader view for this file; markdown tabs: ",
	WatchDisabled:      "watch disabled: ",
	SaveFailed:         "save failed: ",
	NoFileToCopy:       "no file open to copy",
	CopiedPath:         "✓ copied path",
	NotSavedNoPath:     "not saved: no config path",
	NotSaved:           "not saved",
	RenderErrorF:       "[render error: %s]",
	BarHelp:            "help",
	BarQuit:            "quit",
	SearchBarHintF:     "enter open │ tab %s │ esc close ",
	SearchNames:        "Search names",
	SearchContents:     "Search contents",
	TargetNames:        "names",
	TargetContents:     "contents",
	SearchHintNames:    "Type to search file names. tab switches to contents, enter opens, esc closes.",
	SearchHintContents: "Type to search file contents. tab switches to names, enter opens, esc closes.",
	TabHintF:           "tab: %s",
	Searching:          "searching…",
	NoMatches:          "no matches",
	OneMatch:           "1 match",
	MatchesF:           "%d matches",
	MatchesMoreF:       "%d+ matches",
	SettingsTitle:      "Settings",
	PressAKey:          "press a key…",
	Empty:              "(empty)",
	DefaultLabel:       "default: ",
	LinksTitle:         "Links",
	NoLinks:            "no local links in this file",
	LinkMissing:        "not found",
	OneLink:            "1 link",
	LinksF:             "%d links",
	LinksBarHint:       "enter open │ esc close ",

	HintApply:     "enter apply │ esc cancel ",
	HintCapturing: "press a key │ ctrl+c quit ",
	HintPick:      "↑/↓ choose │ enter apply │ esc cancel ",
	HintClose:     "esc close ",
	HintUndo:      "u undo │ ",
	HintToggle:    "enter toggle │ ",
	HintChoose:    "enter choose │ ←/→ change │ ",
	HintCapture:   "enter capture │ backspace remove │ ",
	HintEdit:      "enter edit │ ",
}

var ja = Text{
	KeyQuit: "終了", KeyUp: "上へ", KeyDown: "下へ", KeyOpen: "開く / 展開",
	KeyBack: "サイドバーへ移動", KeyCloseTab: "タブを閉じる", KeyNextTab: "次のタブ",
	KeyPrevTab: "前のタブ", KeyToggleSidebar: "サイドバーの表示切替",
	KeyHalfPageDown: "半ページ下へ", KeyHalfPageUp: "半ページ上へ",
	KeyTop: "先頭へ", KeyBottom: "末尾へ", KeyReload: "読み直す",
	KeyToggleMode: "reader / source の切替", KeyToggleAllFiles: "全ファイル / Markdown のみ",
	KeyToggleHidden: "ドットファイルの表示切替", KeyToggleLineNo: "行番号",
	KeyCopyPath: "ファイルのパスをコピー", KeySearch: "ファイル名で検索",
	KeySearchContent: "内容で検索", KeySettings: "設定", KeyHelp: "ヘルプの表示切替",
	KeyLinks: "ファイル内のリンクを開く",

	HelpTitle:  "キー操作",
	HelpSearch: "検索: 入力で絞り込み、tab で名前 / 内容を切替、\n↑/↓ で選択、enter で開く、esc で閉じる。",
	HelpMouse:  "マウス: クリックで選択 / 開く、ホイールでスクロール、\nタブをクリックで切替、✕ で閉じる。",

	Tagline:            " TUI Markdown ビューア",
	Welcome:            "サイドバーで Markdown ファイルを選ぶか、\nマウスでクリックしてください。",
	ShowAllFiles:       "全ファイルを表示",
	ShowDotfiles:       "ドットファイルを表示",
	Help:               "ヘルプ",
	EmptyNoMarkdown:    "Markdown ファイルがありません。",
	EmptyNoFiles:       "ファイルがありません。",
	EmptyNothing:       "表示するものがありません。",
	Loading:            "読み込み中…",
	BinaryFileF:        "%s\n\nバイナリファイル（%s）のため表示しません",
	NoReaderView:       "このファイルに reader 表示はありません。Markdown のタブ: ",
	WatchDisabled:      "監視を無効にしました: ",
	SaveFailed:         "保存に失敗しました: ",
	NoFileToCopy:       "コピーするファイルが開かれていません",
	CopiedPath:         "✓ パスをコピーしました",
	NotSavedNoPath:     "保存されません: 設定ファイルのパスがありません",
	NotSaved:           "保存されません",
	RenderErrorF:       "[描画エラー: %s]",
	BarHelp:            "ヘルプ",
	BarQuit:            "終了",
	SearchBarHintF:     "enter 開く │ tab %s │ esc 閉じる ",
	SearchNames:        "ファイル名で検索",
	SearchContents:     "内容で検索",
	TargetNames:        "名前",
	TargetContents:     "内容",
	SearchHintNames:    "入力してファイル名を検索。tab で内容検索に切替、enter で開く、esc で閉じる。",
	SearchHintContents: "入力してファイルの内容を検索。tab で名前検索に切替、enter で開く、esc で閉じる。",
	TabHintF:           "tab: %s",
	Searching:          "検索中…",
	NoMatches:          "見つかりません",
	OneMatch:           "1 件",
	MatchesF:           "%d 件",
	MatchesMoreF:       "%d 件以上",
	SettingsTitle:      "設定",
	PressAKey:          "キーを押してください…",
	Empty:              "（空）",
	DefaultLabel:       "既定値: ",
	LinksTitle:         "リンク",
	NoLinks:            "このファイルにローカルリンクはありません",
	LinkMissing:        "見つかりません",
	OneLink:            "1 件",
	LinksF:             "%d 件",
	LinksBarHint:       "enter 開く │ esc 閉じる ",

	HintApply:     "enter 適用 │ esc 取消 ",
	HintCapturing: "キーを押す │ ctrl+c 終了 ",
	HintPick:      "↑/↓ 選ぶ │ enter 適用 │ esc 取消 ",
	HintClose:     "esc 閉じる ",
	HintUndo:      "u 元に戻す │ ",
	HintToggle:    "enter 切替 │ ",
	HintChoose:    "enter 選ぶ │ ←/→ 変更 │ ",
	HintCapture:   "enter キー入力 │ backspace 削除 │ ",
	HintEdit:      "enter 編集 │ ",

	Fields: map[string]string{
		"general.watch":            "ディスク上で変更されたら、開いているファイルとツリーを読み直す",
		"general.mermaid":          "reader 表示で mermaid のコードブロックを図として描く",
		"general.language":         "mado の表示言語。auto はロケールに従う",
		"theme.style":              "glamour の Markdown スタイル、またはスタイル JSON のパス",
		"theme.default_mode":       "ファイルを開いたときの表示モード",
		"theme.source_style":       "source 表示の chroma スタイル。空なら style から選ぶ",
		"theme.accent_color":       "フォーカス中の枠、アクティブなタブ、選択（#RRGGBB または 0-255）",
		"theme.border_color":       "フォーカスのないペインの枠（#RRGGBB または 0-255）",
		"theme.dir_color":          "サイドバーのディレクトリ名（#RRGGBB または 0-255）",
		"theme.file_color":         "サイドバーのファイル名（#RRGGBB または 0-255）",
		"theme.selection_fg":       "選択行の文字色（#RRGGBB または 0-255）",
		"theme.selection_bg":       "選択行の背景色（#RRGGBB または 0-255）",
		"theme.status_fg":          "ステータスバーの文字色（#RRGGBB または 0-255）",
		"theme.status_bg":          "ステータスバーの背景色（#RRGGBB または 0-255）",
		"sidebar.width":            "サイドバーの幅（桁数、16 以上）",
		"sidebar.show_all_files":   "Markdown 以外のファイルも一覧に出す",
		"sidebar.show_hidden":      "ドットファイルとドットディレクトリを一覧に出す",
		"search.exclude":           "検索で飛ばすパターン（カンマ区切り）。空ならすべて検索する",
		"keys.quit":                "終了",
		"keys.up":                  "上へ移動 / 上へスクロール",
		"keys.down":                "下へ移動 / 下へスクロール",
		"keys.open":                "ファイルを開く / ディレクトリを展開",
		"keys.back":                "サイドバーへ移動",
		"keys.close_tab":           "タブを閉じる",
		"keys.next_tab":            "次のタブ",
		"keys.prev_tab":            "前のタブ",
		"keys.toggle_sidebar":      "サイドバーの表示 / 非表示",
		"keys.half_page_down":      "半ページ下へ",
		"keys.half_page_up":        "半ページ上へ",
		"keys.top":                 "先頭へ",
		"keys.bottom":              "末尾へ",
		"keys.reload":              "ツリーとファイルを読み直す",
		"keys.toggle_mode":         "reader / source の切替",
		"keys.toggle_all_files":    "全ファイル / Markdown のみ",
		"keys.toggle_hidden":       "ドットファイルの表示 / 非表示",
		"keys.toggle_line_numbers": "source 表示の行番号",
		"keys.copy_path":           "開いているファイルの絶対パスをコピー",
		"keys.links":               "開いているファイルのローカルリンクを一覧して開く",
		"keys.search":              "ファイル名で検索",
		"keys.search_content":      "ファイルの内容で検索",
		"keys.settings":            "この設定パネルを開く",
		"keys.help":                "ヘルプ",
	},
}
