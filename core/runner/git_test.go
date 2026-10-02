package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, out)
	}
	return strings.TrimSpace(string(out))
}
func gitFixture(t *testing.T) (Binding, string) {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("PROKECTFILES_ROOT", root)
	t.Setenv("HQ_DATA_ROOT", t.TempDir())
	repo := filepath.Join(root, "acme/prod/repo")
	os.MkdirAll(repo, 0700)
	runGit(t, repo, "init", "-b", "task/test")
	os.WriteFile(filepath.Join(repo, "file"), []byte("initial\n"), 0600)
	runGit(t, repo, "add", "file")
	runGit(t, repo, "commit", "-m", "initial")
	return Binding{CompanyID: "acme", RepoID: "acme/prod/repo", WorktreePath: repo, Preset: "work", HomeKey: "work/acme/prod/repo/main"}, filepath.Join(t.TempDir(), "git")
}
func TestGitSyncImportsOnlyBoundBranchAndIndexWithoutHooks(t *testing.T) {
	b, dir := gitFixture(t)
	state, e := beginGit(b, dir)
	if e != nil {
		t.Fatal(e)
	}
	marker := filepath.Join(t.TempDir(), "executed")
	os.MkdirAll(filepath.Join(dir, "hooks"), 0700)
	os.WriteFile(filepath.Join(dir, "hooks/reference-transaction"), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0700)
	os.WriteFile(filepath.Join(b.WorktreePath, "file"), []byte("changed\n"), 0600)
	t.Setenv("GIT_DIR", dir)
	t.Setenv("GIT_WORK_TREE", b.WorktreePath)
	runGit(t, b.WorktreePath, "-c", "core.hooksPath=/dev/null", "add", "file")
	runGit(t, b.WorktreePath, "-c", "core.hooksPath=/dev/null", "commit", "-m", "sandbox")
	// An uncommitted staged file must survive synchronization too.
	os.WriteFile(filepath.Join(b.WorktreePath, "staged"), []byte("stage\n"), 0600)
	runGit(t, b.WorktreePath, "-c", "core.hooksPath=/dev/null", "add", "staged")
	t.Setenv("GIT_DIR", "")
	t.Setenv("GIT_WORK_TREE", "")
	if e = finishGit(context.Background(), state); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("task hook executed as service")
	}
	os.Unsetenv("GIT_DIR")
	os.Unsetenv("GIT_WORK_TREE")
	if got := runGit(t, b.WorktreePath, "log", "-1", "--format=%s"); got != "sandbox" {
		t.Fatal(got)
	}
	if got := runGit(t, b.WorktreePath, "diff", "--cached", "--name-only"); got != "staged" {
		t.Fatal(got)
	}
}
func TestGitSyncRejectsBranchSwitchAndHostHeadRace(t *testing.T) {
	for _, change := range []string{"branch", "host"} {
		t.Run(change, func(t *testing.T) {
			b, dir := gitFixture(t)
			state, e := beginGit(b, dir)
			if e != nil {
				t.Fatal(e)
			}
			if change == "branch" {
				os.WriteFile(filepath.Join(dir, "HEAD"), []byte("ref: refs/heads/other\n"), 0600)
			} else {
				runGit(t, b.WorktreePath, "commit", "--allow-empty", "-m", "host advanced")
			}
			if e = finishGit(context.Background(), state); e == nil {
				t.Fatal("conflicting Git state was imported")
			}
			if _, e = os.Stat(dir); e != nil {
				t.Fatal("private Git recovery data lost")
			}
		})
	}
}

func TestLinkedWorktreeImportKeepsMainCheckoutAndMetadataSeparate(t *testing.T) {
	b, private := gitFixture(t)
	mainHead := runGit(t, b.WorktreePath, "rev-parse", "HEAD")
	worktree := filepath.Join(os.Getenv("PROKECTFILES_ROOT"), ".worktrees/acme/prod/repo/task-one")
	runGit(t, b.WorktreePath, "worktree", "add", "-b", "task/linked", worktree)
	mainPath := b.WorktreePath
	b.WorktreePath = worktree
	b.HomeKey = "work/acme/prod/repo/worktree-task-one"
	state, err := beginGit(b, private)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(private, "worktrees")); !os.IsNotExist(err) {
		t.Fatal("shared worktree registry exposed")
	}
	os.WriteFile(filepath.Join(worktree, "file"), []byte("linked change\n"), 0600)
	t.Setenv("GIT_DIR", private)
	t.Setenv("GIT_WORK_TREE", worktree)
	runGit(t, worktree, "add", "file")
	runGit(t, worktree, "commit", "-m", "linked task")
	os.Unsetenv("GIT_DIR")
	os.Unsetenv("GIT_WORK_TREE")
	if err = finishGit(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, mainPath, "rev-parse", "HEAD"); got != mainHead {
		t.Fatal("main checkout advanced")
	}
	if got := runGit(t, worktree, "log", "-1", "--format=%s"); got != "linked task" {
		t.Fatal(got)
	}
	// A mutable worktree marker must be checked before opening its referenced metadata.
	os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: /private/other-company/.git\n"), 0600)
	if _, _, err = gitLocations(b); err == nil {
		t.Fatal("foreign Git directory accepted")
	}
}
