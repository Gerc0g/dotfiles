package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Gerc0g/dotfiles/core/world"
)

// makeWorld lays out one company/product/repo triple on disk.
func makeWorld(t *testing.T) *world.Tree {
	t.Helper()
	root := t.TempDir()

	repo := filepath.Join(root, "acme", "shop", "backend")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
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

	tree, err := world.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func readFile(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFromTreeEntries(t *testing.T) {
	tree := makeWorld(t)
	entries := FromTree(tree, true, true)

	if len(entries) != 2 {
		t.Fatalf("want product + repo, got %d entries", len(entries))
	}
	if entries[0].Name != "Product: acme/shop" {
		t.Errorf("product name: %q", entries[0].Name)
	}
	if entries[1].Name != "backend" {
		t.Errorf("repo name: %q", entries[1].Name)
	}
	wantTags := []string{ManagedTag, "repo", "company:acme", "product:shop", "repo:backend"}
	for i, tag := range wantTags {
		if entries[1].Tags[i] != tag {
			t.Errorf("repo tag %d: want %q, got %q", i, tag, entries[1].Tags[i])
		}
	}
}

func TestFromWorktreesNaming(t *testing.T) {
	entries := FromWorktrees([]Worktree{{
		Path: "/pool/acme/shop/backend/abc123", ID: "abc123",
		Company: "acme", Product: "shop", Repo: "backend",
		Task: "fix-login", Branch: "agent/fix-login-abc123", State: "active",
	}})

	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "backend @ abc123 · fix-login" {
		t.Errorf("name: %q", entries[0].Name)
	}
	last := entries[0].Tags[len(entries[0].Tags)-1]
	if last != "branch:agent/fix-login-abc123" {
		t.Errorf("branch tag: %q", last)
	}
}

func TestSyncFreshFile(t *testing.T) {
	tree := makeWorld(t)
	path := filepath.Join(t.TempDir(), "projects.json")

	res, err := Sync(path, FromTree(tree, true, true), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || res.Stats.Added != 2 {
		t.Fatalf("want 2 added, got %+v", res.Stats)
	}

	projects := readFile(t, path)
	if len(projects) != 2 {
		t.Fatalf("want 2 projects on disk, got %d", len(projects))
	}

	// Second run must be a no-op.
	res, err = Sync(path, FromTree(tree, true, true), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Error("second sync must not report changes")
	}
}

func TestSyncKeepsUserFieldsAndOrder(t *testing.T) {
	tree := makeWorld(t)
	repoPath := tree.Repos("", "")[0].Path
	path := filepath.Join(t.TempDir(), "projects.json")

	seed := []map[string]any{{
		"name":     "old-name",
		"rootPath": repoPath,
		"paths":    []any{"extra"},
		"tags":     []any{ManagedTag, "repo"},
		"enabled":  false,
		"profile":  "work",
	}}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Sync(path, FromTree(tree, false, true), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.Updated != 1 {
		t.Fatalf("want 1 updated, got %+v", res.Stats)
	}

	got := readFile(t, path)[0]
	if got["name"] != "backend" {
		t.Errorf("name must be regenerated: %v", got["name"])
	}
	if got["enabled"] != false || got["profile"] != "work" {
		t.Errorf("user fields must survive: enabled=%v profile=%v", got["enabled"], got["profile"])
	}
}

func TestSyncPrunesAndPreserves(t *testing.T) {
	tree := makeWorld(t)
	path := filepath.Join(t.TempDir(), "projects.json")

	seed := []map[string]any{
		{"name": "stale", "rootPath": "/gone", "tags": []any{ManagedTag}},
		{"name": "mine", "rootPath": "/personal", "tags": []any{}},
	}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Sync(path, FromTree(tree, false, true), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.Pruned != 1 || res.Stats.ExternalPruned != 1 {
		t.Fatalf("want prune=1 external_prune=1, got %+v", res.Stats)
	}

	opts := DefaultOptions()
	opts.PreserveExternal = true
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = Sync(path, FromTree(tree, false, true), opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.Preserved != 1 || res.Stats.ExternalPruned != 0 {
		t.Fatalf("preserve-external: got %+v", res.Stats)
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	tree := makeWorld(t)
	path := filepath.Join(t.TempDir(), "projects.json")

	opts := DefaultOptions()
	opts.DryRun = true
	res, err := Sync(path, FromTree(tree, true, true), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Error("dry-run must still report the pending change")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("dry-run must not create the file")
	}
}
