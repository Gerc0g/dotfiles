package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

// Memory links are untracked by design, so a worktree only gets them if
// something puts them there. Start does that for new worktrees — but the pool
// predates that logic, and 102 of 131 existing worktrees had no
// docs/knowledge at all. Work happens in worktrees, so in all of them the
// memory was invisible: nothing to read at session start, nowhere to append a
// lesson.

// MemoryGap is one worktree that could have memory links but has none.
type MemoryGap struct {
	Worktree string
	RepoDir  string
}

// MemoryGaps lists worktrees whose main checkout carries the vault links while
// the worktree itself does not. Repos that were never bootstrapped have
// nothing to mirror and are not gaps.
//
// The pool is walked by path rather than by workspace metadata: only 29 of 239
// worktrees were created through `hq workspace`, and the hand-made ones are
// just as much a place where sessions run and lessons are written.
func (m *Manager) MemoryGaps() ([]MemoryGap, error) {
	dirs, err := filepath.Glob(filepath.Join(m.worktrees, "*", "*", "*", "*"))
	if err != nil {
		return nil, err
	}

	var gaps []MemoryGap
	for _, wtDir := range dirs {
		// A worktree always carries a .git entry; anything else in the pool is
		// leftover scaffolding, not a checkout.
		if _, err := os.Stat(filepath.Join(wtDir, ".git")); err != nil {
			continue
		}
		rel, err := filepath.Rel(m.worktrees, wtDir)
		if err != nil {
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 4 {
			continue
		}
		repoDir := m.RepoDir(parts[0], parts[1], parts[2])

		for _, name := range knowledgeLinks {
			source := filepath.Join(repoDir, "docs", name)
			if _, err := os.Readlink(source); err != nil {
				continue // nothing to mirror
			}
			if _, err := os.Stat(filepath.Join(wtDir, "docs", name)); err == nil {
				continue // already linked
			}
			gaps = append(gaps, MemoryGap{Worktree: wtDir, RepoDir: repoDir})
			break
		}
	}
	return gaps, nil
}

// RelinkMemory repairs every gap and reports how many worktrees it touched.
func (m *Manager) RelinkMemory() (int, error) {
	gaps, err := m.MemoryGaps()
	if err != nil {
		return 0, err
	}
	for _, gap := range gaps {
		linkKnowledge(gap.RepoDir, gap.Worktree)
	}
	return len(gaps), nil
}
