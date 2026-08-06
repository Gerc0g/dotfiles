package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PruneBranch deletes one agent/* branch, but only when no work can be lost:
//
//   - a branch checked out by some worktree is never touched;
//   - it is deleted when merged into the base branch, or fully pushed upstream;
//   - anything else is kept, and the reason is reported.
//
// The returned string describes what happened, or is empty when the branch was
// out of scope entirely.
func (m *Manager) PruneBranch(repoDir, branch string, dryRun bool) (string, error) {
	if branch == "" || !strings.HasPrefix(branch, "agent/") {
		return "", nil
	}
	if !gitOK(repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch) {
		return "", nil
	}
	if branchInUse(repoDir, branch) {
		return "", nil
	}

	reason := m.pruneReason(repoDir, branch)
	if reason == "" {
		return fmt.Sprintf("оставлена ветка %s (есть неслитая/незапушенная работа)", branch), nil
	}

	if dryRun {
		return fmt.Sprintf("будет удалена ветка %s (%s)", branch, reason), nil
	}
	if _, err := git(repoDir, "branch", "-D", branch); err != nil {
		return "", err
	}
	return fmt.Sprintf("удалена ветка %s (%s)", branch, reason), nil
}

// pruneReason explains why a branch is safe to delete, or returns "".
func (m *Manager) pruneReason(repoDir, branch string) string {
	if base, err := m.baseRef(repoDir); err == nil && base != orphanRef {
		if gitOK(repoDir, "merge-base", "--is-ancestor", branch, base) {
			return "влита в " + base
		}
	}

	if !gitOK(repoDir, "rev-parse", "--verify", "--quiet", branch+"@{u}") {
		return ""
	}
	if n, err := gitCount(repoDir, "rev-list", "--count", branch+"@{u}.."+branch); err == nil && n == 0 {
		upstream, _ := git(repoDir, "rev-parse", "--abbrev-ref", branch+"@{u}")
		return "полностью запушена в " + upstream
	}
	return ""
}

// branchInUse reports whether some worktree has the branch checked out.
func branchInUse(repoDir, branch string) bool {
	lines, err := gitLines(repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return false
	}
	for _, line := range lines {
		if line == "branch refs/heads/"+branch {
			return true
		}
	}
	return false
}

// PruneBranches sweeps agent/* branches across every main checkout.
func (m *Manager) PruneBranches(dryRun bool) ([]string, error) {
	repos, err := m.mainCheckouts()
	if err != nil {
		return nil, err
	}

	var out []string
	for _, repoDir := range repos {
		branches, err := gitLines(repoDir, "for-each-ref", "--format=%(refname:short)", "refs/heads/agent/")
		if err != nil {
			continue
		}
		for _, branch := range branches {
			msg, err := m.PruneBranch(repoDir, branch, dryRun)
			if err != nil || msg == "" {
				continue
			}
			rel, _ := filepath.Rel(m.root, repoDir)
			out = append(out, rel+": "+msg)
		}
	}
	return out, nil
}

// mainCheckouts lists <root>/<company>/<product>/<repo> directories that are
// git repositories.
func (m *Manager) mainCheckouts() ([]string, error) {
	pattern := filepath.Join(m.root, "*", "*", "*")
	candidates, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("поиск репозиториев: %w", err)
	}

	var out []string
	for _, dir := range candidates {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			out = append(out, dir)
		}
	}
	return out, nil
}
