package main

import (
	"bytes"
	"flag"
	"reflect"
	"strings"
	"testing"
)

func TestJapaneseLocale(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"LANG ja", map[string]string{"LANG": "ja_JP.UTF-8"}, true},
		{"LANG en", map[string]string{"LANG": "en_US.UTF-8"}, false},
		{"nothing set", nil, false},
		{"C locale", map[string]string{"LANG": "C"}, false},
		{"LC_ALL wins over LANG", map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "ja_JP.UTF-8"}, false},
		{"LC_MESSAGES wins over LANG", map[string]string{"LC_MESSAGES": "ja_JP.UTF-8", "LANG": "en_US.UTF-8"}, true},
		{"empty LC_ALL is skipped", map[string]string{"LC_ALL": "", "LANG": "ja"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(k string) string { return tt.env[k] }
			if got := japaneseLocale(getenv); got != tt.want {
				t.Errorf("japaneseLocale = %v, want %v", got, tt.want)
			}
		})
	}
}

// usageOf registers the flags in a fresh set, as main does, and
// returns what -h prints.
func usageOf(t *testing.T, m helpText) string {
	t.Helper()
	fs := flag.NewFlagSet("mado", flag.ContinueOnError)
	var out bytes.Buffer
	fs.SetOutput(&out)
	registerFlags(fs, m, "/tmp/config.toml")
	fs.Usage()
	return out.String()
}

func TestUsageIsTranslated(t *testing.T) {
	en, ja := usageOf(t, helpEN), usageOf(t, helpJA)
	for _, want := range []string{"usage: mado", "Opens a markdown viewer", "-watch", "print version and exit"} {
		if !strings.Contains(en, want) {
			t.Errorf("English usage lacks %q:\n%s", want, en)
		}
	}
	for _, want := range []string{"使い方: mado", "Markdown ビューア", "パス（既定値: \"/tmp/config.toml\"）", "-watch", "バージョンを表示して終了"} {
		if !strings.Contains(ja, want) {
			t.Errorf("Japanese usage lacks %q:\n%s", want, ja)
		}
	}
}

// Every text is filled in both languages, so a flag added later
// cannot be left blank in one of them.
func TestHelpTextsAreComplete(t *testing.T) {
	for name, m := range map[string]helpText{"en": helpEN, "ja": helpJA} {
		v := reflect.ValueOf(m)
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).String() == "" {
				t.Errorf("%s: %s is empty", name, v.Type().Field(i).Name)
			}
		}
	}
}
