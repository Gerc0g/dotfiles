package wiki

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushVaultArchivesWithoutImportingRemoteChanges(t *testing.T) {
	// Each fixture uses an actual remote and two independent writers. A
	// remote-only commit must not get pulled into the Obsidian-managed vault,
	// whether the local branch is merely behind or has diverged.
	for _, diverged := range []bool{false, true} {
		name := "remote-ahead"
		if diverged {
			name = "diverged"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			// Ignore personal Git settings, signing and hooks in this fixture.
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_AUTHOR_NAME", "Archive Test")
			t.Setenv("GIT_AUTHOR_EMAIL", "archive@example.invalid")
			t.Setenv("GIT_COMMITTER_NAME", "Archive Test")
			t.Setenv("GIT_COMMITTER_EMAIL", "archive@example.invalid")
			run := func(dir string, args ...string) string {
				t.Helper()
				out, err := gitOut(dir, args...)
				if err != nil {
					t.Fatal(err)
				}
				return out
			}
			write := func(dir, name, body string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			remote := filepath.Join(root, "archive.git")
			vault := filepath.Join(root, "vault")
			other := filepath.Join(root, "other")
			run(root, "init", "--bare", "--initial-branch=main", remote)
			run(root, "clone", remote, vault)
			write(vault, "note.md", "initial\n")
			run(vault, "add", "note.md")
			run(vault, "commit", "-m", "initial")
			// clone establishes the upstream even for an initially empty remote.
			var out bytes.Buffer
			if err := PushVault(vault, &out); err != nil {
				t.Fatalf("initial archive push: %v", err)
			}
			if got, want := run(remote, "rev-parse", "main"), run(vault, "rev-parse", "HEAD"); got != want {
				t.Fatalf("archive HEAD = %s, want %s", got, want)
			}
			if !strings.Contains(out.String(), "push-only") {
				t.Fatalf("missing archive success message: %s", &out)
			}

			run(root, "clone", remote, other)
			write(other, "remote-only.md", "must stay in archive\n")
			run(other, "add", "remote-only.md")
			run(other, "commit", "-m", "remote-only edit")
			run(other, "push")
			if diverged {
				write(vault, "note.md", "local committed edit\n")
				run(vault, "add", "note.md")
				run(vault, "commit", "-m", "local-only edit")
			}
			write(vault, "draft.md", "uncommitted draft\n")
			localHead := run(vault, "rev-parse", "HEAD")
			remoteHead := run(remote, "rev-parse", "main")
			trackingHead := run(vault, "rev-parse", "origin/main")
			status := run(vault, "status", "--porcelain")
			out.Reset()
			err := PushVault(vault, &out)
			if err == nil {
				t.Fatal("remote changes must cause a visible archive failure")
			}
			for _, want := range []string{"сохранены локально", "отдельном клоне", "Obsidian Sync", "rejected"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("missing diagnostic %q: %v", want, err)
				}
			}
			if out.Len() != 0 {
				t.Errorf("failure reported as success: %s", &out)
			}
			for _, check := range []struct{ dir, ref, want string }{
				{vault, "HEAD", localHead},
				{remote, "main", remoteHead},
				{vault, "origin/main", trackingHead},
			} {
				if got := run(check.dir, "rev-parse", check.ref); got != check.want {
					t.Errorf("%s %s changed: %s, want %s", check.dir, check.ref, got, check.want)
				}
			}
			if got := run(vault, "status", "--porcelain"); got != status {
				t.Errorf("worktree status changed: %q, want %q", got, status)
			}
			if got, err := os.ReadFile(filepath.Join(vault, "draft.md")); err != nil || string(got) != "uncommitted draft\n" {
				t.Errorf("draft changed: %q, %v", got, err)
			}
			for _, path := range []string{"remote-only.md", ".git/FETCH_HEAD", ".git/rebase-merge", ".git/rebase-apply"} {
				if _, err := os.Stat(filepath.Join(vault, path)); !os.IsNotExist(err) {
					t.Errorf("unexpected imported file or Git operation state %s: %v", path, err)
				}
			}
		})
	}
}
