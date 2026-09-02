package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/reflow/wordwrap"

	"github.com/hidekingerz/mado/internal/filetree"
	"github.com/hidekingerz/mado/internal/search"
	"github.com/hidekingerz/mado/internal/termsafe"
)

// searchHeaderRows is the title, the prompt and a blank line above the
// result list in the search panel.
const searchHeaderRows = 3

// searchState is the search panel: a query typed at a prompt and the
// files (or lines) matching it, listed in the content pane. Each
// change to the query starts a new search in the background; results
// carry the generation they answer, so a slow search that finishes
// after the query moved on is dropped rather than shown.
type searchState struct {
	open      bool
	target    search.Target
	query     string
	results   []search.Match
	truncated bool
	err       string
	cursor    int
	off       int
	gen       int  // generation of the query the results belong to, or in flight
	pending   bool // the search for gen has not answered yet
	cancel    context.CancelFunc
	prevFocus focusArea // where to return when the panel closes
}

// searchResultMsg delivers the outcome of one background search.
type searchResultMsg struct {
	gen int
	res search.Result
}

// searchOptions applies the sidebar's file filter and the configured
// exclusions to a search.
func (m *Model) searchOptions() search.Options {
	return search.Options{
		Tree:    m.treeOpts,
		Exclude: m.cfg.Search.Exclude,
	}
}

// openSearch shows the panel for target. A query left from last time
// is searched again, so the list never shows results from before the
// tree changed.
func (m *Model) openSearch(target search.Target) tea.Cmd {
	if !m.search.open {
		m.search.open = true
		m.search.prevFocus = m.focus
		m.showHelp = false
	}
	m.search.target = target
	return m.runSearch()
}

// closeSearch hides the panel and returns the focus to where it was.
func (m *Model) closeSearch() {
	if !m.search.open {
		return
	}
	m.search.open = false
	m.cancelSearch()
	m.restoreFocus(m.search.prevFocus)
}

func (m *Model) cancelSearch() {
	if m.search.cancel != nil {
		m.search.cancel()
		m.search.cancel = nil
	}
	m.search.pending = false
}

// runSearch starts a search for the current query and target,
// abandoning any search still running. The previous results stay on
// screen until the new ones arrive, so typing does not flicker.
func (m *Model) runSearch() tea.Cmd {
	m.cancelSearch()
	m.search.gen++
	m.search.cursor, m.search.off = 0, 0
	m.search.err = ""
	if m.search.query == "" {
		m.search.results, m.search.truncated = nil, false
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.search.cancel = cancel
	m.search.pending = true
	gen, root := m.search.gen, m.root.Path
	target, query, opts := m.search.target, m.search.query, m.searchOptions()
	return func() tea.Msg {
		res := search.Run(ctx, root, target, query, opts)
		if ctx.Err() != nil {
			return nil
		}
		return searchResultMsg{gen: gen, res: res}
	}
}

// handleSearchResult shows the results of a search, unless the query
// has moved on since it started.
func (m *Model) handleSearchResult(msg searchResultMsg) {
	if msg.gen != m.search.gen {
		return
	}
	m.search.pending = false
	m.search.results = msg.res.Matches
	m.search.truncated = msg.res.Truncated
	if msg.res.Err != nil {
		m.search.err = msg.res.Err.Error()
	}
	m.clampSearch()
}

// handleSearchKey edits the query or moves through the results. Keys
// that type a character go to the query, so only keys that cannot be
// typed act on the panel.
func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace {
		switch {
		case key.Matches(msg, m.keys.Search):
			return m, m.openSearch(search.Names)
		case key.Matches(msg, m.keys.SearchContent):
			return m, m.openSearch(search.Contents)
		}
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.closeSearch()
	case tea.KeyEnter:
		m.openSearchResult()
	case tea.KeyUp, tea.KeyCtrlP:
		m.moveSearchCursor(-1)
	case tea.KeyDown, tea.KeyCtrlN:
		m.moveSearchCursor(1)
	case tea.KeyPgUp:
		m.moveSearchCursor(-maxInt(m.searchListHeight(), 1))
	case tea.KeyPgDown:
		m.moveSearchCursor(maxInt(m.searchListHeight(), 1))
	case tea.KeyHome:
		m.moveSearchCursor(-len(m.search.results))
	case tea.KeyEnd:
		m.moveSearchCursor(len(m.search.results))
	case tea.KeyTab:
		return m, m.openSearch(m.search.target.Toggle())
	default:
		if next, ok := editPrompt(m.search.query, msg); ok && next != m.search.query {
			m.search.query = next
			return m, m.runSearch()
		}
	}
	return m, nil
}

