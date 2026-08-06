package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Salvage copies artifacts that exist nowhere else out of a worktree before it
// dies: second-model answers under .agents/oracle, and uncommitted task notes
// under docs/epics. Both live only in the worktree, so removing it without this
// step destroys them.
//
// Salvage never deletes anything itself — dropSalvaged does that, and only for
// what was successfully copied.
func (m *Manager) Salvage(w Workspace) (int, error) {
	if w.Company == "" || w.Product == "" || w.Repo == "" || w.ID == "" {
		return 0, nil
	}

	dest := filepath.Join(m.vault, w.Company, w.Product, "repos", w.Repo, "_salvage", w.ID)
	saved := 0

	oracle, _ := filepath.Glob(filepath.Join(w.Path, ".agents", "oracle", "*.md"))
	for _, src := range oracle {
		if err := copyInto(src, filepath.Join(dest, "oracle", filepath.Base(src))); err != nil {
			return saved, err
		}
		saved++
	}

	for _, rel := range changedEpics(w.Path) {
		src := filepath.Join(w.Path, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyInto(src, filepath.Join(dest, rel)); err != nil {
			return saved, err
		}
		saved++
	}

	if saved == 0 {
		return 0, nil
	}
	return saved, writeSalvageInfo(dest, w, saved, m.now().Format("2006-01-02 15:04:05"))
}

// changedEpics lists modified or untracked docs/epics/*.md paths.
func changedEpics(wtDir string) []string {
	lines, err := gitLines(wtDir, "status", "--porcelain", "--", "docs/epics/*.md")
	if err != nil {
		return nil
	}

	var out []string
	for _, line := range lines {
		if len(line) > 3 {
			out = append(out, strings.TrimSpace(line[3:]))
		}
	}
	return out
}

// dropSalvaged removes what salvage already copied out, so a finished worktree
// stops counting as dirty. Tracked files are never touched: `git clean` with
// -x removes untracked and ignored entries only, and the epic sweep looks at
// untracked files alone.
func dropSalvaged(wtDir string) {
	_, _ = git(wtDir, "clean", "-fdxq", "--", ".agents")

	lines, err := gitLines(wtDir, "status", "--porcelain", "--", "docs/epics/*.md")
	if err != nil {
		return
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "??") && len(line) > 3 {
			_ = os.Remove(filepath.Join(wtDir, strings.TrimSpace(line[3:])))
		}
	}
}

func copyInto(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("создание %s: %w", filepath.Dir(dst), err)
	}

	body, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("чтение %s: %w", src, err)
	}
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		return fmt.Errorf("запись %s: %w", dst, err)
	}
	return nil
}

// writeSalvageInfo leaves a note for the curator explaining what this pile is.
func writeSalvageInfo(dest string, w Workspace, saved int, when string) error {
	body := fmt.Sprintf(`# salvage: %s/%s/%s @ %s

branch: %s
task: %s
salvaged_at: %s
files: %d

Куратор: разобрать при синхронизации памяти — ценное перенести в уроки,
остальное удалить.
`, w.Company, w.Product, w.Repo, w.ID, w.Branch, w.Task, when, saved)

	path := filepath.Join(dest, "INFO.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("запись %s: %w", path, err)
	}
	return nil
}
