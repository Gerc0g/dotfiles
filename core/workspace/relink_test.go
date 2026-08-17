package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// Most of the pool was made by hand, without workspace metadata. Those
// worktrees are where sessions actually run, so the repair must see them.
func TestMemoryGapsCoversUnmanagedWorktrees(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault", "repo-memory")
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(root, "co", "prod", "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(vault, filepath.Join(repoDir, "docs", "knowledge")); err != nil {
		t.Fatal(err)
	}

	// A worktree with no .agent-workspace file at all.
	wtDir := filepath.Join(root, ".worktrees", "co", "prod", "repo", "task")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	gaps, err := m.MemoryGaps()
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 1 {
		t.Fatalf("gaps = %d, want 1", len(gaps))
	}

	if _, err := m.RelinkMemory(); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(wtDir, "docs", "knowledge"))
	if err != nil {
		t.Fatalf("worktree still has no memory: %v", err)
	}
	if target != vault {
		t.Errorf("link → %s, want %s", target, vault)
	}

	gaps, _ = m.MemoryGaps()
	if len(gaps) != 0 {
		t.Errorf("still %d gaps after repair", len(gaps))
	}
}
