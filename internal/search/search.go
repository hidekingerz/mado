// Package search finds files under mado's root by name or by content,
// for the search panel.
package search

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hidekingerz/mado/internal/filetree"
)

// Target selects what a query is matched against.
type Target int

const (
	// Names matches the query against root-relative file paths.
	Names Target = iota
	// Contents matches the query against the lines of text files.
	Contents
)

func (t Target) String() string {
	if t == Contents {
		return "contents"
	}
	return "names"
}

// Toggle returns the other target.
func (t Target) Toggle() Target {
	if t == Contents {
		return Names
	}
	return Contents
}

// DefaultMaxResults bounds a result set when Options.MaxResults is 0.
const DefaultMaxResults = 1000

// maxContentSize is the largest file whose contents are searched.
// Bigger files are skipped rather than read into memory.
const maxContentSize = 8 << 20

// binarySampleSize is how many leading bytes are inspected when
// deciding whether a file is binary (a NUL byte is the signal, as in
// the viewer).
const binarySampleSize = 8192

// Options controls which files are searched.
type Options struct {
	// Tree is the sidebar's filter: markdown only or all files, and
	// whether dotfiles are included. The search sees what the sidebar
	// could show.
	Tree filetree.Options
	// Exclude lists glob patterns for files and directories to leave
	// out. See Excluded for how they are matched.
	Exclude []string
	// MaxResults caps the number of matches; 0 means DefaultMaxResults.
	MaxResults int
}

// Match is one hit.
type Match struct {
	Path string // absolute path of the file
	Rel  string // path relative to the root, slash-separated
	Line int    // 1-based line number; 0 for a name match
	Text string // the matching line, for content matches
}

// Result is the outcome of one search.
type Result struct {
	Matches []Match
	// Truncated reports that MaxResults was reached and more matches
	// may exist.
	Truncated bool
	// Err is the first read error met while walking, if any. A search
	// that hit errors still returns the matches it found.
	Err error
}

// Run searches the tree under root. It stops early when ctx is
// cancelled, returning what it found so far. An empty query matches
// nothing.
func Run(ctx context.Context, root string, target Target, query string, opts Options) Result {
	if query == "" {
		return Result{}
	}
	limit := opts.MaxResults
	if limit <= 0 {
		limit = DefaultMaxResults
	}
	m := newMatcher(query)
	var res Result
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if res.Err == nil {
				res.Err = err
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if p == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		name := d.Name()
		if !opts.Tree.ShowHidden && strings.HasPrefix(name, ".") || Excluded(rel, opts.Exclude) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !opts.Tree.ShowAllFiles && !filetree.IsMarkdown(name) {
			return nil
		}
		switch target {
		case Names:
			if m.index(rel) >= 0 {
				res.Matches = append(res.Matches, Match{Path: p, Rel: rel})
			}
		case Contents:
			res.Matches = append(res.Matches, grepFile(p, rel, d, m, limit-len(res.Matches))...)
			if len(res.Matches) >= limit {
				res.Truncated = true
				return fs.SkipAll
			}
		}
		return nil
	})
	if walkErr != nil && !errorsIsSkip(walkErr) && res.Err == nil {
		res.Err = walkErr
	}
	if target == Names {
		sortNames(res.Matches, m)
		if len(res.Matches) > limit {
			res.Matches = res.Matches[:limit]
			res.Truncated = true
		}
	}
	return res
}

func errorsIsSkip(err error) bool {
	return errors.Is(err, fs.SkipDir) || errors.Is(err, fs.SkipAll)
}

