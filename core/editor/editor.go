// Package editor mirrors the workspace model into the editor's project list.
//
// VS Code Project Manager keeps a flat projects.json. This package regenerates
// the dotfiles-managed entries in it from the world scan and the worktree
// pool, so the editor sidebar always matches what is actually on disk. Managed
// entries carry the "dotfiles" tag; what happens to the rest is decided by
// SyncOptions, not by accident.
//
// The package deliberately does not import the workspace package — workspace
// calls into here after every pool change, so worktrees arrive as the plain
// Worktree value instead of a workspace.Workspace.
package editor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/world"
)

// ManagedTag marks a projects.json entry as owned by this sync.
const ManagedTag = "dotfiles"

// Entry is one desired projects.json record.
type Entry struct {
	Name     string
	RootPath string
	Tags     []string
}

// Worktree is the slice of workspace metadata the project list cares about.
type Worktree struct {
	Path    string
	ID      string
	Company string
	Product string
	Repo    string
	Task    string
	Branch  string
	State   string
}

// Stats counts what Sync did to the existing project list.
type Stats struct {
	Preserved      int
	Adopted        int
	Updated        int
	Added          int
	Pruned         int
	ExternalPruned int
}

func (s Stats) String() string {
	return fmt.Sprintf("add=%d adopt=%d update=%d prune=%d external_prune=%d preserve=%d",
		s.Added, s.Adopted, s.Updated, s.Pruned, s.ExternalPruned, s.Preserved)
}

// SyncOptions controls which entries are generated and what happens to
// everything else in the file.
type SyncOptions struct {
	Products         bool
	Repos            bool
	Prune            bool
	PreserveExternal bool
	Backup           bool
	DryRun           bool
}

// DefaultOptions mirrors the Project Manager as the platform's view: products
// and repos in, stale managed entries out, external entries out.
func DefaultOptions() SyncOptions {
	return SyncOptions{Products: true, Repos: true, Prune: true}
}

// DefaultProjectsFile is where the Project Manager extension keeps its list.
func DefaultProjectsFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "Code", "User",
		"globalStorage", "alefragnani.project-manager", "projects.json"), nil
}

// FromTree builds the product and repo entries of a scanned world.
func FromTree(tree *world.Tree, products, repos bool) []Entry {
	var out []Entry

	if products {
		for _, p := range tree.Products("") {
			out = append(out, Entry{
				Name:     fmt.Sprintf("Product: %s/%s", p.Company, p.Slug),
				RootPath: p.Path,
				Tags: []string{ManagedTag, "product",
					"company:" + p.Company, "product:" + p.Slug},
			})
		}
	}

	if repos {
		for _, r := range tree.Repos("", "") {
			out = append(out, Entry{
				Name:     r.Slug,
				RootPath: r.Path,
				Tags: []string{ManagedTag, "repo",
					"company:" + r.Company, "product:" + r.Product, "repo:" + r.Slug},
			})
		}
	}

	return out
}

