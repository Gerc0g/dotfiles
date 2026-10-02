// Package runner is the sole server-side owner of Codex execution boundaries.
package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

type Binding struct {
	Kind         string `json:"kind"`
	EntityRef    string `json:"entityRef,omitempty"`
	CompanyID    string `json:"companyId,omitempty"`
	RepoID       string `json:"repoId,omitempty"`
	WorktreePath string `json:"directory"`
	Preset       string `json:"preset"`
	HomeKey      string `json:"-"`
}

func dataRoot() string {
	if p := os.Getenv("HQ_DATA_ROOT"); p != "" {
		return p
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local/state/hq")
}
func wikiRoot() string {
	if p := os.Getenv("HQ_WIKI_ROOT"); p != "" {
		return p
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Desktop/WikiPedik")
}

// OrdinaryDirectory exposes an existing owner-provisioned workspace. Catalog
// reads and launch validation never create directories or follow child aliases.
func OrdinaryDirectory() (string, error) {
	root, err := world.SafePath(dataRoot(), "runner", "workspaces", "ordinary")
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", fmt.Errorf("ordinary directory unavailable")
	}
	return root, nil
}

// Resolve accepts registered work targets and the exact managed chat roots.
// The workspace path, never mutable .workspace.json metadata, supplies the binding.
func Resolve(directory, preset string) (Binding, error) {
	b := Binding{Preset: preset}
	if !filepath.IsAbs(directory) {
		return b, fmt.Errorf("HQ requires an absolute directory")
	}
	directory = filepath.Clean(directory)
	if preset == "work" {
		if research, err := world.SafePath(wikiRoot(), "research"); err == nil && directory == research {
			preset = "research"
			b.Preset = preset
		}
		if ordinary, err := OrdinaryDirectory(); err == nil && directory == ordinary {
			preset = "ordinary"
			b.Preset = preset
		}
	}
	if preset == "ordinary" {
		root, err := OrdinaryDirectory()
		if err != nil {
			return b, err
		}
		if directory != root {
			return b, fmt.Errorf("ordinary requires the isolated ordinary directory")
		}
		b.WorktreePath, b.Kind, b.HomeKey = root, "ordinary", "ordinary"
		return b, nil
	}
	if preset == "research" {
		root, err := world.SafePath(wikiRoot(), "research")
		if err != nil {
			return b, err
		}
		if directory != root {
			return b, fmt.Errorf("research requires the isolated research directory")
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			return b, fmt.Errorf("research directory unavailable")
		}
		b.WorktreePath = root
		b.Kind = "research"
		b.HomeKey = "research"
		return b, nil
	}
	if preset != "work" {
		return b, fmt.Errorf("unknown HQ preset")
	}
	root, err := world.Root()
	if err != nil {
		return b, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return b, err
	}
	rel, err := filepath.Rel(root, directory)
	if err != nil {
		return b, err
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 1 || len(parts) == 2 {
		for _, part := range parts {
			if err = world.ValidateSegment(part); err != nil {
				return b, err
			}
		}
		safe, e := world.SafePath(root, parts...)
		if e != nil {
			return b, e
		}
		if !world.IsCompanyDir(filepath.Join(root, parts[0])) || (len(parts) == 2 && !world.IsProductDir(safe)) {
			return b, fmt.Errorf("HQ entity is not registered")
		}
		b.Kind = "company"
		if len(parts) == 2 {
			b.Kind = "product"
		}
		b.EntityRef = strings.Join(parts, "/")
		b.CompanyID = parts[0]
		b.WorktreePath = safe
		b.HomeKey = filepath.Join("context", b.EntityRef)
		return b, nil
	}
	worktree := len(parts) == 5 && parts[0] == ".worktrees"
	if len(parts) != 3 && !worktree {
		return b, fmt.Errorf("HQ requires an exact registered repository or task worktree")
	}
	triple := parts
	if worktree {
		triple = parts[1:4]
	}
	for _, s := range triple {
		if err := world.ValidateSegment(s); err != nil {
			return b, err
		}
	}
	if worktree {
		if err := world.ValidateSegment(parts[4]); err != nil {
			return b, err
		}
	}
	safe, err := world.SafePath(root, parts...)
	if err != nil {
		return b, err
	}
	repo, err := world.SafePath(root, triple...)
	if err != nil {
		return b, err
	}
	if !world.IsCompanyDir(filepath.Join(root, triple[0])) || !world.IsProductDir(filepath.Join(root, triple[0], triple[1])) || !world.IsRepoDir(repo) || !world.IsRepoDir(safe) {
		return b, fmt.Errorf("repository is not registered in HQ")
	}
	b.CompanyID = triple[0]
	b.RepoID = strings.Join(triple, "/")
	b.EntityRef = b.RepoID
	b.Kind = "repo"
	if worktree {
		b.Kind = "worktree"
	}
	b.WorktreePath = safe
	b.HomeKey = filepath.Join("work", b.RepoID, "main")
	if worktree {
		b.HomeKey = filepath.Join("work", b.RepoID, "worktree-"+parts[4])
	}
	return b, nil
}

// Preserve existing Work/Research labels across an in-place upgrade. The reserved
// dot prefix cannot collide with a registered company slug.
func containerScope(b Binding) string {
	if b.Preset == "ordinary" {
		return ".ordinary"
	}
	return b.CompanyID
}
