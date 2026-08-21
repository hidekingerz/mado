package termsafe

import "testing"

func TestStringKeepsOrdinaryText(t *testing.T) {
	in := "# Heading\n\nsome *text* with a tab\there\nand 日本語 ✓\n"
	if got := String(in); got != in {
		t.Errorf("String changed plain text:\n got %q\nwant %q", got, in)
	}
}

func TestStringDefusesEscapeSequences(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"OSC 52 clipboard write", "a \x1b]52;c;aGFjaw==\x07 b", "a ^[]52;c;aGFjaw==^G b"},
		{"CSI screen clear", "a \x1b[2J b", "a ^[[2J b"},
		{"carriage return overwrite", "real line\rfake line", "real line^Mfake line"},
		{"delete", "a\x7fb", "a^?b"},
		{"single-byte CSI", "a\u009b2Jb", "a<9B>2Jb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := String(c.in); got != c.want {
				t.Errorf("String(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestStringLeavesNoEscape(t *testing.T) {
	out := String("\x1b]0;title\x07\x1b[31mred\x1b[0m")
	for _, r := range out {
		if isControl(r) {
			t.Fatalf("control character %q survived in %q", r, out)
		}
	}
}
