package workspace

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeCommitRepo builds a workspace root with one repo that has an initial
// commit and a configured identity.
func makeCommitRepo(t *testing.T) (root, repo string) {
	t.Helper()
	root = t.TempDir()

	repo = filepath.Join(root, "acme", "shop", "backend")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(root, "acme", ".company-config"):         "git_email: dev@acme.test\n",
		filepath.Join(root, "acme", "shop", ".product-config"): "slug: shop\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "test")
	run("config", "user.email", "dev@acme.test")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-q", "-m", "init")

	return root, repo
}

func TestCommitRejectsBadInput(t *testing.T) {
	root, repo := makeCommitRepo(t)

	err := Commit(root, repo, CommitOptions{Message: "просто текст", Paths: []string{"a.txt"}})
	if err == nil || !strings.Contains(err.Error(), "Conventional Commit") {
		t.Errorf("non-conventional message must fail, got: %v", err)
	}

	err = Commit(root, repo, CommitOptions{Message: "fix: x", Paths: []string{"."}})
	if err == nil || !strings.Contains(err.Error(), "запрещён") {
		t.Errorf("broad path must fail, got: %v", err)
	}

	err = Commit(root, repo, CommitOptions{Message: "fix: x", Paths: []string{"ghost.txt"}})
	if err == nil || !strings.Contains(err.Error(), "не отслеживается") {
		t.Errorf("missing path must fail, got: %v", err)
	}
}

func TestCommitIdentityMismatch(t *testing.T) {
	root, repo := makeCommitRepo(t)

	cmd := exec.Command("git", "config", "user.email", "wrong@example.com")
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	err := Commit(root, repo, CommitOptions{Message: "fix: x", Paths: []string{"a.txt"}})
	if err == nil || !strings.Contains(err.Error(), "user.email") {
		t.Errorf("identity mismatch must fail, got: %v", err)
	}
}

func TestCommitOnlyNamedPaths(t *testing.T) {
	root, repo := makeCommitRepo(t)

	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Commit(root, repo, CommitOptions{Message: "fix(a): правка a", Paths: []string{"a.txt"}}); err != nil {
		t.Fatal(err)
	}

	status, err := git(repo, "status", "--short")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "b.txt") {
		t.Errorf("b.txt must stay uncommitted, status: %q", status)
	}
	subject, err := git(repo, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if subject != "fix(a): правка a" {
		t.Errorf("subject: %q", subject)
	}

	// Re-running with no changes left is an outcome, not an error.
	err = Commit(root, repo, CommitOptions{Message: "fix(a): повтор", Paths: []string{"a.txt"}})
	if !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("want ErrNothingToCommit, got: %v", err)
	}
}

func TestFinishGuards(t *testing.T) {
	_, repo := makeCommitRepo(t)

	// Not a managed worktree → finish refuses.
	err := Finish(repo, FinishOptions{})
	if err == nil || !strings.Contains(err.Error(), MetaFile) {
		t.Errorf("finish outside worktree must fail, got: %v", err)
	}

	// NoReview from main without the env unlock → refuses.
	err = Finish(repo, FinishOptions{NoReview: true})
	if err == nil || !strings.Contains(err.Error(), "интеграционную") {
		t.Errorf("no-review from main must fail, got: %v", err)
	}
}
