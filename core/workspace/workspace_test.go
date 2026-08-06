package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newFixture builds a workspace root with one real git repository, so the tests
// exercise git itself rather than a mock of it.
func newFixture(t *testing.T) (*Manager, string) {
	t.Helper()

	root := t.TempDir()
	repoDir := filepath.Join(root, "acme", "product", "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	company := filepath.Join(root, "acme", ".company-config")
	if err := os.WriteFile(company, []byte("slug: acme\ngit_email: dev@acme.test\n"), 0o644); err != nil {
		t.Fatalf("company config: %v", err)
	}
	product := filepath.Join(root, "acme", "product", ".product-config")
	if err := os.WriteFile(product, []byte("slug: product\n"), 0o644); err != nil {
		t.Fatalf("product config: %v", err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "test")
	run("config", "user.email", "test@test.test")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# repo\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	run("add", "README.md")
	run("commit", "-qm", "init")

	manager, err := New(root,
		WithVault(filepath.Join(root, "vault")),
		WithProjectSync(func() {}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return manager, repoDir
}

func TestStartCreatesIsolatedWorktree(t *testing.T) {
	m, _ := newFixture(t)

	w, err := m.Start("acme", "product", "repo", "Fix Timeouts!!")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if w.Task != "fix-timeouts" {
		t.Errorf("task = %q, ожидался fix-timeouts", w.Task)
	}
	if !strings.HasPrefix(w.Branch, "agent/fix-timeouts-") {
		t.Errorf("branch = %q", w.Branch)
	}
	if w.State != StateActive {
		t.Errorf("state = %q, ожидался active", w.State)
	}
	if got := currentBranch(w.Path); got != w.Branch {
		t.Errorf("в worktree ветка %q, ожидалась %q", got, w.Branch)
	}

	// Git identity must come from the company config, not the personal default.
	email, err := git(w.Path, "config", "user.email")
	if err != nil || email != "dev@acme.test" {
		t.Errorf("user.email = %q (%v), ожидался dev@acme.test", email, err)
	}
}

func TestStartMarksMetadataNeverDirty(t *testing.T) {
	m, _ := newFixture(t)

	w, err := m.Start("acme", "product", "repo", "task")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// The metadata file itself must not make the worktree dirty, or it would
	// pin the workspace in "has uncommitted changes" forever.
	clean, err := isClean(w.Path)
	if err != nil {
		t.Fatalf("isClean: %v", err)
	}
	if !clean {
		out, _ := git(w.Path, "status", "--short")
		t.Errorf("свежий worktree грязный:\n%s", out)
	}
}

func TestTwoWorkspacesOnOneRepo(t *testing.T) {
	m, _ := newFixture(t)

	first, err := m.Start("acme", "product", "repo", "one")
	if err != nil {
		t.Fatalf("первый Start: %v", err)
	}
	second, err := m.Start("acme", "product", "repo", "two")
	if err != nil {
		t.Fatalf("второй Start: %v", err)
	}

	if first.ID == second.ID || first.Path == second.Path {
		t.Error("два воркспейса получили один id — параллельные задачи столкнутся")
	}

	all, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List вернул %d, ожидалось 2", len(all))
	}
}

// The gate that matters most: an active workspace is never removed by cleanup,
// no matter how clean it looks. This is the regression that once deleted work.
func TestCleanupRefusesActiveWorkspace(t *testing.T) {
	m, _ := newFixture(t)

	w, err := m.Start("acme", "product", "repo", "task")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	results, err := m.Cleanup(CleanupOptions{Days: 0})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if len(results) != 1 || results[0].Removed {
		t.Fatalf("активный воркспейс удалён: %+v", results)
	}
	if results[0].Reason != "не помечен ready" {
		t.Errorf("причина = %q", results[0].Reason)
	}
	if _, err := os.Stat(w.Path); err != nil {
		t.Error("каталог воркспейса исчез")
	}
}

func TestCleanupRefusesDirtyReadyWorkspace(t *testing.T) {
	m, _ := newFixture(t)

	w, err := m.Start("acme", "product", "repo", "task")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := os.WriteFile(filepath.Join(w.Path, "README.md"), []byte("правки\n"), 0o644); err != nil {
		t.Fatalf("правка: %v", err)
	}
	if _, err := m.Ready("acme", "product", "repo", w.ID); err != nil {
		t.Fatalf("Ready: %v", err)
	}

	results, err := m.Cleanup(CleanupOptions{Days: 0})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if results[0].Removed {
		t.Fatal("воркспейс с незакоммиченными правками удалён")
	}
	if results[0].Reason != "есть незакоммиченное" {
		t.Errorf("причина = %q", results[0].Reason)
	}
}

// Without an upstream nothing can be pushed, so removal would destroy the only
// copy of the commits.
func TestCleanupRefusesWorkspaceWithoutUpstream(t *testing.T) {
	m, _ := newFixture(t)

	w, err := m.Start("acme", "product", "repo", "task")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := m.Ready("acme", "product", "repo", w.ID); err != nil {
		t.Fatalf("Ready: %v", err)
	}

	results, err := m.Cleanup(CleanupOptions{Days: 0})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if results[0].Removed {
		t.Fatal("воркспейс без upstream удалён — коммиты существовали только в нём")
	}
	if results[0].Reason != "нет upstream" {
		t.Errorf("причина = %q", results[0].Reason)
	}
}

func TestCleanupRespectsAgeGate(t *testing.T) {
	m, _ := newFixture(t)

	w, err := m.Start("acme", "product", "repo", "task")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := m.Ready("acme", "product", "repo", w.ID); err != nil {
		t.Fatalf("Ready: %v", err)
	}

	results, err := m.Cleanup(CleanupOptions{Days: 7})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if results[0].Removed || !strings.Contains(results[0].Reason, "моложе") {
		t.Errorf("возрастной гейт не сработал: %+v", results[0])
	}
}

func TestSalvageCopiesArtifactsOutOfWorktree(t *testing.T) {
	m, _ := newFixture(t)

	w, err := m.Start("acme", "product", "repo", "task")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	oracle := filepath.Join(w.Path, ".agents", "oracle")
	if err := os.MkdirAll(oracle, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oracle, "answer.md"), []byte("ответ\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	epics := filepath.Join(w.Path, "docs", "epics")
	if err := os.MkdirAll(epics, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(epics, "plan.md"), []byte("план\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	saved, err := m.Salvage(w)
	if err != nil {
		t.Fatalf("Salvage: %v", err)
	}
	if saved != 2 {
		t.Errorf("спасено %d файлов, ожидалось 2", saved)
	}

	dest := filepath.Join(m.vault, "acme", "product", "repos", "repo", "_salvage", w.ID)
	for _, rel := range []string{"oracle/answer.md", "docs/epics/plan.md", "INFO.md"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("не спасено: %s", rel)
		}
	}
}

func TestStaleUsesClockNotWallTime(t *testing.T) {
	m, _ := newFixture(t)

	if _, err := m.Start("acme", "product", "repo", "task"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if stale, err := m.Stale(3); err != nil || len(stale) != 0 {
		t.Fatalf("свежий воркспейс попал в stale: %v %v", stale, err)
	}

	m.now = func() time.Time { return time.Now().Add(10 * 24 * time.Hour) }
	stale, err := m.Stale(3)
	if err != nil {
		t.Fatalf("Stale: %v", err)
	}
	if len(stale) != 1 {
		t.Fatalf("stale вернул %d, ожидался 1", len(stale))
	}
	if stale[0].IdleDays < 3 {
		t.Errorf("idle = %d", stale[0].IdleDays)
	}
}

func TestPruneBranchKeepsUnmergedWork(t *testing.T) {
	m, repoDir := newFixture(t)

	w, err := m.Start("acme", "product", "repo", "task")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Commit inside the worktree, then detach the branch by removing the tree.
	if err := os.WriteFile(filepath.Join(w.Path, "new.md"), []byte("работа\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := git(w.Path, "add", "new.md"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := git(w.Path, "commit", "-qm", "работа"); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := git(repoDir, "worktree", "remove", "--force", w.Path); err != nil {
		t.Fatalf("worktree remove: %v", err)
	}

	msg, err := m.PruneBranch(repoDir, w.Branch, false)
	if err != nil {
		t.Fatalf("PruneBranch: %v", err)
	}
	if !strings.Contains(msg, "оставлена") {
		t.Errorf("ветка с неслитой работой не сохранена: %q", msg)
	}
	if !gitOK(repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+w.Branch) {
		t.Error("ветка удалена, хотя работа не слита и не запушена")
	}
}

func TestPruneBranchSkipsBranchInUse(t *testing.T) {
	m, repoDir := newFixture(t)

	w, err := m.Start("acme", "product", "repo", "task")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	msg, err := m.PruneBranch(repoDir, w.Branch, false)
	if err != nil {
		t.Fatalf("PruneBranch: %v", err)
	}
	if msg != "" {
		t.Errorf("ветка живого воркспейса рассматривалась: %q", msg)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Fix Timeouts":     "fix-timeouts",
		"  ticket/QA-42  ": "ticket-qa-42",
		"a---b":            "a-b",
		"---":              "",
		"keep.dots_and-1":  "keep.dots_and-1",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestFindRejectsUnmanagedPath(t *testing.T) {
	m, _ := newFixture(t)

	if _, err := m.Find("acme", "product", "repo", "nope"); err == nil {
		t.Error("Find принял несуществующий воркспейс")
	}
}
