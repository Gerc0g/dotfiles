package workspace

import (
	"os"
	"path/filepath"

	"github.com/Gerc0g/dotfiles/core/editor"
	"github.com/Gerc0g/dotfiles/core/world"
)

// SyncEditorProjects mirrors the current world and worktree pool into the
// editor's project list. Call sites treat it as best effort: a pool change
// must not fail because VS Code's configuration is unavailable.
func (m *Manager) SyncEditorProjects() error {
	tree, err := world.Scan(m.root)
	if err != nil {
		return err
	}
	list, err := m.List()
	if err != nil {
		return err
	}
	path, err := editor.DefaultProjectsFile()
	if err != nil {
		return err
	}

	desired := editor.FromTree(tree, true, true)
	desired = append(desired, editor.FromWorktrees(EditorWorktrees(list))...)

	_, err = editor.Sync(path, desired, editor.DefaultOptions())
	return err
}

// EditorWorktrees converts live workspaces for the editor package. A worktree
// without .git is a corpse and stays out of the project list.
func EditorWorktrees(list []Workspace) []editor.Worktree {
	out := make([]editor.Worktree, 0, len(list))
	for _, w := range list {
		if _, err := os.Stat(filepath.Join(w.Path, ".git")); err != nil {
			continue
		}
		out = append(out, editor.Worktree{
			Path:    w.Path,
			ID:      w.ID,
			Company: w.Company,
			Product: w.Product,
			Repo:    w.Repo,
			Task:    w.Task,
			Branch:  w.Branch,
			State:   string(w.State),
		})
	}
	return out
}
