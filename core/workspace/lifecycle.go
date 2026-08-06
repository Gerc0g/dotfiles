package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Ready marks a workspace finished, which is what makes Cleanup eligible to
// remove it later. Nothing else sets this: a worktree becomes removable only
// because the user said so.
func (m *Manager) Ready(company, product, repo, id string) (Workspace, error) {
	w, err := m.Find(company, product, repo, id)
	if err != nil {
		return Workspace{}, err
	}

	w.State = StateReady
	w.PushedAt = m.now().Format("2006-01-02 15:04:05")
	if branch := currentBranch(w.Path); branch != "" {
		w.Branch = branch
	}

	if err := writeMeta(w); err != nil {
		return Workspace{}, err
	}
	return w, nil
}

// Remove deletes one workspace by name. Artifacts are salvaged first, and git
// itself refuses to remove a worktree with modified tracked files — that
// refusal is the safety net and is deliberately not overridden.
func (m *Manager) Remove(company, product, repo, id string) error {
	w, err := m.Find(company, product, repo, id)
	if err != nil {
		return err
	}
	return m.destroy(w)
}

// CleanupOptions tunes a cleanup sweep.
type CleanupOptions struct {
	// Days a workspace must have been ready before it may be removed.
	Days int
	// DryRun reports what would happen and changes nothing.
	DryRun bool
}

// CleanupResult is one workspace's outcome in a sweep.
type CleanupResult struct {
	Workspace Workspace
	Removed   bool
	Reason    string
}

// Cleanup removes finished workspaces. Every gate must agree:
//
//  1. the user marked it ready, or a review exists and its pull request merged;
//  2. it has been in that state for at least Days;
//  3. the worktree is clean, tracks an upstream, and has nothing unpushed.
//
// Any gate that fails leaves the workspace alone. This is why an abandoned
// worktree can pile up — that is the intended trade: clutter over data loss.
func (m *Manager) Cleanup(opts CleanupOptions) ([]CleanupResult, error) {
	if opts.Days < 0 {
		return nil, fmt.Errorf("--days должен быть неотрицательным")
	}

	all, err := m.List()
	if err != nil {
		return nil, err
	}

	var results []CleanupResult
	for _, w := range all {
		if reason, ok := m.eligible(w, opts.Days); !ok {
			results = append(results, CleanupResult{Workspace: w, Reason: reason})
			continue
		}

		if opts.DryRun {
			results = append(results, CleanupResult{Workspace: w, Removed: true, Reason: "будет удалён"})
			continue
		}

		if err := m.destroy(w); err != nil {
			results = append(results, CleanupResult{Workspace: w, Reason: err.Error()})
			continue
		}
		results = append(results, CleanupResult{Workspace: w, Removed: true, Reason: "удалён"})
	}

	if !opts.DryRun {
		m.syncProjects()
	}
	return results, nil
}

// eligible applies the cleanup gates and explains the first failure.
func (m *Manager) eligible(w Workspace, days int) (string, bool) {
	switch w.State {
	case StateReady:
	case StateReview:
		if !reviewMerged(w.ReviewURL) {
			return "review ещё не влит", false
		}
	default:
		return "не помечен ready", false
	}

	if days > 0 {
		info, err := os.Stat(filepath.Join(w.Path, MetaFile))
		if err != nil {
			return "нет метаданных", false
		}
		if m.now().Sub(info.ModTime()) < time.Duration(days)*24*time.Hour {
			return fmt.Sprintf("моложе %d дней", days), false
		}
	}

	// Salvage before judging cleanliness: it drops the untracked artifacts it
	// copied out, which is often what stands between "dirty" and removable.
	_, _ = m.Salvage(w)
	dropSalvaged(w.Path)

	clean, err := isClean(w.Path)
	if err != nil {
		return err.Error(), false
	}
	if !clean {
		return "есть незакоммиченное", false
	}
	if !hasUpstream(w.Path) {
		return "нет upstream", false
	}
	if n, err := unpushed(w.Path); err != nil || n > 0 {
		return "есть незапушенное", false
	}

	return "", true
}

// destroy salvages, removes the worktree and prunes its branch.
func (m *Manager) destroy(w Workspace) error {
	if _, err := m.Salvage(w); err != nil {
		return err
	}
	dropSalvaged(w.Path)

	repoDir := m.RepoDir(w.Company, w.Product, w.Repo)
	if _, err := git(repoDir, "worktree", "remove", w.Path); err != nil {
		return err
	}

	_, _ = m.PruneBranch(repoDir, w.Branch, false)
	return nil
}

// StaleInfo describes an active workspace nobody has touched for a while.
type StaleInfo struct {
	Workspace Workspace
	IdleDays  int
	Dirty     int
	Unpushed  int
}

// Stale lists active workspaces with no commits for at least days. Cleanup
// never touches these — they may hold unpushed work — so they accumulate
// silently, and this view exists for a human to triage them.
func (m *Manager) Stale(days int) ([]StaleInfo, error) {
	if days < 0 {
		return nil, fmt.Errorf("days должен быть неотрицательным")
	}

	all, err := m.List()
	if err != nil {
		return nil, err
	}

	var out []StaleInfo
	for _, w := range all {
		if w.State != StateActive {
			continue
		}

		idle := m.idleDays(w)
		if idle < days {
			continue
		}

		info := StaleInfo{Workspace: w, IdleDays: idle, Dirty: dirtyCount(w.Path)}
		if n, err := unpushed(w.Path); err == nil {
			info.Unpushed = n
		}
		out = append(out, info)
	}
	return out, nil
}

// idleDays measures from the last commit, falling back to the metadata file.
func (m *Manager) idleDays(w Workspace) int {
	if out, err := git(w.Path, "log", "-1", "--format=%ct"); err == nil && out != "" {
		var epoch int64
		if _, err := fmt.Sscanf(out, "%d", &epoch); err == nil && epoch > 0 {
			return int(m.now().Sub(time.Unix(epoch, 0)).Hours() / 24)
		}
	}

	info, err := os.Stat(filepath.Join(w.Path, MetaFile))
	if err != nil {
		return 0
	}
	return int(m.now().Sub(info.ModTime()).Hours() / 24)
}

// reviewMerged asks gh whether a pull request merged. A missing gh, an
// unreachable API or any other doubt reports false, so the workspace survives.
func reviewMerged(url string) bool {
	if url == "" {
		return false
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return false
	}

	out, err := exec.Command("gh", "pr", "view", url, "--json", "state", "-q", ".state").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "MERGED"
}
