package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hidekingerz/mado/internal/config"
)

func japaneseModel(t *testing.T, files map[string]string, open ...string) Model {
	t.Helper()
	cfg := config.Default()
	cfg.Theme.Style = "notty"
	cfg.General.Language = "ja"
	return testModelWithConfig(t, cfg, files, open...)
}

func screen(m Model) string { return ansi.Strip(m.render()) }

func TestJapaneseScreens(t *testing.T) {
	m := japaneseModel(t, map[string]string{"a.md": "# A"})
	if s := screen(m); !strings.Contains(s, "サイドバーで Markdown ファイルを選ぶか") {
		t.Errorf("welcome screen is not in Japanese:\n%s", s)
	}

	m = update(t, m, keyRune('?'))
	s := screen(m)
	for _, want := range []string{"キー操作", "タブを閉じる", "ファイルのパスをコピー"} {
		if !strings.Contains(s, want) {
			t.Errorf("help screen lacks %q:\n%s", want, s)
		}
	}
	m = update(t, m, keyRune('?'))

	m = update(t, m, keyRune(','))
	if s := screen(m); !strings.Contains(s, "設定") || !strings.Contains(s, "esc 閉じる") {
		t.Errorf("settings panel is not in Japanese:\n%s", s)
	}
	m = update(t, m, esc())

	m = update(t, m, keyRune('/'))
	if s := screen(m); !strings.Contains(s, "ファイル名で検索") {
		t.Errorf("search panel is not in Japanese:\n%s", s)
	}
}

func TestEnglishByDefaultInTests(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	m = update(t, m, keyRune('?'))
	if s := screen(m); !strings.Contains(s, "Keyboard shortcuts") {
		t.Errorf("help screen is not in English:\n%s", s)
	}
}

func TestLanguageSwitchesAtRuntime(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A"})
	next := m.cfg
	next.General.Language = "ja"
	m.applyConfig(next)
	m = update(t, m, keyRune('?'))
	if s := screen(m); !strings.Contains(s, "キー操作") {
		t.Errorf("after switching to ja the help screen is:\n%s", s)
	}
}

func TestJapaneseCopyNotice(t *testing.T) {
	stubClipboard(t)
	m := japaneseModel(t, map[string]string{"a.md": "# A"}, "a.md")
	m = update(t, m, tea.KeyPressMsg{Code: 'y', Text: "y"})
	if s := m.renderStatusBar(); !strings.Contains(ansi.Strip(s), "パスをコピーしました") {
		t.Errorf("status bar = %q", ansi.Strip(s))
	}
}