// FromWorktrees builds entries for active agent worktrees.
func FromWorktrees(worktrees []Worktree) []Entry {
	var out []Entry
	for _, w := range worktrees {
		task := w.Task
		if task == "" {
			task = "work"
		}
		state := w.State
		if state == "" {
			state = "unknown"
		}

		tags := []string{ManagedTag, "worktree",
			"company:" + orUnknown(w.Company),
			"product:" + orUnknown(w.Product),
			"repo:" + orUnknown(w.Repo),
			"task:" + task,
			"state:" + state,
			"id:" + w.ID,
		}
		if w.Branch != "" {
			tags = append(tags, "branch:"+w.Branch)
		}

		out = append(out, Entry{
			Name:     fmt.Sprintf("%s @ %s · %s", orUnknown(w.Repo), w.ID, task),
			RootPath: w.Path,
			Tags:     tags,
		})
	}
	return out
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// Result reports one Sync run.
type Result struct {
	Stats      Stats
	Changed    bool
	BackupPath string
	Desired    int
}

// Sync merges the desired entries into the projects file.
//
// Existing entries keep their order and, for managed ones, their user-tuned
// fields (enabled, profile, paths). Entries the model no longer produces are
// pruned when opts.Prune is set; entries the model never produced are pruned
// unless opts.PreserveExternal keeps them. New entries land at the end,
// sorted by name.
func Sync(path string, desired []Entry, opts SyncOptions) (Result, error) {
	existing, err := loadProjects(path)
	if err != nil {
		return Result{}, err
	}

	desiredByRoot := map[string]Entry{}
	for _, entry := range desired {
		desiredByRoot[entry.RootPath] = entry
	}

	var result []map[string]any
	var stats Stats

	for _, project := range existing {
		root := projectRoot(project)
		want, found := desiredByRoot[root]
		if found {
			delete(desiredByRoot, root)
			result = append(result, want.toJSON(project))
			if isManaged(project) {
				stats.Updated++
			} else {
				stats.Adopted++
			}
			continue
		}

		if opts.Prune && isManaged(project) {
			stats.Pruned++
			continue
		}
		if !opts.PreserveExternal && !isManaged(project) {
			stats.ExternalPruned++
			continue
		}

		result = append(result, project)
		stats.Preserved++
	}

	var missing []Entry
	for _, entry := range desiredByRoot {
		missing = append(missing, entry)
	}
	sort.Slice(missing, func(i, j int) bool {
		return strings.ToLower(missing[i].Name) < strings.ToLower(missing[j].Name)
	})
	for _, entry := range missing {
		result = append(result, entry.toJSON(nil))
		stats.Added++
	}

	res := Result{
		Stats:   stats,
		Changed: !reflect.DeepEqual(result, existing),
		Desired: len(desired),
	}

	if opts.DryRun || !res.Changed {
		return res, nil
	}

	backupPath, err := writeProjects(path, result, opts.Backup)
	if err != nil {
		return Result{}, err
	}
	res.BackupPath = backupPath
	return res, nil
}

// toJSON renders an entry, keeping the user-tunable fields of the record it
// replaces.
func (e Entry) toJSON(existing map[string]any) map[string]any {
	enabled := true
	profile := ""
	paths := []any{}

	if existing != nil {
		if v, ok := existing["enabled"].(bool); ok {
			enabled = v
		}
		if v, ok := existing["profile"].(string); ok {
			profile = v
		}
		if v, ok := existing["paths"].([]any); ok {
			paths = v
		}
	}

	tags := make([]any, len(e.Tags))
	for i, tag := range e.Tags {
		tags[i] = tag
	}

	return map[string]any{
		"name":     e.Name,
		"rootPath": e.RootPath,
		"paths":    paths,
		"tags":     tags,
		"enabled":  enabled,
		"profile":  profile,
	}
}

func isManaged(project map[string]any) bool {
	tags, ok := project["tags"].([]any)
	if !ok {
		return false
	}
	for _, tag := range tags {
		if tag == ManagedTag {
			return true
		}
	}
	return false
}

func projectRoot(project map[string]any) string {
	if v, ok := project["rootPath"].(string); ok && v != "" {
		return v
	}
	if v, ok := project["fullPath"].(string); ok && v != "" {
		return v
	}
	return ""
}

func loadProjects(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("чтение %s: %w", path, err)
	}

	var raw []any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("разбор %s: %w", path, err)
	}

	var projects []map[string]any
	for _, item := range raw {
		if project, ok := item.(map[string]any); ok {
			projects = append(projects, project)
		}
	}
	return projects, nil
}

func writeProjects(path string, projects []map[string]any, backup bool) (string, error) {
	if projects == nil {
		projects = []map[string]any{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("создание %s: %w", filepath.Dir(path), err)
	}

	backupPath := ""
	if backup {
		if _, err := os.Stat(path); err == nil {
			backupPath = fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("чтение %s: %w", path, err)
			}
			if err := os.WriteFile(backupPath, data, 0o644); err != nil {
				return "", fmt.Errorf("запись %s: %w", backupPath, err)
			}
		}
	}

	data, err := json.MarshalIndent(projects, "", "    ")
	if err != nil {
		return "", fmt.Errorf("сериализация %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("запись %s: %w", path, err)
	}
	return backupPath, nil
}