// editPrompt applies one editing key to a prompt's text: backspace
// removes a rune, ctrl+w a word, ctrl+u everything; space and typed
// characters (control characters dropped, alt-combinations ignored)
// are appended. It reports whether msg was such a key, so callers can
// treat every other key as a command.
func editPrompt(s string, msg tea.KeyMsg) (string, bool) {
	switch msg.Type {
	case tea.KeyBackspace:
		if s != "" {
			_, size := utf8.DecodeLastRuneInString(s)
			s = s[:len(s)-size]
		}
		return s, true
	case tea.KeyCtrlW:
		q := strings.TrimRightFunc(s, unicode.IsSpace)
		if i := strings.LastIndexFunc(q, unicode.IsSpace); i >= 0 {
			q = q[:i+1]
		} else {
			q = ""
		}
		return q, true
	case tea.KeyCtrlU:
		return "", true
	case tea.KeySpace:
		return s + " ", true
	case tea.KeyRunes:
		if msg.Alt {
			return s, true
		}
		var b strings.Builder
		for _, r := range msg.Runes {
			// A pasted newline or escape has no place in a value.
			if !unicode.IsControl(r) {
				b.WriteRune(r)
			}
		}
		if b.Len() > 0 {
			s += b.String()
		}
		return s, true
	}
	return s, false
}

// handleSearchMouse scrolls or picks from the result list. Clicks
// outside the panel close it and act as they would without it.
func (m Model) handleSearchMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	inSidebar := m.sidebar && msg.X < m.sidebarWidth()
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inSidebar {
			m.scrollTree(-wheelStep)
		} else {
			m.scrollSearch(-wheelStep)
		}
	case tea.MouseButtonWheelDown:
		if inSidebar {
			m.scrollTree(wheelStep)
		} else {
			m.scrollSearch(wheelStep)
		}
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		switch {
		case msg.Y < tabBarHeight:
			m.closeSearch()
			m.clickTabBar(msg.X)
		case inSidebar:
			m.closeSearch()
			m.clickSidebar(msg.X, msg.Y)
		default:
			row := msg.Y - tabBarHeight - 1 - searchHeaderRows
			idx := m.search.off + row
			if row >= 0 && idx < len(m.search.results) {
				m.search.cursor = idx
				m.openSearchResult()
			}
		}
	}
	return m, nil
}

// openSearchResult opens the selected match in a tab and closes the
// panel. A content match also scrolls the tab to its line.
func (m *Model) openSearchResult() {
	s := m.search
	if s.cursor < 0 || s.cursor >= len(s.results) {
		return
	}
	r := s.results[s.cursor]
	m.closeSearch()
	if err := m.openFile(r.Path); err != nil {
		return
	}
	m.focus = focusContent
	if r.Line > 0 {
		m.scrollToMatch(m.activeTab(), r, s.query)
	}
}

func (m *Model) searchListHeight() int {
	return maxInt(m.contentInnerHeight()-searchHeaderRows, 0)
}

func (m *Model) moveSearchCursor(delta int) {
	m.search.cursor += delta
	m.clampSearch()
	h := m.searchListHeight()
	if h <= 0 {
		return
	}
	if m.search.cursor < m.search.off {
		m.search.off = m.search.cursor
	}
	if m.search.cursor >= m.search.off+h {
		m.search.off = m.search.cursor - h + 1
	}
}

func (m *Model) scrollSearch(delta int) {
	m.search.off += delta
	m.clampSearch()
}

