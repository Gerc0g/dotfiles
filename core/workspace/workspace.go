// Package workspace manages isolated git worktrees for agent tasks.
//
// One task gets one worktree on its own `agent/<task>-<id>` branch, so an agent
// never writes into the checkout the user has open, and several tasks on one
// repository can run at once.
//
// The rules about destroying a worktree are the important part of this package.
// An earlier version reaped "clean and pushed" worktrees automatically on every
// start, and twice deleted work that was simply finished-for-now rather than
// abandoned. Nothing here removes a worktree unless the user asked for it by
// name, or every gate in Cleanup agrees.
package workspace

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// MetaFile marks a directory as a managed worktree and holds its metadata.
const MetaFile = ".agent-workspace"

// State is where a worktree sits in its lifecycle.
type State string

const (
	// StateActive is work in progress. Cleanup never touches it.
	StateActive State = "active"
	// StateReady means the user declared the work finished and pushed.
	StateReady State = "ready"
	// StateReview means a pull request exists; Cleanup checks whether it merged.
	StateReview State = "review"
)

// Workspace is one managed worktree.
type Workspace struct {
	Path      string
	ID        string
	Company   string
	Product   string
	Repo      string
	Task      string
	Branch    string
	BaseRef   string
	State     State
	ReviewURL string
	CreatedAt string
	PushedAt  string
}

// Ref is the company/product/repo/id path of a workspace.
func (w Workspace) Ref() string {
	return fmt.Sprintf("%s/%s/%s/%s", w.Company, w.Product, w.Repo, w.ID)
}

// Manager owns the worktree pool under a workspace root.
type Manager struct {
	root      string
	worktrees string
	vault     string
	now       func() time.Time
	// syncProjects refreshes the editor's project list after the pool changes.
	// It is a field rather than a direct call so tests do not rewrite the real
	// VS Code configuration of whoever runs them.
	syncProjects func()
}

// Option customises a Manager.
type Option func(*Manager)

// WithVault overrides where salvaged artifacts are written.
func WithVault(path string) Option {
	return func(m *Manager) { m.vault = path }
}

// WithClock overrides the clock, so tests can age a workspace deterministically.
func WithClock(now func() time.Time) Option {
	return func(m *Manager) { m.now = now }
}

// WithProjectSync overrides the editor project-list refresh.
func WithProjectSync(fn func()) Option {
	return func(m *Manager) { m.syncProjects = fn }
}

// New builds a Manager for a workspace root.
func New(root string, opts ...Option) (*Manager, error) {
	if root == "" {
		return nil, fmt.Errorf("workspace root is empty")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	m := &Manager{
		root:         root,
		worktrees:    filepath.Join(root, ".worktrees"),
		vault:        filepath.Join(home, "Desktop", "WikiPedik", "dev", "20-projects"),
		now:          time.Now,
		syncProjects: syncEditorProjects,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

// RepoDir is the main checkout of a repository.
func (m *Manager) RepoDir(company, product, repo string) string {
	return filepath.Join(m.root, company, product, repo)
}

// WorktreeDir is where a workspace with the given id lives.
func (m *Manager) WorktreeDir(company, product, repo, id string) string {
	return filepath.Join(m.worktrees, company, product, repo, id)
}

// Pool reports the root of the worktree pool.
func (m *Manager) Pool() string { return m.worktrees }

// List returns every managed workspace, sorted by path.
func (m *Manager) List() ([]Workspace, error) {
	metas, err := m.metaFiles()
	if err != nil {
		return nil, err
	}

	var out []Workspace
	for _, meta := range metas {
		w, err := readMeta(meta)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

// Find resolves one workspace by company/product/repo/id.
func (m *Manager) Find(company, product, repo, id string) (Workspace, error) {
	path := m.WorktreeDir(company, product, repo, id)

	meta := filepath.Join(path, MetaFile)
	if _, err := os.Stat(meta); err != nil {
		return Workspace{}, fmt.Errorf("не управляемый worktree: %s", path)
	}
	return readMeta(meta)
}

// metaFiles finds every workspace metadata file in the pool.
func (m *Manager) metaFiles() ([]string, error) {
	if _, err := os.Stat(m.worktrees); os.IsNotExist(err) {
		return nil, nil
	}

	var found []string
	err := filepath.WalkDir(m.worktrees, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			// An unreadable subtree must not hide the rest of the pool.
			return nil
		}
		if !entry.IsDir() && entry.Name() == MetaFile {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("обход %s: %w", m.worktrees, err)
	}

	sort.Strings(found)
	return found, nil
}

// readMeta parses a .agent-workspace file.
func readMeta(path string) (Workspace, error) {
	file, err := os.Open(path)
	if err != nil {
		return Workspace{}, fmt.Errorf("чтение %s: %w", path, err)
	}
	defer file.Close()

	w := Workspace{Path: filepath.Dir(path)}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if !found {
			continue
		}
		switch key {
		case "id":
			w.ID = value
		case "company":
			w.Company = value
		case "product":
			w.Product = value
		case "repo":
			w.Repo = value
		case "task":
			w.Task = value
		case "branch":
			w.Branch = value
		case "base_ref":
			w.BaseRef = value
		case "cleanup_state":
			w.State = State(value)
		case "review_url":
			w.ReviewURL = value
		case "created_at":
			w.CreatedAt = value
		case "pushed_at":
			w.PushedAt = value
		}
	}
	if err := scanner.Err(); err != nil {
		return Workspace{}, fmt.Errorf("чтение %s: %w", path, err)
	}

	if w.ID == "" {
		w.ID = filepath.Base(w.Path)
	}
	return w, nil
}

// writeMeta persists a workspace's metadata.
func writeMeta(w Workspace) error {
	var b strings.Builder
	write := func(key, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%s=%s\n", key, value)
		}
	}

	write("id", w.ID)
	write("company", w.Company)
	write("product", w.Product)
	write("repo", w.Repo)
	write("task", w.Task)
	write("branch", w.Branch)
	write("base_ref", w.BaseRef)
	write("cleanup_state", string(w.State))
	write("review_url", w.ReviewURL)
	write("created_at", w.CreatedAt)
	write("pushed_at", w.PushedAt)

	path := filepath.Join(w.Path, MetaFile)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("запись %s: %w", path, err)
	}
	return nil
}

var slugUnsafe = regexp.MustCompile(`[^a-z0-9._-]+`)
var slugDashes = regexp.MustCompile(`-{2,}`)

// Slugify turns free text into a branch-safe slug.
func Slugify(text string) string {
	s := slugUnsafe.ReplaceAllString(strings.ToLower(text), "-")
	s = slugDashes.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