// grepFile returns up to limit matching lines of the file at p. Files
// that are too big, unreadable, or binary yield nothing.
func grepFile(p, rel string, d fs.DirEntry, m matcher, limit int) []Match {
	if limit <= 0 {
		return nil
	}
	if info, err := d.Info(); err != nil || info.Size() > maxContentSize {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	sample := data
	if len(sample) > binarySampleSize {
		sample = sample[:binarySampleSize]
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return nil
	}
	var out []Match
	for i, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if m.index(string(line)) < 0 {
			continue
		}
		out = append(out, Match{Path: p, Rel: rel, Line: i + 1, Text: string(line)})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// sortNames orders name matches by usefulness: a hit in the file name
// itself before one only in a directory, then shallower paths, then
// alphabetically.
func sortNames(ms []Match, m matcher) {
	rank := func(x Match) (int, int) {
		inBase := 1
		if m.index(path.Base(x.Rel)) >= 0 {
			inBase = 0
		}
		return inBase, strings.Count(x.Rel, "/")
	}
	sort.SliceStable(ms, func(i, j int) bool {
		bi, di := rank(ms[i])
		bj, dj := rank(ms[j])
		if bi != bj {
			return bi < bj
		}
		if di != dj {
			return di < dj
		}
		return strings.ToLower(ms[i].Rel) < strings.ToLower(ms[j].Rel)
	})
}

// Excluded reports whether the root-relative, slash-separated path rel
// matches one of the exclude patterns. A pattern without a slash is
// matched against the last path element, so "node_modules" excludes
// that directory at any depth and "*.log" every log file; a pattern
// with a slash is matched against the whole relative path, so
// "docs/drafts" excludes just that directory. Patterns use path.Match
// syntax; a malformed pattern matches literally. A trailing slash is
// ignored.
func Excluded(rel string, patterns []string) bool {
	base := path.Base(rel)
	for _, pat := range patterns {
		pat = strings.TrimSuffix(strings.TrimSpace(pat), "/")
		if pat == "" {
			continue
		}
		subject := base
		if strings.Contains(pat, "/") {
			subject = rel
		}
		ok, err := path.Match(pat, subject)
		if err != nil {
			ok = pat == subject
		}
		if ok {
			return true
		}
	}
	return false
}

// Find returns the byte range [start, end) of the first occurrence of
// query in s, or (-1, -1). Matching is smart-case: a query without
// upper-case letters ignores case, one with any upper-case letter is
// exact.
func Find(s, query string) (start, end int) {
	return newMatcher(query).find(s)
}

// Index is Find's start offset, or -1.
func Index(s, query string) int {
	start, _ := Find(s, query)
	return start
}

// matcher holds a query prepared for repeated matching.
type matcher struct {
	query  string
	folded []rune // lower-cased query; nil when the match is exact
}

func newMatcher(query string) matcher {
	m := matcher{query: query}
	if !strings.ContainsFunc(query, unicode.IsUpper) {
		m.folded = foldRunes(query)
	}
	return m
}

func (m matcher) index(s string) int {
	start, _ := m.find(s)
	return start
}

func (m matcher) find(s string) (start, end int) {
	if m.query == "" {
		return -1, -1
	}
	if m.folded == nil {
		i := strings.Index(s, m.query)
		if i < 0 {
			return -1, -1
		}
		return i, i + len(m.query)
	}
	// Lower-casing rune by rune keeps the rune count, so a position in
	// the folded text maps back onto the original bytes — even where
	// a folded rune is a different byte length from the original.
	pos := indexRunes(foldRunes(s), m.folded)
	if pos < 0 {
		return -1, -1
	}
	start, end = -1, -1
	off, i := 0, 0
	for _, r := range s {
		if i == pos {
			start = off
		}
		if i == pos+len(m.folded) {
			end = off
			break
		}
		off += utf8.RuneLen(r)
		i++
	}
	if end < 0 {
		end = len(s)
	}
	return start, end
}

func foldRunes(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		out = append(out, unicode.ToLower(r))
	}
	return out
}

func indexRunes(s, sub []rune) int {
	if len(sub) == 0 {
		return 0
	}
outer:
	for i := 0; i+len(sub) <= len(s); i++ {
		for j := range sub {
			if s[i+j] != sub[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}
