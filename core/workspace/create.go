package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

// excludeLines never count as "dirty" in a managed worktree. Without them a
// worktree pins itself in a modified state forever and can never be cleaned up:
// the metadata file, the runtime scratch directory, and the memory symlinks are
// all untracked by design.
var excludeLines = []string{
	MetaFile,
	".agents/",
	"docs/knowledge",
	"docs/product-knowledge",
	"docs/company-knowledge",
}

// knowledgeLinks are the vault symlinks mirrored from the main checkout.
var knowledgeLinks = []string{"knowledge", "product-knowledge", "company-knowledge"}

// Start creates a worktree for one task and returns it.
func (m *Manager) Start(company, product, repo, task string) (Workspace, error) {
	return m.StartWithRequestID(company, product, repo, task, "")
}

// CreationNotStartedError proves that no worktree creation command was issued.
type CreationNotStartedError struct{ Err error }

func (e *CreationNotStartedError) Error() string { return e.Err.Error() }
func (e *CreationNotStartedError) Unwrap() error { return e.Err }

// StartWithRequestID persists the caller's opaque identity independently of the task slug.
func (m *Manager) StartWithRequestID(company, product, repo, task, requestID string) (result Workspace, err error) {
	creationIssued := false
	defer func() {
		if err != nil && !creationIssued {
			err = &CreationNotStartedError{Err: err}
		}
	}()
	if strings.TrimSpace(requestID) != requestID || strings.ContainsAny(requestID, "\r\n\x00=") {
		return Workspace{}, fmt.Errorf("invalid workspace request identity")
	}
	if requestID != "" {
		all, listErr := m.List()
		if listErr != nil {
			return Workspace{}, listErr
		}
		for _, existing := range all {
			if existing.CreationRequestID != requestID {
				continue
			}
			if existing.Company != company || existing.Product != product || existing.Repo != repo || existing.Task != Slugify(task) {
				return Workspace{}, fmt.Errorf("workspace request identity belongs to another task")
			}
			return existing, nil
		}
	}
	slug := Slugify(task)
	if slug == "" {
		return Workspace{}, fmt.Errorf("пустой слаг задачи")
	}

	repoDir := m.RepoDir(company, product, repo)
	if !isRepo(repoDir) {
		return Workspace{}, fmt.Errorf("не git-репозиторий: %s", repoDir)
	}

	id, branch, wtDir, err := m.freeSlot(repoDir, company, product, repo, slug)
	if err != nil {
		return Workspace{}, err
	}

	if err := os.MkdirAll(filepath.Dir(wtDir), 0o755); err != nil {
		return Workspace{}, fmt.Errorf("создание %s: %w", filepath.Dir(wtDir), err)
	}

	baseRef, err := m.baseRef(repoDir)
	if err != nil {
		return Workspace{}, err
	}

	creationIssued = true
	if baseRef == orphanRef {
		if _, err := git(repoDir, "worktree", "add", "--quiet", "--orphan", "-b", branch, wtDir); err != nil {
			return Workspace{}, err
		}
		if err := seedOrphan(repoDir, wtDir); err != nil {
			return Workspace{}, err
		}
	} else {
		if _, err := git(repoDir, "worktree", "add", "--quiet", wtDir, "-b", branch, baseRef); err != nil {
			return Workspace{}, err
		}
	}

	if err := m.configureIdentity(company, wtDir); err != nil {
		return Workspace{}, err
	}
	if err := ensureExcludes(wtDir); err != nil {
		return Workspace{}, err
	}
	w := Workspace{
		Path:              wtDir,
		ID:                id,
		Company:           company,
		Product:           product,
		Repo:              repo,
		Task:              slug,
		CreationRequestID: requestID,
		Branch:            branch,
		BaseRef:           baseRef,
		State:             StateActive,
		CreatedAt:         m.now().Format("2006-01-02 15:04:05"),
	}
	if err := writeMeta(w); err != nil {
		return Workspace{}, err
	}
	issues, err := m.RepairContext(w)
	for _, issue := range issues {
		if m.warnings != nil {
			fmt.Fprintf(m.warnings, "context %s: %s: %s\n", issue.Severity, issue.Path, issue.Message)
		}
	}
	if err != nil {
		return w, fmt.Errorf("worktree создан в %s; контекст требует исправления: %w", w.Path, err)
	}

	m.syncProjects()
	return w, nil
}

// freeSlot picks an id whose directory and branch are both unused.
func (m *Manager) freeSlot(repoDir, company, product, repo, slug string) (id, branch, dir string, err error) {
	for attempt := 0; attempt < 16; attempt++ {
		id, err = shortID()
		if err != nil {
			return "", "", "", err
		}
		branch = "agent/" + slug + "-" + id
		dir = m.WorktreeDir(company, product, repo, id)

		_, statErr := os.Stat(dir)
		branchExists := gitOK(repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)

		if os.IsNotExist(statErr) && !branchExists {
			return id, branch, dir, nil
		}
	}
	return "", "", "", fmt.Errorf("не удалось подобрать свободный id для %s", slug)
}

