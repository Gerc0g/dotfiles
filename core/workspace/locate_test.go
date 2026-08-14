package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

// makeLocateWorld builds a root with one checkout and one managed worktree.
func makeLocateWorld(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	repoSrc := filepath.Join(root, "acme", "shop", "backend", "src")
	if err := os.MkdirAll(repoSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "acme", "shop", "backend", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		filepath.Join(root, "acme", ".company-config"),
		filepath.Join(root, "acme", "shop", ".product-config"),
	} {
		if err := os.WriteFile(marker, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	wt := filepath.Join(root, ".worktrees", "acme", "shop", "backend", "ab12cd34")
	if err := os.MkdirAll(filepath.Join(wt, "deep", "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := "id=ab12cd34\ncompany=acme\nproduct=shop\nrepo=backend\ntask=fix-login\n" +
		"branch=agent/fix-login-ab12cd34\ncleanup_state=active\n"
	if err := os.WriteFile(filepath.Join(wt, MetaFile), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestLocateRepoFromDepth(t *testing.T) {
	root := makeLocateWorld(t)

	ctx, err := Locate(root, filepath.Join(root, "acme", "shop", "backend", "src"))
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Kind != "repo" || ctx.Ref() != "acme/shop/backend" {
		t.Errorf("repo ctx: kind=%s ref=%s", ctx.Kind, ctx.Ref())
	}
	// Locate resolves symlinks (macOS /var → /private/var), so compare
	// against the resolved root.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Path != filepath.Join(resolvedRoot, "acme", "shop", "backend") {
		t.Errorf("repo path: %s", ctx.Path)
	}
}

func TestLocateProductAndCompany(t *testing.T) {
	root := makeLocateWorld(t)

	ctx, err := Locate(root, filepath.Join(root, "acme", "shop"))
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Kind != "product" || ctx.Ref() != "acme/shop" {
		t.Errorf("product ctx: kind=%s ref=%s", ctx.Kind, ctx.Ref())
	}

	ctx, err = Locate(root, filepath.Join(root, "acme"))
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Kind != "company" || ctx.Ref() != "acme" {
		t.Errorf("company ctx: kind=%s ref=%s", ctx.Kind, ctx.Ref())
	}
}

func TestLocateWorktreeFromDepth(t *testing.T) {
	root := makeLocateWorld(t)

	dir := filepath.Join(root, ".worktrees", "acme", "shop", "backend", "ab12cd34", "deep", "inside")
	ctx, err := Locate(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Kind != "worktree" {
		t.Fatalf("kind: %s", ctx.Kind)
	}
	if ctx.Ref() != "acme/shop/backend/ab12cd34" {
		t.Errorf("ref: %s", ctx.Ref())
	}
	if ctx.Task != "fix-login" || ctx.State != "active" {
		t.Errorf("meta: task=%s state=%s", ctx.Task, ctx.State)
	}
}

func TestLocateOutsideRoot(t *testing.T) {
	root := makeLocateWorld(t)

	if _, err := Locate(root, t.TempDir()); err == nil {
		t.Error("directories outside the root must not resolve")
	}
	if _, err := Locate(root, root); err == nil {
		t.Error("the root itself is not a context")
	}
}

func TestLocateUnmarkedDirs(t *testing.T) {
	root := makeLocateWorld(t)

	scratch := filepath.Join(root, "scratch")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Locate(root, scratch); err == nil {
		t.Error("a directory without .company-config must not resolve as company")
	}

	// A plain directory inside a product that is not a git repo resolves to
	// the product, not to a phantom repo.
	notRepo := filepath.Join(root, "acme", "shop", "notes")
	if err := os.MkdirAll(notRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, err := Locate(root, notRepo)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Kind != "product" {
		t.Errorf("want product fallback, got %s", ctx.Kind)
	}
}
