// Package ui implements mado's terminal user interface: a file tree
// sidebar, a tab bar for multiple open files, and a markdown viewport.
package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	gstyles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/termenv"

	"github.com/hidekingerz/mado/internal/config"
	"github.com/hidekingerz/mado/internal/filetree"
)

type focusArea int

const (
	focusSidebar focusArea = iota
	focusContent
)

// viewMode selects how a file's content is displayed.
type viewMode int

const (
	// modeReader renders markdown as a clean document (no ##/### syntax
	// markers; heading levels are shown via color and underline).
	modeReader viewMode = iota
	// modeSource shows the raw markdown, for checking the syntax itself.
	modeSource
)

func (v viewMode) String() string {
	if v == modeSource {
		return "SOURCE"
	}
	return "READER"
}

const (
	tabBarHeight    = 1
	statusBarHeight = 1
	wheelStep       = 3
)

// tab is one open file.
type tab struct {
	path         string
	title        string
	raw          string
	vp           viewport.Model
	rendered     int // viewport width the content was last rendered at; 0 = never
	renderedMode viewMode
	renderErr    string
}

// tabRegion records where a tab was drawn in the tab bar, for mouse
// hit-testing. closeX is the column of the ✕ glyph.
type tabRegion struct {
	start, end int
	closeX     int
}

// Model is the root bubbletea model.
type Model struct {
	cfg  config.Config
	keys keyMap

	width  int
	height int

	root        *filetree.Node
	treeOpts    filetree.Options
	items       []filetree.Item
	cursor      int
	treeOff     int
	sidebar     bool
	focus       focusArea
	mode        viewMode
	style       string // resolved glamour style ("auto" already decided)
	sourceStyle string // chroma style for source mode; "" = no highlighting
	formatter   string // chroma formatter; pinned at startup, "" = no color
	showHelp    bool
	statusMsg   string

	tabs       []*tab
	active     int
	tabRegions []tabRegion

	styles styles
}

type styles struct {
	accent      lipgloss.Style
	border      lipgloss.Style
	selection   lipgloss.Style
	dimmed      lipgloss.Style
	status      lipgloss.Style
	tabActive   lipgloss.Style
	tabInactive lipgloss.Style
}

// New creates the root model. initialFiles are opened as tabs.
func New(cfg config.Config, rootDir string, initialFiles []string) (Model, error) {
	opts := filetree.Options{
		ShowAllFiles: cfg.Sidebar.ShowAllFiles,
		ShowHidden:   cfg.Sidebar.ShowHidden,
	}
	root, err := filetree.New(rootDir, opts)
	if err != nil {
		return Model{}, err
	}

	mode := modeReader
	if cfg.Theme.DefaultMode == "source" {
		mode = modeSource
	}
	accent := lipgloss.Color(cfg.Theme.AccentColor)
	style := resolveStyle(cfg.Theme.Style)
	m := Model{
		cfg:         cfg,
		keys:        newKeyMap(cfg.Keys),
		root:        root,
		treeOpts:    opts,
		sidebar:     true,
		focus:       focusSidebar,
		mode:        mode,
		style:       style,
		sourceStyle: chromaStyleName(style, cfg.Theme.SourceStyle, termenv.HasDarkBackground()),
		formatter:   chromaFormatterName(),
		styles: styles{
			accent:    lipgloss.NewStyle().Foreground(accent),
			border:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Theme.BorderColor)),
			selection: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Theme.SelectionFg)).Background(lipgloss.Color(cfg.Theme.SelectionBg)).Bold(true),
			dimmed:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
			status:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Theme.StatusFg)).Background(lipgloss.Color(cfg.Theme.StatusBg)),
			tabActive: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Theme.SelectionFg)).Background(accent).Bold(true),
			tabInactive: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Theme.StatusFg)).
				Background(lipgloss.Color(cfg.Theme.StatusBg)),
		},
	}
	m.items = filetree.Flatten(m.root)
	for _, f := range initialFiles {
		m.openFile(f)
	}
	if len(m.tabs) > 0 {
		m.focus = focusContent
	}
	return m, nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layoutTabs()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	switch {
	case key.Matches(msg, k.Quit):
		return m, tea.Quit
	case key.Matches(msg, k.Help):
		m.showHelp = !m.showHelp
		return m, nil
	}
	if m.showHelp {
		// Any other key dismisses the help overlay.
		m.showHelp = false
		return m, nil
	}

	switch {
	case key.Matches(msg, k.ToggleSidebar):
		m.sidebar = !m.sidebar
		if !m.sidebar {
			m.focus = focusContent
		} else if len(m.tabs) == 0 {
			m.focus = focusSidebar
		}
		m.layoutTabs()
	case key.Matches(msg, k.Back):
		if m.sidebar {
			m.focus = focusSidebar
		}
	case key.Matches(msg, k.Up):
		if m.focus == focusSidebar {
			m.moveCursor(-1)
		} else if t := m.activeTab(); t != nil {
			t.vp.ScrollUp(1)
		}
	case key.Matches(msg, k.Down):
		if m.focus == focusSidebar {
			m.moveCursor(1)
		} else if t := m.activeTab(); t != nil {
			t.vp.ScrollDown(1)
		}
	case key.Matches(msg, k.Open):
		if m.focus == focusSidebar {
			m.openSelected()
		}
	case key.Matches(msg, k.CloseTab):
		m.closeTab(m.active)
	case key.Matches(msg, k.NextTab):
		m.switchTab(1)
	case key.Matches(msg, k.PrevTab):
		m.switchTab(-1)
	case key.Matches(msg, k.HalfPageDown):
		if t := m.activeTab(); t != nil {
			t.vp.HalfPageDown()
		}
	case key.Matches(msg, k.HalfPageUp):
		if t := m.activeTab(); t != nil {
			t.vp.HalfPageUp()
		}
	case key.Matches(msg, k.Top):
		if m.focus == focusSidebar {
			m.cursor = 0
			m.scrollCursorIntoView()
		} else if t := m.activeTab(); t != nil {
			t.vp.GotoTop()
		}
	case key.Matches(msg, k.Bottom):
		if m.focus == focusSidebar {
			m.cursor = len(m.items) - 1
			m.scrollCursorIntoView()
		} else if t := m.activeTab(); t != nil {
			t.vp.GotoBottom()
		}
	case key.Matches(msg, k.Reload):
		m.reload()
	case key.Matches(msg, k.ToggleMode):
		if m.mode == modeReader {
			m.mode = modeSource
		} else {
			m.mode = modeReader
		}
		m.ensureRendered(m.activeTab())
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	inSidebar := m.sidebar && msg.X < m.sidebarWidth()

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inSidebar {
			m.scrollTree(-wheelStep)
		} else if t := m.activeTab(); t != nil {
			t.vp.ScrollUp(wheelStep)
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if inSidebar {
			m.scrollTree(wheelStep)
		} else if t := m.activeTab(); t != nil {
			t.vp.ScrollDown(wheelStep)
		}
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		switch {
		case msg.Y < tabBarHeight:
			m.clickTabBar(msg.X)
		case inSidebar:
			m.clickSidebar(msg.X, msg.Y)
		default:
			m.focus = focusContent
		}
	}
	return m, nil
}

