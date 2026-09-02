// Package ui implements mado's terminal user interface: a file tree
// sidebar, a tab bar for multiple open files, and a markdown viewport.
package ui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
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
	"github.com/hidekingerz/mado/internal/remote"
	"github.com/hidekingerz/mado/internal/search"
	"github.com/hidekingerz/mado/internal/termsafe"
	"github.com/hidekingerz/mado/internal/watch"
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
	binary       bool        // content is binary; raw holds a placeholder, not file bytes
	img          image.Image // decoded image; nil = not an image tab
	vp           viewport.Model
	content      string // what the viewport shows, kept for finding a search hit's row
	rendered     int    // viewport width the content was last rendered at; 0 = never
	renderedH    int    // viewport height the content was last rendered at; image art depends on it
	renderedMode viewMode
	renderedNums bool // line-number state the content was last rendered with
	renderErr    string
}

// setContent stores file data on the tab. Images become half-block
// art tabs; other binary data is replaced with a placeholder so raw
// control bytes never reach the terminal.
func (t *tab) setContent(data []byte, profile termenv.Profile) {
	t.img = nil
	if hasImageExt(t.path) && (profile == termenv.TrueColor || profile == termenv.ANSI256) {
		if img, err := decodeImage(data); err == nil {
			t.img = img
			t.binary = false
			t.raw = ""
			return
		}
	}
	if looksBinary(data) {
		t.binary = true
		t.raw = fmt.Sprintf("%s\n\nbinary file (%s) — not displayed", t.title, humanSize(len(data)))
		return
	}
	t.binary = false
	// Whatever wrote this file chose these bytes; they are text to
	// show, not instructions for the terminal.
	t.raw = termsafe.String(string(data))
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

	root         *filetree.Node
	treeOpts     filetree.Options
	items        []filetree.Item
	cursor       int
	treeOff      int
	sidebar      bool
	focus        focusArea
	mode         viewMode
	style        string          // resolved glamour style ("auto" already decided)
	sourceStyle  string          // chroma style for source mode; "" = no highlighting
	formatter    string          // chroma formatter; pinned at startup, "" = no color
	profile      termenv.Profile // terminal color capability, pinned at startup
	darkBG       bool            // terminal background, pinned at startup like profile
	configPath   string          // where the settings panel saves; "" = nowhere
	showLineNums bool            // vi :set nu for source-rendered content
	showHelp     bool
	statusMsg    string

	tabs       []*tab
	active     int
	tabRegions []tabRegion

	// search is the find-by-name / find-in-files panel.
	search searchState

	// settings is the panel that edits the config in place.
	settings settingsState

	// watcher is nil unless auto-reload is enabled; when set, changes
	// under the open files and the sidebar tree trigger a reload.
	watcher *watch.Watcher

	// srv is nil unless this instance accepts remote commands; when
	// set, `mado --remote …` opens files in this window.
	srv *remote.Server

	styles styles
}

type styles struct {
	accent      lipgloss.Style
	border      lipgloss.Style
	selection   lipgloss.Style
	cursor      lipgloss.Style
	dir         lipgloss.Style
	file        lipgloss.Style
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
	darkBG := termenv.HasDarkBackground()
	style := resolveStyle(cfg.Theme.Style, darkBG)
	m := Model{
		cfg:         cfg,
		keys:        newKeyMap(cfg.Keys),
		root:        root,
		treeOpts:    opts,
		sidebar:     true,
		focus:       focusSidebar,
		mode:        mode,
		style:       style,
		sourceStyle: chromaStyleName(style, cfg.Theme.SourceStyle, darkBG),
		formatter:   chromaFormatterName(),
		profile:     termenv.ColorProfile(),
		darkBG:      darkBG,
		styles:      newStyles(cfg.Theme),
	}
	m.items = filetree.Flatten(m.root)
	if cfg.General.Watch {
		w, err := watch.New(watch.DefaultDebounce)
		if err != nil {
			// Auto-reload is a convenience; losing it should not stop
			// mado from opening the files.
			m.statusMsg = "watch disabled: " + err.Error()
		} else {
			m.watcher = w
		}
	}
	for _, f := range initialFiles {
		_ = m.openFile(f)
	}
	if len(m.tabs) > 0 {
		m.focus = focusContent
	}
	m.syncWatch()
	return m, nil
}

// Serve routes commands from `mado --remote …` into this instance.
func (m Model) Serve(s *remote.Server) Model {
	m.srv = s
	return m
}

// remoteRequestMsg carries one command from another process.
type remoteRequestMsg struct{ req *remote.Request }

// waitForRequest blocks until another process sends a command. A
// closed server ends the loop.
func waitForRequest(s *remote.Server) tea.Cmd {
	return func() tea.Msg {
		req, ok := s.Next()
		if !ok {
			return nil
		}
		return remoteRequestMsg{req}
	}
}

