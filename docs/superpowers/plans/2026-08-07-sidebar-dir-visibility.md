# Sidebar Directory Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Directories in the sidebar render with a trailing slash and a dedicated configurable color so they are easy to tell apart from files.

**Architecture:** A new theme color `dir_color` (default `#89B4FA`) feeds a new `styles.dir` lipgloss style (color + bold). `renderSidebar` appends `/` to directory names before truncation and uses `styles.dir` for non-cursor directory rows. Display priority is unchanged: selection > cursor accent > directory style > plain.

**Tech Stack:** Go, lipgloss (existing patterns only — no new dependencies).

**Spec:** `docs/superpowers/specs/2026-08-07-sidebar-dir-visibility-design.md`

## Global Constraints

- Default `dir_color` is exactly `"#89B4FA"` (blue — deliberately distinct from the accent purple `#7C6AEF`).
- The cursor row keeps its current look: focused-cursor selection style and cursor accent style still override the directory style (switch-case order in `renderSidebar` already guarantees this — do not reorder it).
- The trailing `/` is part of the name text, appended BEFORE truncation/padding so long names truncate correctly.
- Mouse hit-testing is row-based and must remain untouched.
- Run `gofmt -l .` (must print nothing) and `go vet ./...` before each commit; CI also runs `go test -race ./...`.

---

### Task 1: `dir_color` config key

**Files:**
- Modify: `internal/config/config.go` (Theme struct ~line 19-35, `Default()` ~line 67-80, `merge` ~line 142-150)
- Modify: `config.example.toml` (theme table, after `border_color`)
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces (used by Task 2): `Config.Theme.DirColor string` (TOML key `dir_color`, default `"#89B4FA"`).

- [ ] **Step 1: Write the failing test**

Read `internal/config/config_test.go` first and match its style. Add:

```go
func TestDirColorKey(t *testing.T) {
	if got := Default().Theme.DirColor; got != "#89B4FA" {
		t.Errorf("default DirColor = %q, want #89B4FA", got)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[theme]\ndir_color = \"#FF0000\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.DirColor != "#FF0000" {
		t.Errorf("DirColor = %q, want #FF0000", cfg.Theme.DirColor)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestDirColorKey -v`
Expected: FAIL to build with "unknown field DirColor".

- [ ] **Step 3: Implement**

In `internal/config/config.go`:

1. `Theme` struct — after the `BorderColor` field add:

```go
	DirColor    string `toml:"dir_color"`
```

2. `Default()` — after the `BorderColor:` line add:

```go
			DirColor:    "#89B4FA",
```

3. `merge` — after the `mergeStr(&dst.Theme.BorderColor, ...)` line add:

```go
	mergeStr(&dst.Theme.DirColor, src.Theme.DirColor)
```

In `config.example.toml`, after the `border_color` line add:

```toml
dir_color = "#89B4FA"      # directory names in the sidebar
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/config/ -v`
Expected: PASS (new test and all existing ones).

- [ ] **Step 5: Verify formatting and vet, then commit**

Run: `gofmt -l . && go vet ./...`
Expected: no output from gofmt, no vet errors.

```bash
git add internal/config/config.go internal/config/config_test.go config.example.toml
git commit -m "Add theme.dir_color config key"
```

---

### Task 2: Render directories with slash and color

**Files:**
- Modify: `internal/ui/model.go` (`styles` struct ~line 99-107, `New` styles literal ~line 134-143)
- Modify: `internal/ui/view.go` (`renderSidebar` ~line 90-111)
- Modify: `README.md` (config sample `[theme]` block, ~line 96-98)
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `Config.Theme.DirColor` (Task 1).
- Produces: `styles.dir lipgloss.Style` (unexported; used only by `renderSidebar`).

- [ ] **Step 1: Write the failing tests**

Add to `internal/ui/model_test.go`:

```go
func TestSidebarMarksDirectories(t *testing.T) {
	m := testModel(t, map[string]string{"docs/a.md": "# A", "b.md": "# B"})
	view := m.View()
	if !strings.Contains(view, "docs/") {
		t.Error("directory should render with a trailing slash")
	}
	if strings.Contains(view, "b.md/") {
		t.Error("file must not have a trailing slash")
	}
	if !m.styles.dir.GetBold() {
		t.Error("dir style should be bold")
	}
	if fg, ok := m.styles.dir.GetForeground().(lipgloss.Color); !ok || string(fg) != "#89B4FA" {
		t.Errorf("dir style foreground = %v, want #89B4FA", m.styles.dir.GetForeground())
	}
}
```

Add `"github.com/charmbracelet/lipgloss"` to the test file's imports if it is not already there.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestSidebarMarksDirectories -v`
Expected: FAIL to build with "m.styles.dir undefined".

- [ ] **Step 3: Implement**

1. `internal/ui/model.go` — `styles` struct, after the `selection` field add:

```go
	dir         lipgloss.Style
```

In `New`, in the `styles:` literal after the `selection:` line add:

```go
			dir:       lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Theme.DirColor)).Bold(true),
```

2. `internal/ui/view.go` — in `renderSidebar`, replace:

```go
		text := strings.Repeat("  ", it.Depth) + icon + " " + it.Node.Name
```

with:

```go
		name := it.Node.Name
		if it.Node.IsDir {
			name += "/"
		}
		text := strings.Repeat("  ", it.Depth) + icon + " " + name
```

and replace the directory case:

```go
		case it.Node.IsDir:
			text = lipgloss.NewStyle().Bold(true).Render(text)
```

with:

```go
		case it.Node.IsDir:
			text = m.styles.dir.Render(text)
```

3. `README.md` — in the config sample's `[theme]` block (after `accent_color`), add:

```toml
dir_color = "#89B4FA"      # sidebar directory names
```

- [ ] **Step 4: Run the full test suite**

Run: `go test -race ./...`
Expected: all packages PASS.

- [ ] **Step 5: Verify formatting and vet, then commit**

Run: `gofmt -l . && go vet ./...`
Expected: no output from gofmt, no vet errors.

```bash
git add internal/ui/model.go internal/ui/view.go internal/ui/model_test.go README.md
git commit -m "Render sidebar directories with slash and dedicated color"
```

- [ ] **Step 6: Manual verification in tmux**

```bash
go build -o /tmp/mado-dir .
tmux new-session -d -s madodir -x 120 -y 30 '/tmp/mado-dir'
sleep 1
tmux capture-pane -t madodir -p | sed -n '2,8p'        # expect docs/ and internal/ with slashes
tmux capture-pane -t madodir -p -e | sed -n '3,4p'     # expect 38;2;137;180;250 (=#89B4FA) on dir rows
tmux kill-session -t madodir
```

Note (macOS host): the `timeout` command does not exist; use the `sleep` calls as shown. Cursor row check: move the cursor with `j`/`k` and confirm the selected row still uses the selection/accent styling, not the dir color.
