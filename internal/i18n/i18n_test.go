package i18n

import (
	"reflect"
	"regexp"
	"slices"
	"testing"

	"github.com/hidekingerz/mado/internal/config"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Lang
	}{
		{"LANG ja", map[string]string{"LANG": "ja_JP.UTF-8"}, JA},
		{"LANG en", map[string]string{"LANG": "en_US.UTF-8"}, EN},
		{"nothing set", nil, EN},
		{"C locale", map[string]string{"LANG": "C"}, EN},
		{"LC_ALL wins over LANG", map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "ja_JP.UTF-8"}, EN},
		{"LC_MESSAGES wins over LANG", map[string]string{"LC_MESSAGES": "ja_JP.UTF-8", "LANG": "en_US.UTF-8"}, JA},
		{"empty LC_ALL is skipped", map[string]string{"LC_ALL": "", "LANG": "ja"}, JA},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(k string) string { return tt.env[k] }
			if got := Detect(getenv); got != tt.want {
				t.Errorf("Detect = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	jaEnv := func(k string) string {
		if k == "LANG" {
			return "ja_JP.UTF-8"
		}
		return ""
	}
	for setting, want := range map[string]Lang{"en": EN, "ja": JA, "auto": JA, "": JA} {
		if got := Resolve(setting, jaEnv); got != want {
			t.Errorf("Resolve(%q) on a Japanese locale = %v, want %v", setting, got, want)
		}
	}
	// Every value the config accepts resolves.
	for _, l := range config.Languages {
		if got := Resolve(l, jaEnv); got != EN && got != JA {
			t.Errorf("Resolve(%q) = %q", l, got)
		}
	}
}

var verb = regexp.MustCompile(`%[a-z]`)

// Every string is filled in both languages, and a format takes the
// same arguments in each, so adding a string in one language only is
// caught here rather than on someone's screen.
func TestTextsAreComplete(t *testing.T) {
	ev, jv := reflect.ValueOf(en), reflect.ValueOf(ja)
	for i := 0; i < ev.NumField(); i++ {
		name := ev.Type().Field(i).Name
		if ev.Field(i).Kind() != reflect.String {
			continue
		}
		e, j := ev.Field(i).String(), jv.Field(i).String()
		if e == "" || j == "" {
			t.Errorf("%s: en %q, ja %q; want both filled", name, e, j)
		}
		if ev, jv := verb.FindAllString(e, -1), verb.FindAllString(j, -1); !slices.Equal(ev, jv) {
			t.Errorf("%s: en takes %v, ja takes %v", name, ev, jv)
		}
	}
}

// Japanese describes every row of the settings panel, and nothing
// that is no longer a row.
func TestJapaneseDescribesEverySetting(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range config.Fields() {
		id := f.Table + "." + f.Key
		seen[id] = true
		if ja.Fields[id] == "" {
			t.Errorf("no Japanese description for %s", id)
		}
		if got := For(EN).FieldDesc(f.Table, f.Key, f.Desc); got != f.Desc {
			t.Errorf("English %s = %q, want the field's own %q", id, got, f.Desc)
		}
	}
	for id := range ja.Fields {
		if !seen[id] {
			t.Errorf("Japanese describes %s, which is not a setting", id)
		}
	}
}
