package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

// ContextIssue is a visible context gap. Missing documentation or optional
// memory is a warning; an occupied adapter path is an error, never overwritten.
type ContextIssue struct {
	Path     string `json:"path"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type contextEntry struct {
	source, dest, kind, level string
	required                  bool
}

// ContextIssues checks one managed worktree without changing any files.
func (m *Manager) ContextIssues(w Workspace) ([]ContextIssue, error) {
	return m.materializeContext(w, false)
}

// RepairContext restores absent context only. Parent references remain live;
// untracked repo instructions are seeded as independent branch-local snapshots.
// Neither custom files nor tracked deletions are replaced. Missing source
// context is allowed and returned as warnings, so agents can help complete it.
func (m *Manager) RepairContext(w Workspace) ([]ContextIssue, error) {
	return m.materializeContext(w, true)
}

func (m *Manager) materializeContext(w Workspace, repair bool) ([]ContextIssue, error) {
	for _, segment := range []string{w.Company, w.Product, w.Repo, w.ID} {
		if err := world.ValidateSegment(segment); err != nil {
			return nil, err
		}
	}
	expected, err := world.SafePath(m.root, ".worktrees", w.Company, w.Product, w.Repo, w.ID)
	if err != nil {
		return nil, err
	}
	_, actual, err := normalize(m.root, w.Path)
	if err != nil {
		return nil, err
	}
	if actual != expected {
		return nil, fmt.Errorf("worktree не соответствует пути HQ: %s", w.Path)
	}
	if _, err := m.Find(w.Company, w.Product, w.Repo, w.ID); err != nil {
		return nil, err
	}
	if !isRepo(w.Path) {
		return nil, fmt.Errorf("не git worktree: %s", w.Path)
	}
	repoDir, err := world.SafePath(m.root, w.Company, w.Product, w.Repo)
	if err != nil {
		return nil, err
	}
	productDir, companyDir := filepath.Dir(repoDir), filepath.Dir(filepath.Dir(repoDir))
	bucket := filepath.Dir(w.Path)
	var entries []contextEntry
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		for _, parent := range []struct{ source, dest, level string }{
			{companyDir, filepath.Dir(bucket), "company"},
			{productDir, bucket, "product"},
		} {
			entries = append(entries, contextEntry{
				source: filepath.Join(parent.source, name), dest: filepath.Join(parent.dest, name),
				kind: "reference", level: parent.level, required: name == "AGENTS.md",
			})
		}
		entries = append(entries, contextEntry{
			source: filepath.Join(repoDir, name), dest: filepath.Join(w.Path, name),
			kind: "snapshot", required: name == "AGENTS.md",
		})
	}
	entries = append(entries, contextEntry{source: filepath.Join(productDir, "docs"), dest: filepath.Join(bucket, "docs"), kind: "link"})
	var issues []ContextIssue
	var failures []error
	for _, entry := range entries {
		issue, err := inspectContextEntry(w.Path, entry, repair)
		if issue != nil {
			issues = append(issues, *issue)
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	memoryIssues, err := inspectKnowledge(repoDir, w.Path, repair)
	issues = append(issues, memoryIssues...)
	if err != nil {
		failures = append(failures, err)
	}
	if info, err := os.Stat(filepath.Join(w.Path, "docs", "knowledge", "hot.md")); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		issues = append(issues, ContextIssue{
			Path: filepath.Join(w.Path, "docs", "knowledge", "hot.md"), Severity: "warning",
			Message: "WikiPedik не подключён или hot.md пуст; память необязательна для запуска",
		})
	}
	return issues, errors.Join(failures...)
}

func inspectContextEntry(wtDir string, entry contextEntry, repair bool) (*ContextIssue, error) {
	issue := func(severity, message string) *ContextIssue {
		return &ContextIssue{Path: entry.dest, Severity: severity, Message: message}
	}
	info, statErr := os.Lstat(entry.dest)
	if entry.kind == "snapshot" {
		if statErr == nil {
			if resolved, err := os.Stat(entry.dest); err != nil || !resolved.Mode().IsRegular() {
				return issue("warning", "существующий контекст недоступен; файл сохранён"), nil
			}
			if content, err := os.ReadFile(entry.dest); err != nil || len(strings.TrimSpace(string(content))) == 0 {
				return issue("warning", "существующий контекст пуст или не читается; файл сохранён"), nil
			}
			return nil, nil // branch-owned content, even when different from main
		}
		name := filepath.Base(entry.dest)
		if gitOK(wtDir, "ls-files", "--error-unmatch", "--", name) || gitOK(wtDir, "cat-file", "-e", "HEAD:"+name) {
			return issue("warning", "tracked контекст удалён в worktree; удаление сохранено"), nil
		}
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		return issue("error", statErr.Error()), statErr
	}
	sourceInfo, sourceErr := os.Stat(entry.source)
	if sourceErr != nil {
		if os.IsNotExist(sourceErr) {
			if entry.required {
				return issue("warning", "нет канонического контекста: "+entry.source), nil
			}
			return nil, nil
		}
		return issue("error", sourceErr.Error()), sourceErr
	}
	if (entry.kind == "link" && !sourceInfo.IsDir()) || (entry.kind != "link" && !sourceInfo.Mode().IsRegular()) {
		err := fmt.Errorf("неожиданный тип источника контекста: %s", entry.source)
		return issue("error", err.Error()), err
	}
	var content []byte
	if entry.kind == "reference" {
		content = []byte(parentContextReference(entry.level, entry.source))
	}
	if statErr == nil {
		if entry.kind == "link" && contextLinkMatches(entry.dest, entry.source) {
			return nil, nil
		}
		if entry.kind == "reference" && info.Mode().IsRegular() {
			if existing, err := os.ReadFile(entry.dest); err == nil && string(existing) == string(content) {
				return nil, nil
			}
		}
		err := fmt.Errorf("путь контекста занят; существующее содержимое сохранено: %s", entry.dest)
		return issue("error", err.Error()), err
	}
	if !repair {
		return issue("warning", "контекст не материализован; выполните hq workspace context-repair"), nil
	}
	var err error
	if entry.kind == "link" {
		err = ensureContextLink(entry.source, entry.dest)
	} else {
		if entry.kind == "snapshot" {
			content, err = os.ReadFile(entry.source)
		}
		if err == nil {
			err = writeContextFile(entry.dest, content)
		}
	}
	if err != nil {
		return issue("error", err.Error()), err
	}
	return nil, nil
}

func parentContextReference(level, source string) string {
	return fmt.Sprintf("<!-- hq-worktree-context v1 -->\n# HQ %s context\n\n"+
		"Canonical instructions: [read %s](<%s>).\n\n"+
		"This generated reference contains no copied instructions. Read the canonical file when its scope applies.\n"+
		"Resolve that file's relative links from `%s`, not from this worktree pool.\n", level, filepath.Base(source), source, filepath.Dir(source))
}

func writeContextFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			if existing, readErr := os.ReadFile(path); readErr == nil && string(existing) == string(content) {
				return nil
			}
		}
		return err
	}
	_, writeErr := file.Write(content)
	return errors.Join(writeErr, file.Close())
}

func resolvedLinkTarget(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target), nil
}

func contextLinkMatches(path, target string) bool {
	current, err := resolvedLinkTarget(path)
	if err != nil {
		return false
	}
	if current == filepath.Clean(target) {
		return true
	}
	resolvedCurrent, currentErr := filepath.EvalSymlinks(current)
	resolvedTarget, targetErr := filepath.EvalSymlinks(target)
	return currentErr == nil && targetErr == nil && resolvedCurrent == resolvedTarget
}

// ensureContextLink is deliberately non-destructive, including dangling links.
func ensureContextLink(target, path string) error {
	if _, err := os.Lstat(path); err == nil {
		if contextLinkMatches(path, target) {
			return nil
		}
		return fmt.Errorf("путь контекста занят; файл сохранён: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.Symlink(target, path); err != nil {
		// Two starts for the same repo may materialize a shared parent together.
		if os.IsExist(err) && contextLinkMatches(path, target) {
			return nil
		}
		return err
	}
	return nil
}

// ContextSummary formats issues for interactive CLI output.
func ContextSummary(issues []ContextIssue) string {
	var out strings.Builder
	for _, issue := range issues {
		fmt.Fprintf(&out, "%s: %s: %s\n", issue.Severity, issue.Path, issue.Message)
	}
	return out.String()
}
