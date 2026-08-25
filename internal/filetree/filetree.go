// Package filetree builds a lazily-loaded directory tree rooted at a
// directory, for display in mado's sidebar.
package filetree

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options controls which entries are shown.
type Options struct {
	ShowAllFiles bool // show every file, not just markdown
	ShowHidden   bool // show dotfiles and dot-directories
}

// Node is a file or directory in the tree.
type Node struct {
	Name     string
	Path     string
	IsDir    bool
	Expanded bool
	loaded   bool
	Children []*Node
}

// Item is a visible row: a node plus its indentation depth.
type Item struct {
	Node  *Node
	Depth int
}

var markdownExts = map[string]bool{
	".md":       true,
	".markdown": true,
	".mdown":    true,
	".mkd":      true,
}

// IsMarkdown reports whether path has a markdown file extension.
func IsMarkdown(path string) bool {
	return markdownExts[strings.ToLower(filepath.Ext(path))]
}

// New builds the root node for dir and loads its first level.
func New(dir string, opts Options) (*Node, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root := &Node{Name: filepath.Base(abs), Path: abs, IsDir: true, Expanded: true}
	if err := root.load(opts); err != nil {
		return nil, err
	}
	return root, nil
}

// Toggle expands or collapses a directory node, loading its children on
// first expansion. It is a no-op for files.
func (n *Node) Toggle(opts Options) error {
	if !n.IsDir {
		return nil
	}
	if n.Expanded {
		n.Expanded = false
		return nil
	}
	if !n.loaded {
		if err := n.load(opts); err != nil {
			return err
		}
	}
	n.Expanded = true
	return nil
}

// Reveal expands every ancestor directory of path under n, loading
// lazily as needed, so the entry becomes visible in the flattened
// tree. Paths outside n and ancestors the walk cannot find (filtered,
// hidden, or gone) end the walk quietly: revealing is best-effort and
// never fails the caller's open. Only a directory read error during
// expansion is returned.
func (n *Node) Reveal(path string, opts Options) error {
	rel, err := filepath.Rel(n.Path, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return nil
	}
	segs := strings.Split(rel, string(filepath.Separator))
	cur := n
	for _, seg := range segs[:len(segs)-1] {
		var next *Node
		for _, c := range cur.Children {
			if c.IsDir && c.Name == seg {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		if !next.loaded {
			if err := next.load(opts); err != nil {
				return err
			}
		}
		next.Expanded = true
		cur = next
	}
	return nil
}

// Reload re-reads the children of every loaded directory under n,
// preserving expansion state at every depth where paths still exist —
// including state remembered inside directories that are currently
// collapsed but were loaded before. Directories that were never loaded
// stay lazy. If a previously loaded subdirectory can no longer be
// read, it is left collapsed and the first such error is returned so
// the caller can surface it.
func (n *Node) Reload(opts Options) error {
	if !n.IsDir || !n.loaded {
		return nil
	}
	// Snapshot the whole old tree before load() replaces any node.
	loaded := map[string]bool{}
	expanded := map[string]bool{}
	collectState(n, loaded, expanded)
	return n.reloadInto(loaded, expanded, opts)
}

func collectState(n *Node, loaded, expanded map[string]bool) {
	for _, c := range n.Children {
		if !c.IsDir {
			continue
		}
		if c.Expanded {
			expanded[c.Path] = true
		}
		if c.loaded {
			loaded[c.Path] = true
			collectState(c, loaded, expanded)
		}
	}
}

// reloadInto re-reads n's children, recursing into every directory that
// was loaded before and restoring its expansion flag.
func (n *Node) reloadInto(loaded, expanded map[string]bool, opts Options) error {
	if err := n.load(opts); err != nil {
		return err
	}
	var firstErr error
	for _, c := range n.Children {
		if !c.IsDir || !loaded[c.Path] {
			continue
		}
		if err := c.reloadInto(loaded, expanded, opts); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		c.Expanded = expanded[c.Path]
	}
	return firstErr
}

func (n *Node) load(opts Options) error {
	entries, err := os.ReadDir(n.Path)
	if err != nil {
		return err
	}
	var dirs, files []*Node
	for _, e := range entries {
		name := e.Name()
		if !opts.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}
		child := &Node{
			Name:  name,
			Path:  filepath.Join(n.Path, name),
			IsDir: e.IsDir(),
		}
		if child.IsDir {
			dirs = append(dirs, child)
		} else {
			if !opts.ShowAllFiles && !IsMarkdown(name) {
				continue
			}
			files = append(files, child)
		}
	}
	sortNodes(dirs)
	sortNodes(files)
	n.Children = append(dirs, files...)
	n.loaded = true
	return nil
}

func sortNodes(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
}

// Flatten returns the visible rows of the tree. The root itself is not
// included; its children start at depth 0.
func Flatten(root *Node) []Item {
	var items []Item
	var walk func(n *Node, depth int)
	walk = func(n *Node, depth int) {
		for _, c := range n.Children {
			items = append(items, Item{Node: c, Depth: depth})
			if c.IsDir && c.Expanded {
				walk(c, depth+1)
			}
		}
	}
	if root.Expanded {
		walk(root, 0)
	}
	return items
}