const orphanRef = "__orphan__"

// baseRef picks what a new branch forks from: origin/dev, then origin/main,
// then their local counterparts, then whatever is checked out.
func (m *Manager) baseRef(repoDir string) (string, error) {
	if !hasCommits(repoDir) {
		return orphanRef, nil
	}

	// Best effort: a stale origin/dev would fork from an old base.
	_, _ = git(repoDir, "fetch", "origin", "--prune")

	for _, ref := range []string{"refs/remotes/origin/dev", "refs/remotes/origin/main", "refs/heads/dev", "refs/heads/main"} {
		if gitOK(repoDir, "show-ref", "--verify", "--quiet", ref) {
			return strings.TrimPrefix(strings.TrimPrefix(ref, "refs/remotes/"), "refs/heads/"), nil
		}
	}

	branch := currentBranch(repoDir)
	if branch == "" {
		return "", fmt.Errorf("не удалось определить базовую ветку для %s", repoDir)
	}
	return branch, nil
}

// configureIdentity sets the git author from the company config, so commits in
// a client repository are not signed with a personal address.
func (m *Manager) configureIdentity(company, wtDir string) error {
	if _, err := git(wtDir, "config", "user.name", "egor"); err != nil {
		return err
	}

	tree, err := world.Scan(m.root)
	if err != nil {
		return nil
	}
	for _, c := range tree.Companies() {
		if c.Slug != company {
			continue
		}
		if email := c.Config.Get("git_email"); email != "" {
			if _, err := git(wtDir, "config", "user.email", email); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensureExcludes appends the never-dirty entries to the worktree's exclude file.
func ensureExcludes(wtDir string) error {
	path, err := git(wtDir, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(wtDir, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание %s: %w", filepath.Dir(path), err)
	}

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("чтение %s: %w", path, err)
	}

	present := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		present[strings.TrimSpace(line)] = true
	}

	var add strings.Builder
	for _, line := range excludeLines {
		if !present[line] {
			add.WriteString(line + "\n")
		}
	}
	if add.Len() == 0 {
		return nil
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("открытие %s: %w", path, err)
	}
	defer file.Close()

	if _, err := file.WriteString(add.String()); err != nil {
		return fmt.Errorf("запись %s: %w", path, err)
	}
	return nil
}

// linkKnowledge mirrors the vault symlinks from the main checkout. They are
// untracked by design, so a fresh worktree starts without them and the memory
// write path would silently go dark.
func linkKnowledge(repoDir, wtDir string) error {
	_, err := inspectKnowledge(repoDir, wtDir, true)
	return err
}

func inspectKnowledge(repoDir, wtDir string, repair bool) ([]ContextIssue, error) {
	var sources []string
	for _, name := range knowledgeLinks {
		sources = append(sources, filepath.Join(repoDir, "docs", name))
	}
	rules, _ := filepath.Glob(filepath.Join(repoDir, ".claude", "rules", "wiki-*"))
	sources = append(sources, rules...)
	var issues []ContextIssue
	var failures []error
	for _, source := range sources {
		target, err := resolvedLinkTarget(source)
		if err != nil {
			continue
		}
		if _, err := os.Stat(target); err != nil {
			issues = append(issues, ContextIssue{Path: source, Severity: "warning", Message: "источник WikiPedik недоступен: " + err.Error()})
			continue
		}
		rel, err := filepath.Rel(repoDir, source)
		if err != nil {
			return issues, err
		}
		link := filepath.Join(wtDir, rel)
		if contextLinkMatches(link, target) {
			continue
		}
		_, statErr := os.Lstat(link)
		if os.IsNotExist(statErr) && !repair {
			issues = append(issues, ContextIssue{Path: link, Severity: "warning", Message: "WikiPedik ссылка отсутствует; выполните hq workspace context-repair"})
			continue
		}
		if repair {
			err = ensureContextLink(target, link)
		} else {
			err = fmt.Errorf("путь контекста занят; файл сохранён: %s", link)
		}
		if err != nil {
			issues = append(issues, ContextIssue{Path: link, Severity: "error", Message: err.Error()})
			failures = append(failures, err)
		}
	}
	return issues, errors.Join(failures...)
}

// seedOrphan copies the checkout contents into an orphan worktree, which git
// creates empty.
func seedOrphan(repoDir, wtDir string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return fmt.Errorf("чтение %s: %w", repoDir, err)
	}

	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		src := filepath.Join(repoDir, entry.Name())
		if err := exec.Command("cp", "-R", src, wtDir).Run(); err != nil {
			return fmt.Errorf("копирование %s: %w", src, err)
		}
	}
	return nil
}

// shortID returns 8 hex characters.
func shortID() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("генерация id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