// clickTabBar activates or closes the tab under column x.
func (m *Model) clickTabBar(x int) {
	for i, r := range m.tabRegions {
		if x < r.start || x >= r.end {
			continue
		}
		if x == r.closeX {
			m.closeTab(i)
		} else {
			m.active = i
			m.focus = focusContent
			m.ensureRendered(m.activeTab())
		}
		return
	}
}

// clickSidebar selects the tree row under (x, y); a second effect
// follows immediately: directories toggle, files open.
func (m *Model) clickSidebar(x, y int) {
	m.focus = focusSidebar
	innerTop := tabBarHeight + 1 // below the sidebar's top border
	row := y - innerTop
	if row < 0 || row >= m.treeInnerHeight() {
		return
	}
	idx := m.treeOff + row
	if idx < 0 || idx >= len(m.items) {
		return
	}
	m.cursor = idx
	m.openSelected()
}

func (m *Model) openSelected() {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return
	}
	n := m.items[m.cursor].Node
	if n.IsDir {
		if err := n.Toggle(m.treeOpts); err != nil {
			m.statusMsg = err.Error()
		}
		m.items = filetree.Flatten(m.root)
		m.clampTree()
		return
	}
	m.openFile(n.Path)
	m.focus = focusContent
}

// openFile opens path in a new tab, or activates the existing tab.
func (m *Model) openFile(path string) {
	for i, t := range m.tabs {
		if t.path == path {
			m.active = i
			m.ensureRendered(t)
			return
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		m.statusMsg = err.Error()
		return
	}
	t := &tab{
		path:  path,
		title: baseName(path),
		raw:   string(data),
	}
	t.vp = viewport.New(m.contentInnerWidth(), m.contentInnerHeight())
	m.tabs = append(m.tabs, t)
	m.active = len(m.tabs) - 1
	m.layoutTabs()
	m.ensureRendered(t)
	m.statusMsg = ""
}

func (m *Model) closeTab(i int) {
	if i < 0 || i >= len(m.tabs) {
		return
	}
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
	if len(m.tabs) == 0 {
		m.focus = focusSidebar
		if !m.sidebar {
			m.sidebar = true
		}
	}
	m.layoutTabs()
}

func (m *Model) switchTab(delta int) {
	if len(m.tabs) < 2 {
		return
	}
	m.active = (m.active + delta + len(m.tabs)) % len(m.tabs)
	m.focus = focusContent
	m.ensureRendered(m.activeTab())
}

func (m *Model) activeTab() *tab {
	if m.active < 0 || m.active >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.active]
}

func (m *Model) reload() {
	if err := m.root.Reload(m.treeOpts); err != nil {
		m.statusMsg = err.Error()
	}
	m.items = filetree.Flatten(m.root)
	m.clampTree()
	if t := m.activeTab(); t != nil {
		if data, err := os.ReadFile(t.path); err == nil {
			t.raw = string(data)
			t.rendered = 0
			m.ensureRendered(t)
		} else {
			m.statusMsg = err.Error()
		}
	}
}