// handleRemote carries out one remote command and answers the process
// that sent it.
func (m *Model) handleRemote(req *remote.Request) {
	switch req.Cmd {
	case remote.CmdOpen:
		if err := m.openFile(req.Path); err != nil {
			req.Respond(err)
			return
		}
		// The file is the point of the command: show it, whatever was
		// on screen before.
		m.showHelp = false
		m.closeSearch()
		m.closeSettings()
		m.focus = focusContent
		req.Respond(nil)
	case remote.CmdFocus:
		for i, t := range m.tabs {
			if t.path == req.Path {
				m.active = i
				m.showHelp = false
				m.closeSearch()
				m.closeSettings()
				m.focus = focusContent
				m.ensureRendered(t)
				req.Respond(nil)
				return
			}
		}
		req.Respond(fmt.Errorf("%s is not open", req.Path))
	default:
		req.Respond(fmt.Errorf("unknown remote command %q", req.Cmd))
	}
}

// fileChangedMsg reports that something under the watched directories
// changed since the last reload.
type fileChangedMsg struct{}

// waitForChange blocks until the watcher reports a change. A closed
// channel (the watcher was shut down) ends the loop.
func waitForChange(w *watch.Watcher) tea.Cmd {
	return func() tea.Msg {
		if _, ok := <-w.Events(); !ok {
			return nil
		}
		return fileChangedMsg{}
	}
}

// syncWatch points the watcher at the directories mado is showing: the
// parent of every open file, plus the tree root and every expanded
// directory, so that new and removed files are noticed too.
func (m *Model) syncWatch() {
	if m.watcher == nil {
		return
	}
	dirs := map[string]bool{m.root.Path: true}
	for _, it := range m.items {
		if it.Node.IsDir && it.Node.Expanded {
			dirs[it.Node.Path] = true
		}
	}
	for _, t := range m.tabs {
		dirs[filepath.Dir(t.path)] = true
	}
	list := make([]string, 0, len(dirs))
	for d := range dirs {
		list = append(list, d)
	}
	m.watcher.SetDirs(list)
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.watcher != nil {
		cmds = append(cmds, waitForChange(m.watcher))
	}
	if m.srv != nil {
		cmds = append(cmds, waitForRequest(m.srv))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

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
	case fileChangedMsg:
		m.reload()
		return m, waitForChange(m.watcher)
	case remoteRequestMsg:
		m.handleRemote(msg.req)
		return m, waitForRequest(m.srv)
	case searchResultMsg:
		m.handleSearchResult(msg)
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	// The settings panel owns the keyboard while open, so a key being
	// captured can be anything — esc included. Only ctrl+c still quits.
	if m.settings.open && msg.Type != tea.KeyCtrlC {
		return m.handleSettingsKey(msg)
	}
	// While the search prompt has the keyboard, typed characters are
	// the query — "q" included. Only a quit key that cannot be typed
	// (ctrl+c) still quits.
	if m.search.open && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace || !key.Matches(msg, k.Quit)) {
		return m.handleSearchKey(msg)
	}
	switch {
	case key.Matches(msg, k.Quit):
		m.cancelSearch()
		if m.watcher != nil {
			m.watcher.Close()
		}
		if m.srv != nil {
			m.srv.Close()
		}
		return m, tea.Quit
	case key.Matches(msg, k.Search):
		return m, m.openSearch(search.Names)
	case key.Matches(msg, k.SearchContent):
		return m, m.openSearch(search.Contents)
	case key.Matches(msg, k.Settings):
		m.openSettings()
		return m, nil
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
	case key.Matches(msg, k.ToggleAllFiles):
		m.treeOpts.ShowAllFiles = !m.treeOpts.ShowAllFiles
		m.reloadTree()
	case key.Matches(msg, k.ToggleLineNums):
		m.showLineNums = !m.showLineNums
		m.ensureRendered(m.activeTab())
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.settings.open {
		return m.handleSettingsMouse(msg)
	}
	if m.search.open {
		return m.handleSearchMouse(msg)
	}
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
			m.revealInSidebar(m.activeTab().path)
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
		m.syncWatch()
		return
	}
	_ = m.openFile(n.Path)
	m.focus = focusContent
}

// openFile opens path in a new tab, or activates the existing tab.
// The error it returns is already shown in the status bar; callers
// acting for another process report it there too.
func (m *Model) openFile(path string) error {
	// Tabs are identified by absolute path, so the same file named
	// two ways — a relative command-line argument and the absolute
	// path the sidebar or another process sends — is one tab.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	for i, t := range m.tabs {
		if t.path == path {
			m.active = i
			m.ensureRendered(t)
			m.revealInSidebar(path)
			return nil
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		m.statusMsg = err.Error()
		return err
	}
	t := &tab{
		path:  path,
		title: termsafe.String(baseName(path)),
	}
	t.setContent(data, m.profile)
	t.vp = viewport.New(m.contentInnerWidth(), m.contentInnerHeight())
	m.tabs = append(m.tabs, t)
	m.active = len(m.tabs) - 1
	m.layoutTabs()
	m.ensureRendered(t)
	m.syncWatch()
	m.revealInSidebar(path)
	m.statusMsg = ""
	return nil
}

