package world

import (
	"os"
	"path/filepath"
	"testing"
)

// buildFixture lays out a workspace root that mirrors the real one: two
// companies, a product without repos, a scratch directory with no marker, and
// a worktree-style checkout whose .git is a file rather than a directory.
func buildFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	mkdir := func(parts ...string) string {
		t.Helper()
		path := filepath.Join(append([]string{root}, parts...)...)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		return path
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	neurodesk := mkdir("neurodesk")
	write(filepath.Join(neurodesk, companyMarker), "slug: neurodesk\nvcs: gitlab\n# comment\nhost: example.test\n")

	agents := mkdir("neurodesk", "agents")
	write(filepath.Join(agents, productMarker), "slug: agents\nnamespace: dev/agents\n")
	synapse := mkdir("neurodesk", "agents", "synapse")
	mkdir("neurodesk", "agents", "synapse", ".git")
	_ = synapse
	// Worktree checkouts carry .git as a file.
	vox := mkdir("neurodesk", "agents", "vox")
	write(filepath.Join(vox, repoMarker), "gitdir: /elsewhere/.git/worktrees/vox\n")
	// No marker: must be ignored.
	mkdir("neurodesk", "agents", "scratch")

	empty := mkdir("neurodesk", "empty-product")
	write(filepath.Join(empty, productMarker), "slug: empty-product\n")

	// Directory inside a company without a product marker.
	mkdir("neurodesk", "notes")

	chimera := mkdir("chimera")
	write(filepath.Join(chimera, companyMarker), "slug: chimera\n")
	aetheria := mkdir("chimera", "aetheria")
	write(filepath.Join(aetheria, productMarker), "slug: aetheria\n")
	mkdir("chimera", "aetheria", "Aetheria-AI", ".git")

	// Shared worktree pool and a plain directory at root level: neither is a company.
	mkdir(".worktrees")
	mkdir("random-folder")

	return root
}

func TestScanFindsMarkedDirectoriesOnly(t *testing.T) {
	tree, err := Scan(buildFixture(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	companies := tree.Companies()
	if len(companies) != 2 {
		t.Fatalf("companies = %d, want 2 (%v)", len(companies), companies)
	}
	if companies[0].Slug != "chimera" || companies[1].Slug != "neurodesk" {
		t.Errorf("companies not sorted: %s, %s", companies[0].Slug, companies[1].Slug)
	}

	if got := len(tree.Products("")); got != 3 {
		t.Errorf("products = %d, want 3", got)
	}
	if got := len(tree.Products("neurodesk")); got != 2 {
		t.Errorf("neurodesk products = %d, want 2", got)
	}

	repos := tree.Repos("neurodesk", "agents")
	if len(repos) != 2 {
		t.Fatalf("agents repos = %d, want 2 (%v)", len(repos), repos)
	}
	if repos[0].Slug != "synapse" || repos[1].Slug != "vox" {
		t.Errorf("repos = %s, %s; want synapse, vox", repos[0].Slug, repos[1].Slug)
	}

	if got := len(tree.Repos("neurodesk", "empty-product")); got != 0 {
		t.Errorf("empty product should have no repos, got %d", got)
	}
	if got := len(tree.Repos("", "")); got != 3 {
		t.Errorf("all repos = %d, want 3", got)
	}
}

func TestScanParsesConfig(t *testing.T) {
	tree, err := Scan(buildFixture(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var neurodesk Company
	for _, company := range tree.Companies() {
		if company.Slug == "neurodesk" {
			neurodesk = company
		}
	}

	if got := neurodesk.Config.Get("vcs"); got != "gitlab" {
		t.Errorf("vcs = %q, want gitlab", got)
	}
	if got := neurodesk.Config.Get("host"); got != "example.test" {
		t.Errorf("host = %q, want example.test (comment line must be skipped)", got)
	}
	if got := neurodesk.Config.Get("missing"); got != "" {
		t.Errorf("absent key should be empty, got %q", got)
	}
	if got := neurodesk.Config.Len(); got != 3 {
		t.Errorf("config keys = %d, want 3", got)
	}
}

func TestRepoRefAndLookup(t *testing.T) {
	tree, err := Scan(buildFixture(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	matches := tree.FindRepo("synapse")
	if len(matches) != 1 {
		t.Fatalf("FindRepo(synapse) = %d matches, want 1", len(matches))
	}
	if got := matches[0].Ref(); got != "neurodesk/agents/synapse" {
		t.Errorf("Ref = %q", got)
	}

	if !tree.HasCompany("chimera") {
		t.Error("HasCompany(chimera) = false")
	}
	if tree.HasCompany("nope") {
		t.Error("HasCompany(nope) = true")
	}
	if !tree.HasProduct("neurodesk", "agents") {
		t.Error("HasProduct(neurodesk, agents) = false")
	}
	if tree.HasProduct("chimera", "agents") {
		t.Error("products must not leak across companies")
	}
}

func TestAccessorsReturnCopies(t *testing.T) {
	tree, err := Scan(buildFixture(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	companies := tree.Companies()
	companies[0].Slug = "mutated"

	if tree.Companies()[0].Slug == "mutated" {
		t.Error("Companies() leaked the internal slice")
	}
}

func TestScanRejectsEmptyRoot(t *testing.T) {
	if _, err := Scan(""); err == nil {
		t.Error("Scan(\"\") should fail")
	}
}

func TestScanReportsMissingRoot(t *testing.T) {
	if _, err := Scan(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("Scan of a missing root should fail")
	}
}

func TestRootPrefersEnv(t *testing.T) {
	t.Setenv(RootEnv, "/tmp/custom-root")

	root, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	if root != "/tmp/custom-root" {
		t.Errorf("Root = %q, want /tmp/custom-root", root)
	}
}

func TestRootFallsBackToHome(t *testing.T) {
	t.Setenv(RootEnv, "")

	root, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	if filepath.Base(root) != "Prokectfiles" {
		t.Errorf("Root = %q, want a Prokectfiles path", root)
	}
}

func TestParseLine(t *testing.T) {
	cases := []struct {
		line  string
		key   string
		value string
		ok    bool
	}{
		{"slug: neurodesk", "slug", "neurodesk", true},
		{"  namespace:  dev/agents  ", "namespace", "dev/agents", true},
		{"flag:", "flag", "", true},
		{"# comment", "", "", false},
		{"", "", "", false},
		{"no-colon", "", "", false},
		{": orphan", "", "", false},
	}

	for _, tc := range cases {
		key, value, ok := parseLine(tc.line)
		if ok != tc.ok || key != tc.key || value != tc.value {
			t.Errorf("parseLine(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.line, key, value, ok, tc.key, tc.value, tc.ok)
		}
	}
}
