# Sidebar All-Files Toggle Key Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A key (default `a`) toggles the sidebar between "markdown only" and "all files" at runtime.

**Architecture:** A new rebindable key action `toggle_all_files` flips `Model.treeOpts.ShowAllFiles` and refreshes the tree. The tree-refresh half of the existing `reload()` is extracted into a `reloadTree()` helper shared by both paths. Expansion state survives because the existing `filetree.Reload` already preserves it.

**Tech Stack:** Go, Bubble Tea / bubbles `key` package (existing patterns only — no new dependencies).

**Spec:** `docs/superpowers/specs/2026-08-06-toggle-all-files-design.md`

## Global Constraints

- Startup default stays "markdown only"; `sidebar.show_all_files = true` in config still works and simply sets the toggle's initial state (it already initializes `treeOpts`; no extra code needed — do not add any).
- Expanded directories keep their expansion state across the toggle.
- Directory read failures surface via `statusMsg` (same as the existing `reload()` path); the UI never breaks.
- No binary-file filtering, no hidden-files toggle, no CLI flags (out of scope per spec).
- Run `gofmt -l .` (must print nothing) and `go vet ./...` before each commit; CI also runs `go test -race ./...`.

---

### Task 1: `toggle_all_files` config key

**Files:**
- Modify: `internal/config/config.go` (Keys struct ~line 43-60, `Default()` ~line 80-97, `merge` ~line 154-169)
- Modify: `config.example.toml` (keys table, ~line 28-47)
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces (used by Task 2): `Config.Keys.ToggleAllFiles []string` (TOML key `toggle_all_files`, default `["a"]`).

- [ ] **Step 1: Write the failing test**

Read `internal/config/config_test.go` first and match its style. Add:

```go
func TestToggleAllFilesKey(t *testing.T) {
	if got := Default().Keys.ToggleAllFiles; len(got) != 1 || got[0] != "a" {
		t.Errorf("default ToggleAllFiles = %v, want [a]", got)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[keys]\ntoggle_all_files = [\"F\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Keys.ToggleAllFiles) != 1 || cfg.Keys.ToggleAllFiles[0] != "F" {
		t.Errorf("ToggleAllFiles = %v, want [F]", cfg.Keys.ToggleAllFiles)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestToggleAllFilesKey -v`
Expected: FAIL to build with "unknown field ToggleAllFiles".

- [ ] **Step 3: Implement**

In `internal/config/config.go`:

1. `Keys` struct — after the `ToggleMode` field add:

```go
	ToggleAllFiles []string `toml:"toggle_all_files"`
```

2. `Default()` — after the `ToggleMode:` line add:

```go
		ToggleAllFiles: []string{"a"},
```

3. `merge` — after the `mergeKeys(&dst.Keys.ToggleMode, ...)` line add:

```go
	mergeKeys(&dst.Keys.ToggleAllFiles, src.Keys.ToggleAllFiles)
```

In `config.example.toml`, after the `toggle_mode` line add:

```toml
toggle_all_files = ["a"]     # show all files / markdown only in sidebar
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/config/ -v`
Expected: PASS (new test and all existing ones).

- [ ] **Step 5: Verify formatting and vet, then commit**

Run: `gofmt -l . && go vet ./...`
Expected: no output from gofmt, no vet errors.

```bash
git add internal/config/config.go internal/config/config_test.go config.example.toml
git commit -m "Add toggle_all_files key config"
```

---

### Task 2: Wire the toggle into the model