// revealInSidebar makes the sidebar follow path: ancestors expand and
// the cursor lands on the file's row. Best-effort — files hidden by
// the filter get their ancestors expanded at most, files outside the
// tree change nothing, and a partial expand still re-flattens.
func (m *Model) revealInSidebar(path string) {
	_ = m.root.Reveal(path, m.treeOpts)
	m.items = filetree.Flatten(m.root)
	for i, it := range m.items {
		if it.Node.Path == path {
			m.cursor = i
			break
		}
	}
	m.clampTree()
	m.scrollCursorIntoView()
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
	m.syncWatch()
}

func (m *Model) switchTab(delta int) {
	if len(m.tabs) < 2 {
		return
	}
	m.active = (m.active + delta + len(m.tabs)) % len(m.tabs)
	m.focus = focusContent
	m.ensureRendered(m.activeTab())
	m.revealInSidebar(m.activeTab().path)
}

func (m *Model) activeTab() *tab {
	if m.active < 0 || m.active >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.active]
}

// reloadTree re-reads the file tree, preserving expansion state.
func (m *Model) reloadTree() {
	// Remember the selected node by path: reloading can add or remove
	// rows, so the cursor index alone would drift to a different item.
	var curPath string
	if m.cursor >= 0 && m.cursor < len(m.items) {
		curPath = m.items[m.cursor].Node.Path
	}
	if err := m.root.Reload(m.treeOpts); err != nil {
		m.statusMsg = err.Error()
	}
	m.items = filetree.Flatten(m.root)
	for i, it := range m.items {
		if it.Node.Path == curPath {
			m.cursor = i
			break
		}
	}
	m.clampTree()
	m.scrollCursorIntoView()
	m.syncWatch()
}

// reload re-reads the tree and the contents of every open tab.
// Background tabs are only marked dirty: they re-render the next time
// they are shown, so a reload costs one render however many tabs are
// open. Scroll positions survive, since ensureRendered restores them.
func (m *Model) reload() {
	m.reloadTree()
	active := m.activeTab()
	for _, t := range m.tabs {
		data, err := os.ReadFile(t.path)
		if err != nil {
			if t == active {
				m.statusMsg = err.Error()
			}
			continue
		}
		t.setContent(data, m.profile)
		t.rendered = 0
	}
	m.ensureRendered(active)
}

// ensureRendered (re-)renders a tab's content at the current viewport
// width and view mode if needed.
func (m *Model) ensureRendered(t *tab) {
	if t == nil {
		return
	}
	w := m.contentInnerWidth()
	h := m.contentInnerHeight()
	if w <= 0 || (t.rendered == w && t.renderedH == h && t.renderedMode == m.mode && t.renderedNums == m.showLineNums) {
		return
	}
	t.vp.Width = w
	t.vp.Height = m.contentInnerHeight()
	t.renderErr = ""

	content := t.raw
	if t.img != nil {
		ib := t.img.Bounds()
		art := renderImage(t.img, w-2, m.contentInnerHeight()-3, m.profile)
		caption := m.styles.dimmed.Render(fmt.Sprintf("%s  %d×%d", t.title, ib.Dx(), ib.Dy()))
		content = lipgloss.Place(w, m.contentInnerHeight(), lipgloss.Center, lipgloss.Center, art+"\n\n"+caption)
	} else if t.binary {
		content = lipgloss.Place(w, m.contentInnerHeight(), lipgloss.Center, lipgloss.Center, t.raw)
	} else if m.mode == modeSource || !filetree.IsMarkdown(t.path) {
		if hl, err := highlightSource(t.raw, t.path, m.sourceStyle, m.formatter); err == nil {
			content = hl
		}
		if m.showLineNums {
			content = numberLines(content, w-2, m.styles.dimmed)
		} else {
			content = wordwrap.String(content, w-2)
		}
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
	t.content = content
	t.vp.SetContent(content)
	t.vp.SetYOffset(off)
	t.rendered = w
	t.renderedH = h
	t.renderedMode = m.mode
	t.renderedNums = m.showLineNums
}

// resolveStyle pins "auto" to dark or light. darkBG is the terminal
// background as read once at startup, so renders inside the running
// program never query the terminal.
func resolveStyle(style string, darkBG bool) string {
	if style == "" || style == "auto" {
		if darkBG {
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
	m.clampSearch()
	m.clampSettings()
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
