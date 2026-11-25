package ui

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/hidekingerz/mado/internal/filetree"
	"github.com/hidekingerz/mado/internal/termsafe"
)

// linksHeaderRows is the title and a blank line above the list in the
// links panel.
const linksHeaderRows = 2

// link is one local link of a markdown file: where it points on disk
// and, when it names one, the heading to scroll to.
type link struct {
	text    string // the link's own text; "" when it has none
	path    string // absolute path of the target; the file itself for a bare #anchor
	frag    string // heading anchor, without the #
	missing bool   // nothing to open at path
}

// linksState is the links panel: the local links of the active tab,
// listed in the content pane to pick one from.
type linksState struct {
	open      bool
	items     []link
	cursor    int
	off       int
	prevFocus focusArea // where to return when the panel closes
}

// urlScheme matches the start of a link that leaves the disk: http:,
// mailto: and the like. Two characters at least, so a Windows drive
// letter is still a path.
var urlScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]+:`)

// splitDest takes a link destination apart into a file path and a
// heading anchor. It reports false for anything that is not a file on
// this machine: a URL, or nothing at all.
func splitDest(dest string) (path, frag string, ok bool) {
	dest = strings.TrimSpace(dest)
	if len(dest) >= 7 && strings.EqualFold(dest[:7], "file://") {
		u, err := url.Parse(dest)
		if err != nil || u.Path == "" {
			return "", "", false
		}
		return u.Path, u.Fragment, true
	}
	if urlScheme.MatchString(dest) || strings.HasPrefix(dest, "//") {
		return "", "", false
	}
	path, frag, _ = strings.Cut(dest, "#")
	// A query means something to a web server, not to a file.
	path, _, _ = strings.Cut(path, "?")
	if p, err := url.PathUnescape(path); err == nil {
		path = p
	}
	if f, err := url.PathUnescape(frag); err == nil {
		frag = f
	}
	return path, frag, path != "" || frag != ""
}

// resolveLink finds the file a link path names, as seen from the file
// at from. A relative path starts at that file's directory. A path
// with a leading slash is tried as it is and then under root, which
// is what such a link means in a repository. A directory stands for
// its README or index, and a name without an extension for the
// markdown file of that name. It reports false when there is nothing
// to open, with the path the link was pointing at.
func resolveLink(p, from, root string) (string, bool) {
	p = filepath.FromSlash(p)
	var candidates []string
	switch {
	case strings.HasPrefix(p, string(filepath.Separator)):
		candidates = []string{filepath.Join(root, p), filepath.Clean(p)}
	case filepath.IsAbs(p):
		candidates = []string{filepath.Clean(p)}
	default:
		candidates = []string{filepath.Join(filepath.Dir(from), p)}
	}
	isFile := func(path string) bool {
		st, err := os.Stat(path)
		return err == nil && !st.IsDir()
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil {
			if !st.IsDir() {
				return c, true
			}
			for _, name := range []string{"README.md", "index.md"} {
				if f := filepath.Join(c, name); isFile(f) {
					return f, true
				}
			}
			continue
		}
		if filepath.Ext(c) == "" && isFile(c+".md") {
			return c + ".md", true
		}
	}
	return candidates[0], false
}

// parseMarkdown reads raw into goldmark's tree.
func parseMarkdown(raw string) (ast.Node, []byte) {
	src := []byte(raw)
	return goldmark.New().Parser().Parse(text.NewReader(src)), src
}

// nodeText is the plain text under n, as a reader sees it: no markup.
func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			b.Write(c.Value(src))
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(b.String()), " ")
}

// collectLinks lists the local links in the markdown raw of the file
// at from, in document order. Images count: a local image opens as a
// preview tab. A target named twice is listed once.
func collectLinks(raw, from, root string) []link {
	doc, src := parseMarkdown(raw)
	var links []link
	seen := map[string]bool{}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var dest []byte
		switch n := n.(type) {
		case *ast.Link:
			dest = n.Destination
		case *ast.Image:
			dest = n.Destination
		default:
			return ast.WalkContinue, nil
		}
		p, frag, ok := splitDest(string(dest))
		if !ok {
			return ast.WalkContinue, nil
		}
		l := link{text: nodeText(n, src), path: from, frag: frag}
		if p != "" {
			path, found := resolveLink(p, from, root)
			l.path, l.missing = path, !found
		}
		if id := l.path + "#" + l.frag; !seen[id] {
			seen[id] = true
			links = append(links, l)
		}
		return ast.WalkContinue, nil
	})
	return links
}

// headingSlug turns a heading into the anchor that links to it, the
// way GitHub does: lower case, punctuation dropped, spaces to hyphens.
func headingSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// headingLine finds the heading that anchor names in raw. It returns
// the heading's 1-based line and its text, or 0 when there is none. A
// heading that repeats an earlier one is anchor-1, anchor-2, ….
func headingLine(raw, anchor string) (int, string) {
	anchor = strings.ToLower(anchor)
	doc, src := parseMarkdown(raw)
	used := map[string]int{}
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		h, ok := n.(*ast.Heading)
		if !ok {
			continue
		}
		title := nodeText(h, src)
		slug := headingSlug(title)
		count := used[slug]
		used[slug]++
		if count > 0 {
			slug = fmt.Sprintf("%s-%d", slug, count)
		}
		if slug != anchor || h.Lines().Len() == 0 {
			continue
		}
		return strings.Count(raw[:h.Lines().At(0).Start], "\n") + 1, title
	}
	return 0, ""
}

// scrollToHeading puts the heading anchor names at the top of the
// tab's viewport. A file without that heading is left where it is.
func (m *Model) scrollToHeading(t *tab, anchor string) {
	if t == nil || t.img != nil || t.binary || anchor == "" || !filetree.IsMarkdown(t.path) {
		return
	}
	line, title := headingLine(t.raw, anchor)
	if line == 0 {
		return
	}
	var row int
	if m.mode == modeSource {
		row = sourceRow(t.raw, line, t.vp.Width()-2, m.showLineNums)
	} else {
		row = readerRow(t.raw, t.content, line, title)
		// A heading written with markup inside does not appear in the
		// source as the reader shows it, so there is no occurrence to
		// count: fall back to where the line sits in the file.
		lines := strings.Split(t.raw, "\n")
		if countMatches(lines[line-1], title) == 0 {
			row = strings.Count(t.content, "\n") * (line - 1) / maxInt(len(lines), 1)
		}
	}
	if row >= 0 {
		t.vp.SetYOffset(row)
	}
}

// ── the panel ───────────────────────────────────────────────────────

// tabLinks lists the local links of a tab; none for anything but
// markdown.
func (m *Model) tabLinks(t *tab) []link {
	if t == nil || t.img != nil || t.binary || !filetree.IsMarkdown(t.path) {
		return nil
	}
	return collectLinks(t.raw, t.path, m.root.Path)
}

// openLinks shows the active tab's local links. With none to show it
// says so instead of opening an empty panel.
func (m *Model) openLinks() {
	items := m.tabLinks(m.activeTab())
	if len(items) == 0 {
		m.notice = m.text.NoLinks
		return
	}
	m.showHelp = false
	m.links = linksState{open: true, items: items, prevFocus: m.focus}
}

// closeLinks hides the panel and returns the focus to where it was.
func (m *Model) closeLinks() {
	if !m.links.open {
		return
	}
	m.links.open = false
	m.restoreFocus(m.links.prevFocus)
}

// refreshLinks re-reads the links after the file changed on disk. A
// file left without any closes the panel.
func (m *Model) refreshLinks() {
	if !m.links.open {
		return
	}
	m.links.items = m.tabLinks(m.activeTab())
	if len(m.links.items) == 0 {
		m.closeLinks()
		return
	}
	m.clampLinks()
}

func (m Model) handleLinksKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	page := maxInt(m.linksListHeight(), 1)
	switch s := msg.String(); {
	case key.Matches(msg, k.Quit):
		return m, m.quit()
	case s == "esc" || key.Matches(msg, k.Back) || key.Matches(msg, k.Links):
		m.closeLinks()
	case s == "enter" || key.Matches(msg, k.Open):
		m.openLink()
	case s == "up" || s == "ctrl+p" || key.Matches(msg, k.Up):
		m.moveLinksCursor(-1)
	case s == "down" || s == "ctrl+n" || key.Matches(msg, k.Down):
		m.moveLinksCursor(1)
	case key.Matches(msg, k.HalfPageUp):
		m.moveLinksCursor(-page)
	case key.Matches(msg, k.HalfPageDown):
		m.moveLinksCursor(page)
	case key.Matches(msg, k.Top):
		m.moveLinksCursor(-len(m.links.items))
	case key.Matches(msg, k.Bottom):
		m.moveLinksCursor(len(m.links.items))
	}
	return m, nil
}

// handleLinksMouse scrolls or picks from the list. Clicks outside the
// panel close it and act as they would without it.
func (m Model) handleLinksMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	at := msg.Mouse()
	inSidebar := m.sidebar && at.X < m.sidebarWidth()
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		if step, ok := wheelStepOf(msg); ok {
			if inSidebar {
				m.scrollTree(step)
			} else {
				m.links.off += step
				m.clampLinks()
			}
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		switch {
		case at.Y < tabBarHeight:
			m.closeLinks()
			m.clickTabBar(at.X)
		case inSidebar:
			m.closeLinks()
			m.clickSidebar(at.X, at.Y)
		default:
			row := at.Y - tabBarHeight - 1 - linksHeaderRows
			if idx := m.links.off + row; row >= 0 && idx < len(m.links.items) {
				m.links.cursor = idx
				m.openLink()
			}
		}
	}
	return m, nil
}

// openLink follows the selected link: its file opens in a tab (or the
// tab it already has comes forward) and scrolls to the heading the
// link names. A link to nothing leaves the panel open, with the
// reason in the status bar.
func (m *Model) openLink() {
	s := m.links
	if s.cursor < 0 || s.cursor >= len(s.items) {
		return
	}
	l := s.items[s.cursor]
	if err := m.openFile(l.path); err != nil {
		return
	}
	m.closeLinks()
	m.focus = focusContent
	m.scrollToHeading(m.activeTab(), l.frag)
}

func (m *Model) linksListHeight() int {
	return maxInt(m.contentInnerHeight()-linksHeaderRows, 0)
}

func (m *Model) moveLinksCursor(delta int) {
	m.links.cursor += delta
	m.clampLinks()
	h := m.linksListHeight()
	if h <= 0 {
		return
	}
	if m.links.cursor < m.links.off {
		m.links.off = m.links.cursor
	}
	if m.links.cursor >= m.links.off+h {
		m.links.off = m.links.cursor - h + 1
	}
}

func (m *Model) clampLinks() {
	n := len(m.links.items)
	if m.links.cursor >= n {
		m.links.cursor = n - 1
	}
	if m.links.cursor < 0 {
		m.links.cursor = 0
	}
	maxOff := maxInt(n-m.linksListHeight(), 0)
	if m.links.off > maxOff {
		m.links.off = maxOff
	}
	if m.links.off < 0 {
		m.links.off = 0
	}
}

// ── rendering ───────────────────────────────────────────────────────

// linkTarget names where a link goes: the file as the status bar would
// name it, and the anchor. A link within the open file is the anchor
// alone.
func (m Model) linkTarget(l link) string {
	target := ""
	if t := m.activeTab(); t == nil || t.path != l.path {
		target = filepath.ToSlash(m.displayPath(l.path))
	}
	if l.frag != "" {
		target += "#" + l.frag
	}
	return target
}

func (m Model) renderLinks(w, h int) string {
	s := m.links
	rows := make([]string, 0, h)

	count := fmt.Sprintf(m.text.LinksF, len(s.items))
	if len(s.items) == 1 {
		count = m.text.OneLink
	}
	title := m.styles.accent.Bold(true).Render(m.text.LinksTitle) + "  " + m.styles.dimmed.Render(count)
	rows = append(rows, clip(" "+title, w), "")

	for i := 0; i < h-linksHeaderRows; i++ {
		idx := s.off + i
		if idx >= len(s.items) {
			rows = append(rows, "")
			continue
		}
		rows = append(rows, m.renderLinkRow(s.items[idx], w, idx == s.cursor))
	}
	return strings.Join(rows, "\n")
}

// renderLinkRow lays out one link in w columns: its text, then where
// it goes. A link whose text is its target is shown once.
func (m Model) renderLinkRow(l link, w int, selected bool) string {
	// Both come from the file: display them, do not obey them.
	label := termsafe.String(l.text)
	target := termsafe.String(m.linkTarget(l))
	if l.missing {
		target += "  (" + m.text.LinkMissing + ")"
	}
	if label == "" || label == target {
		label, target = target, ""
	}
	// A long text gives way before the target does.
	if room := w - 3 - lipgloss.Width(target); target != "" && lipgloss.Width(label) > room {
		label = truncate(label, maxInt(room, w/3))
	}
	if selected {
		line := " " + label
		if target != "" {
			line += "  " + target
		}
		return m.styles.selection.Render(padLine(clip(line, w), w))
	}
	style := m.styles.file
	if l.missing {
		style = m.styles.dimmed
	}
	line := " " + style.Render(label)
	if target != "" {
		line += "  " + m.styles.dimmed.Render(target)
	}
	return clip(line, w)
}
