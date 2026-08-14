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

// Reload re-reads the children of every loaded directory under n,
// preserving expansion state at every depth where paths still exist.
func (n *Node) Reload(opts Options) error {
	if !n.IsDir || !n.loaded {
		return nil
	}
	// Collect the expanded paths of the whole old tree before load()
	// replaces any node, so deeply nested state survives the reload.
	expanded := map[string]bool{}
	collectExpanded(n, expanded)
	return n.reloadInto(expanded, opts)
}

func collectExpanded(n *Node, set map[string]bool) {
	for _, c := range n.Children {
		if c.IsDir && c.Expanded {
			set[c.Path] = true
			collectExpanded(c, set)
		}
	}
}

// reloadInto re-reads n's children and re-expands those whose paths are
// in expanded. Only previously expanded directories are loaded, keeping
// the rest lazy.
func (n *Node) reloadInto(expanded map[string]bool, opts Options) error {
	if err := n.load(opts); err != nil {
		return err
	}
	for _, c := range n.Children {
		if c.IsDir && expanded[c.Path] {
			if err := c.reloadInto(expanded, opts); err == nil {
				c.Expanded = true
			}
		}
	}
	return nil
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