// ensureRendered (re-)renders a tab's content at the current viewport
// width and view mode if needed.
func (m *Model) ensureRendered(t *tab) {
	if t == nil {
		return
	}
	w := m.contentInnerWidth()
	if w <= 0 || (t.rendered == w && t.renderedMode == m.mode) {
		return
	}
	t.vp.Width = w
	t.vp.Height = m.contentInnerHeight()
	t.renderErr = ""

	content := t.raw
	if m.mode == modeSource || !filetree.IsMarkdown(t.path) {
		if hl, err := highlightSource(t.raw, t.path, m.sourceStyle, m.formatter); err == nil {
			content = hl
		}
		content = wordwrap.String(content, w-2)
	} else {
		r, err := newRenderer(m.style, w)
		if err == nil {
			var out string
			out, err = r.Render(t.raw)
			if err == nil {
				content = out
			}
		}
		if err != nil {
			t.renderErr = err.Error()
			content = wordwrap.String(t.raw, w-2)
		}
	}
	off := t.vp.YOffset
	t.vp.SetContent(content)
	t.vp.SetYOffset(off)
	t.rendered = w
	t.renderedMode = m.mode
}

// resolveStyle pins "auto" to dark or light once at startup, so that
// renders inside the running program never query the terminal.
func resolveStyle(style string) string {
	if style == "" || style == "auto" {
		if termenv.HasDarkBackground() {
			return "dark"
		}
		return "light"
	}
	return style
}

func newRenderer(style string, width int) (*glamour.TermRenderer, error) {
	opts := []glamour.TermRendererOption{glamour.WithWordWrap(width - 2)}
	switch {
	case strings.HasSuffix(style, ".json"):
		opts = append(opts, glamour.WithStylePath(style))
	default:
		if base, ok := gstyles.DefaultStyles[style]; ok {
			sc := readerStyle(*base)
			opts = append(opts, glamour.WithStyles(sc))
		} else {
			opts = append(opts, glamour.WithStandardStyle(style))
		}
	}
	return glamour.NewTermRenderer(opts...)
}

// readerStyle adapts a glamour style for reading: the literal ##/###
// heading markers are dropped, and heading levels are distinguished by
// underline (h2) and italics (h4-h6) instead.
func readerStyle(base ansi.StyleConfig) ansi.StyleConfig {
	sc := base
	on := true
	sc.H2.Prefix = ""
	sc.H2.Underline = &on
	sc.H3.Prefix = ""
	sc.H4.Prefix = ""
	sc.H4.Italic = &on
	sc.H5.Prefix = ""
	sc.H5.Italic = &on
	sc.H6.Prefix = ""
	sc.H6.Italic = &on
	return sc
}

// ── tree cursor / scrolling ─────────────────────────────────────────

func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	m.clampCursor()
	m.scrollCursorIntoView()
}

func (m *Model) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
}

func (m *Model) scrollCursorIntoView() {
	h := m.treeInnerHeight()
	if h <= 0 {
		return
	}
	if m.cursor < m.treeOff {
		m.treeOff = m.cursor
	}
	if m.cursor >= m.treeOff+h {
		m.treeOff = m.cursor - h + 1
	}
	if m.treeOff < 0 {
		m.treeOff = 0
	}
}

func (m *Model) scrollTree(delta int) {
	m.treeOff += delta
	m.clampTree()
}

func (m *Model) clampTree() {
	maxOff := len(m.items) - m.treeInnerHeight()
	if maxOff < 0 {
		maxOff = 0
	}
	if m.treeOff > maxOff {
		m.treeOff = maxOff
	}
	if m.treeOff < 0 {
		m.treeOff = 0
	}
	m.clampCursor()
}

// ── geometry ────────────────────────────────────────────────────────

func (m *Model) sidebarWidth() int {
	if !m.sidebar {
		return 0
	}
	w := m.cfg.Sidebar.Width
	if max := m.width / 2; m.width > 0 && w > max {
		w = max
	}
	if w < 16 {
		w = 16
	}
	return w
}

func (m *Model) mainHeight() int {
	h := m.height - tabBarHeight - statusBarHeight
	if h < 0 {
		h = 0
	}
	return h
}

func (m *Model) treeInnerHeight() int    { return maxInt(m.mainHeight()-2, 0) }
func (m *Model) contentInnerHeight() int { return maxInt(m.mainHeight()-2, 0) }

func (m *Model) contentInnerWidth() int {
	return maxInt(m.width-m.sidebarWidth()-2, 0)
}

// layoutTabs recomputes viewport sizes and re-renders the active tab
// after any geometry change.
func (m *Model) layoutTabs() {
	if m.width == 0 || m.height == 0 {
		return
	}
	for _, t := range m.tabs {
		t.vp.Width = m.contentInnerWidth()
		t.vp.Height = m.contentInnerHeight()
	}
	if t := m.activeTab(); t != nil {
		t.rendered = 0
		m.ensureRendered(t)
	}
	m.computeTabRegions()
	m.clampTree()
	m.scrollCursorIntoView()
}

func baseName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > w {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