func (m *Model) clampSearch() {
	n := len(m.search.results)
	if m.search.cursor >= n {
		m.search.cursor = n - 1
	}
	if m.search.cursor < 0 {
		m.search.cursor = 0
	}
	maxOff := n - m.searchListHeight()
	if maxOff < 0 {
		maxOff = 0
	}
	if m.search.off > maxOff {
		m.search.off = maxOff
	}
	if m.search.off < 0 {
		m.search.off = 0
	}
}

// ── jumping to a content match ──────────────────────────────────────

// scrollToMatch puts the line of a content match at the top of the
// tab's viewport. Source-rendered text maps a file line to a viewport
// row exactly; a rendered markdown document is searched for the
// match's text instead, since rendering rewraps and drops syntax.
// Neither can fail visibly: when the row cannot be found the tab is
// left where it is.
func (m *Model) scrollToMatch(t *tab, r search.Match, query string) {
	if t == nil || t.img != nil || t.binary || r.Line < 1 {
		return
	}
	var row int
	if m.mode == modeSource || !filetree.IsMarkdown(t.path) {
		row = sourceRow(t.raw, r.Line, t.vp.Width-2, m.showLineNums)
	} else {
		row = readerRow(t.raw, t.content, r.Line, query)
	}
	if row >= 0 {
		t.vp.SetYOffset(row)
	}
}

// sourceRow returns the viewport row where file line `line` (1-based)
// starts when raw is shown as source at width, wrapped the way
// ensureRendered wraps it. Highlighting adds only escape sequences,
// which take no width, so wrapping the plain text lands on the same
// rows.
func sourceRow(raw string, line, width int, lineNums bool) int {
	lines := strings.Split(raw, "\n")
	if line > len(lines) {
		return -1
	}
	if width <= 0 {
		return line - 1
	}
	textW := width
	if lineNums {
		n := len(lines)
		if n > 1 && lines[n-1] == "" {
			n--
		}
		if w := width - len(strconv.Itoa(n)) - 1; w >= 1 {
			textW = w
		}
	}
	row := 0
	for _, l := range lines[:line-1] {
		row += strings.Count(wordwrap.String(l, textW), "\n") + 1
	}
	return row
}

// readerRow finds the viewport row of a content match in a rendered
// markdown document. The match is the k-th occurrence of query in the
// file up to its line; the k-th occurrence in the rendered text is
// taken to be the same one. When the rendered text has fewer
// occurrences (the match was in syntax the renderer dropped), the row
// is estimated from the line's position in the file.
func readerRow(raw, rendered string, line int, query string) int {
	rawLines := strings.Split(raw, "\n")
	if line > len(rawLines) || query == "" {
		return -1
	}
	k := 0
	for _, l := range rawLines[:line] {
		k += countMatches(l, query)
	}
	rows := strings.Split(rendered, "\n")
	seen := 0
	for i, r := range rows {
		seen += countMatches(ansi.Strip(r), query)
		if seen >= k {
			return i
		}
	}
	return len(rows) * (line - 1) / maxInt(len(rawLines), 1)
}

// countMatches counts the non-overlapping occurrences of query in s.
func countMatches(s, query string) int {
	n := 0
	for {
		_, end := search.Find(s, query)
		if end < 0 {
			return n
		}
		n++
		s = s[end:]
	}
}

// ── rendering ───────────────────────────────────────────────────────

// searchSummary describes the state of the result list in a few words.
func (m Model) searchSummary() string {
	s := m.search
	switch {
	case s.query == "":
		return ""
	case s.pending && len(s.results) == 0:
		return "searching…"
	case s.truncated:
		return fmt.Sprintf("%d+ matches", len(s.results))
	case len(s.results) == 0:
		return "no matches"
	case len(s.results) == 1:
		return "1 match"
	default:
		return fmt.Sprintf("%d matches", len(s.results))
	}
}