**Files:**
- Modify: `internal/ui/keys.go` (keyMap struct ~line 9-26, `newKeyMap` ~line 32-49)
- Modify: `internal/ui/model.go` (`handleKeys` switch, the `case key.Matches(msg, k.ToggleMode):` block ends ~line 258; `reload()` ~line 412-427)
- Modify: `internal/ui/view.go` (`renderHelp` rows ~line 162-179)
- Modify: `README.md` (key bindings table; sidebar Features bullet)
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `Config.Keys.ToggleAllFiles []string` (Task 1).
- Produces: unexported `(*Model).reloadTree()` helper; `keyMap.ToggleAllFiles key.Binding`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/model_test.go`:

```go
func TestToggleAllFilesShowsNonMarkdown(t *testing.T) {
	m := testModel(t, map[string]string{"a.md": "# A", "main.go": "package main\n"})
	has := func(name string) bool {
		for _, it := range m.items {
			if it.Node.Name == name {
				return true
			}
		}
		return false
	}
	if has("main.go") {
		t.Fatal("main.go should be hidden by default")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !has("main.go") {
		t.Fatal("main.go should appear after toggle")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if has("main.go") {
		t.Fatal("main.go should be hidden again after second toggle")
	}
}

func TestToggleAllFilesKeepsExpansion(t *testing.T) {
	m := testModel(t, map[string]string{"docs/a.md": "# A", "b.md": "# B", "main.go": "package main\n"})
	// Directories sort first: cursor 0 is docs. Enter expands it.
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	var docsExpanded, hasGo bool
	for _, it := range m.items {
		if it.Node.Name == "docs" && it.Node.Expanded {
			docsExpanded = true
		}
		if it.Node.Name == "main.go" {
			hasGo = true
		}
	}
	if !docsExpanded {
		t.Error("docs should stay expanded across the toggle")
	}
	if !hasGo {
		t.Error("main.go should be visible after the toggle")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run TestToggleAllFiles -v`
Expected: both tests FAIL — the `'a'` key does nothing yet, so `main.go` never appears.

- [ ] **Step 3: Implement**

1. `internal/ui/keys.go` — keyMap struct, after `ToggleMode key.Binding`:

```go
	ToggleAllFiles key.Binding
```

`newKeyMap`, after the `ToggleMode:` line:

```go
		ToggleAllFiles: bind(k.ToggleAllFiles, "all files / markdown only"),
```

2. `internal/ui/model.go` — extract the tree-refresh half of `reload()` into a helper and call it from both places. Replace the current `reload()`:

```go
// reloadTree re-reads the file tree, preserving expansion state.
func (m *Model) reloadTree() {
	if err := m.root.Reload(m.treeOpts); err != nil {
		m.statusMsg = err.Error()
	}
	m.items = filetree.Flatten(m.root)
	m.clampTree()
}

func (m *Model) reload() {
	m.reloadTree()
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
```

In the key switch, after the `case key.Matches(msg, k.ToggleMode):` block add:

```go
	case key.Matches(msg, k.ToggleAllFiles):
		m.treeOpts.ShowAllFiles = !m.treeOpts.ShowAllFiles
		m.reloadTree()
```

3. `internal/ui/view.go` — in `renderHelp`, after the `{k.ToggleMode...}` row add:

```go
		{k.ToggleAllFiles.Help().Key, k.ToggleAllFiles.Help().Desc},
```

4. `README.md` — in the "Default key bindings" table, after the `m` row add:

```markdown
| `a`                | show all files / markdown only  |
```

and extend the sidebar Features bullet to mention the toggle, e.g. "**Sidebar file tree** rooted at the current directory (lazy-loaded, collapsible; `a` toggles between markdown-only and all files)".

- [ ] **Step 4: Run the full test suite**

Run: `go test -race ./...`
Expected: all packages PASS.

- [ ] **Step 5: Verify formatting and vet, then commit**

Run: `gofmt -l . && go vet ./...`
Expected: no output from gofmt, no vet errors.

```bash
git add internal/ui/keys.go internal/ui/model.go internal/ui/view.go internal/ui/model_test.go README.md
git commit -m "Add key to toggle all files in sidebar"
```

- [ ] **Step 6: Manual verification in tmux**

```bash
go build -o /tmp/mado-toggle .
tmux new-session -d -s madotgl -x 140 -y 40 '/tmp/mado-toggle'
sleep 1
tmux capture-pane -t madotgl -p | sed -n '3,12p'   # sidebar: markdown + dirs only
tmux send-keys -t madotgl 'a'
sleep 0.5
tmux capture-pane -t madotgl -p | sed -n '3,12p'   # expect main.go, go.mod, LICENSE etc.
tmux send-keys -t madotgl 'a'
sleep 0.5
tmux capture-pane -t madotgl -p | sed -n '3,12p'   # back to markdown only
tmux kill-session -t madotgl
```

Note (macOS host): the `timeout` command does not exist; use the `sleep` calls as shown.
