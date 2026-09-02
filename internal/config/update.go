package config

import (
	"fmt"
	"strconv"
	"strings"
)

// encodeValue renders v as a TOML value: a basic string, a boolean,
// an integer, or a one-line array of strings.
func encodeValue(v any) (string, error) {
	switch v := v.(type) {
	case string:
		return quoteTOML(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case int:
		return strconv.Itoa(v), nil
	case []string:
		parts := make([]string, len(v))
		for i, s := range v {
			parts[i] = quoteTOML(s)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	default:
		return "", fmt.Errorf("cannot encode %T as TOML", v)
	}
}

// quoteTOML writes s as a TOML basic string. Go's strconv.Quote is
// close but uses escapes TOML does not know (\x, \a), so this spells
// out the few TOML has.
func quoteTOML(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// replaceKey returns src with table.key set to encoded and nothing
// else touched: comments, blank lines, ordering and every other key
// stay as they were. A key missing from its table is added after the
// table's last key; a table missing from the file is appended.
func replaceKey(src, table, key, encoded string) string {
	lines := strings.Split(src, "\n")
	inTable, tableSeen := false, false
	insertAt := -1 // a missing key goes after this line
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if name, ok := tableHeader(t); ok {
			if inTable {
				break // the target table ended
			}
			inTable = name == table
			if inTable {
				tableSeen = true
				insertAt = i
			}
			continue
		}
		if !inTable || t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		name, eq := keyName(lines[i])
		if eq < 0 {
			continue
		}
		end := valueEnd(lines, i, eq+1)
		if name == key {
			lines[i] = lines[i][:eq+1] + " " + encoded + trailingComment(lines[end])
			lines = append(lines[:i+1], lines[end+1:]...)
			return strings.Join(lines, "\n")
		}
		insertAt = end
		i = end
	}
	if !tableSeen {
		out := src
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if out != "" {
			out += "\n"
		}
		return out + "[" + table + "]\n" + key + " = " + encoded + "\n"
	}
	line := key + " = " + encoded
	lines = append(lines[:insertAt+1], append([]string{line}, lines[insertAt+1:]...)...)
	return strings.Join(lines, "\n")
}

// tableHeader reports the name on a "[table]" line. Arrays of tables
// ("[[x]]") are not something mado's config has and are left alone.
func tableHeader(t string) (string, bool) {
	if !strings.HasPrefix(t, "[") || strings.HasPrefix(t, "[[") {
		return "", false
	}
	end := strings.Index(t, "]")
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(t[1:end]), true
}

// keyName returns the bare key at the start of line and the index of
// its "=", or -1 when the line is not a plain "key = value". Quoted
// and dotted keys are not mado's; they are left for the parser to
// judge after the edit.
func keyName(line string) (string, int) {
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", -1
	}
	name := strings.TrimSpace(line[:eq])
	if name == "" || strings.ContainsAny(name, "\"' .") {
		return "", -1
	}
	return name, eq
}

// valueEnd returns the index of the line on which the value starting
// at lines[i][from:] ends: the same line unless an array is left
// open, in which case brackets outside quoted strings are counted
// until it closes.
func valueEnd(lines []string, i, from int) int {
	depth := bracketDepth(lines[i][from:], 0)
	for depth > 0 && i+1 < len(lines) {
		i++
		depth = bracketDepth(lines[i], depth)
	}
	return i
}

// outsideStrings calls fn for every rune of line that is outside a
// quoted string, stopping at a comment; it returns the index where
// the comment starts, or len(line) when there is none.
func outsideStrings(line string, fn func(r rune)) int {
	var quote rune
	escaped := false
	for i, r := range line {
		if quote != 0 {
			switch {
			case escaped:
				escaped = false
			case r == '\\' && quote == '"':
				escaped = true
			case r == quote:
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'':
			quote = r
		case '#':
			return i
		default:
			fn(r)
		}
	}
	return len(line)
}

// bracketDepth adds the brackets on line that are outside strings and
// comments to depth.
func bracketDepth(line string, depth int) int {
	outsideStrings(line, func(r rune) {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
		}
	})
	return depth
}

// trailingComment returns the comment at the end of line together
// with the whitespace before it, or "" when there is none.
func trailingComment(line string) string {
	i := outsideStrings(line, func(rune) {})
	if i == len(line) {
		return ""
	}
	start := i
	for start > 0 && (line[start-1] == ' ' || line[start-1] == '\t') {
		start--
	}
	if start == i {
		return " " + line[i:]
	}
	return line[start:]
}