func (m Model) renderSearch(w, h int) string {
	s := m.search
	rows := make([]string, 0, h)

	title := m.styles.accent.Bold(true).Render("Search " + s.target.String())
	if sum := m.searchSummary(); sum != "" {
		title += "  " + m.styles.dimmed.Render(sum)
	}
	hint := m.styles.dimmed.Render("tab: " + s.target.Toggle().String())
	gap := w - 1 - lipgloss.Width(title) - lipgloss.Width(hint)
	if gap < 1 {
		hint = ""
		gap = 0
	}
	rows = append(rows, clip(" "+title+strings.Repeat(" ", gap)+hint, w))

	prompt := " " + m.styles.accent.Render("> ")
	query := truncate(termsafe.String(s.query), maxInt(w-lipgloss.Width(prompt)-1, 0))
	rows = append(rows, prompt+query+m.styles.selection.Render(" "))
	rows = append(rows, "")

	listH := h - searchHeaderRows
	if s.query == "" && listH > 0 {
		what := "file names"
		if s.target == search.Contents {
			what = "file contents"
		}
		rows = append(rows, clip(m.styles.dimmed.Render(" Type to search "+what+". tab switches to "+s.target.Toggle().String()+", enter opens, esc closes."), w))
		listH--
	} else if s.err != "" && listH > 0 {
		rows = append(rows, clip(m.styles.dimmed.Render(" ⚠ "+termsafe.String(s.err)), w))
		listH--
	}
	for i := 0; i < listH; i++ {
		idx := s.off + i
		if idx >= len(s.results) {
			rows = append(rows, "")
			continue
		}
		rows = append(rows, m.renderSearchRow(s.results[idx], w, idx == s.cursor))
	}
	return strings.Join(rows, "\n")
}

// renderSearchRow lays out one match in w columns: the file's path for
// a name match; path, line number and the matching text for a content
// match. The selected row is drawn in the selection style, others get
// the hit itself emphasized.
func (m Model) renderSearchRow(r search.Match, w int, selected bool) string {
	// Paths and lines come from disk: display them, do not obey them.
	rel := termsafe.String(r.Rel)
	if r.Line == 0 {
		text := truncateLeft(rel, w-1)
		if selected {
			return m.styles.selection.Render(padLine(" "+text, w))
		}
		return " " + m.emphasize(text, m.search.query, m.styles.file)
	}
	loc := truncateLeft(rel+":"+strconv.Itoa(r.Line), maxInt(w/2, 1))
	snipW := w - 1 - lipgloss.Width(loc) - 2
	snippet := searchSnippet(termsafe.String(r.Text), m.search.query, snipW)
	if selected {
		return m.styles.selection.Render(padLine(" "+loc+"  "+snippet, w))
	}
	styledLoc := m.styles.dir.Render(loc)
	if i := strings.LastIndex(loc, ":"); i >= 0 {
		styledLoc = m.styles.dir.Render(loc[:i]) + m.styles.dimmed.Render(loc[i:])
	}
	return " " + styledLoc + "  " + m.emphasize(snippet, m.search.query, lipgloss.NewStyle())
}

// truncateLeft fits plain text into w columns by cutting from the
// left, so the end of a path — the file name — stays readable.
func truncateLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > w {
		runes = runes[1:]
	}
	return "…" + string(runes)
}

// clip cuts an already styled string to w columns, keeping its escape
// sequences whole. truncate is for plain text.
func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// emphasize renders text in base with the first hit of query in the
// accent color.
func (m Model) emphasize(text, query string, base lipgloss.Style) string {
	start, end := search.Find(text, query)
	if start < 0 {
		return base.Render(text)
	}
	return base.Render(text[:start]) + m.styles.accent.Bold(true).Render(text[start:end]) + base.Render(text[end:])
}

// searchSnippet fits a matching line into w columns, keeping the hit
// in view: runs of whitespace collapse to one space, and a line whose
// hit sits far to the right is cut from the left with an ellipsis.
func searchSnippet(line, query string, w int) string {
	if w <= 0 {
		return ""
	}
	line = strings.Join(strings.Fields(line), " ")
	start, _ := search.Find(line, query)
	if start > 0 && lipgloss.Width(line[:start]) > w/2 {
		runes := []rune(line[:start])
		keep := w / 3
		if keep > len(runes) {
			keep = len(runes)
		}
		line = "…" + string(runes[len(runes)-keep:]) + line[start:]
	}
	return truncate(line, w)
}
