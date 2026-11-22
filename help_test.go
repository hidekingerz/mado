package main

import (
	"bytes"
	"flag"
	"reflect"
	"strings"
	"testing"
)

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
