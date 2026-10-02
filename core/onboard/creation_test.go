package onboard

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func localCompany(t *testing.T) string {
	t.Helper()
	root := sandbox(t)
	if err := Company(CompanyOptions{Slug: "solo", VCS: "local", Namespace: "solo"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCompanyNeverInvokesPasswordManager(t *testing.T) {
	root := sandbox(t)
	bin := t.TempDir()
	marker := filepath.Join(root, "op-called")
	t.Setenv("HQ_TEST_OP_MARKER", marker)
	if err := os.WriteFile(filepath.Join(bin, "op"), []byte("#!/bin/sh\ntouch \"$HQ_TEST_OP_MARKER\"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := Company(CompanyOptions{Slug: "local", VCS: "local", Namespace: "local"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("company creation invoked op")
	}
	for _, name := range []string{"AGENTS.md", "README.md"} {
		if got := read(t, filepath.Join(root, "local", name)); strings.Contains(got, "Work-local") || strings.Contains(got, "1Password vault") {
			t.Errorf("%s still promises automatic vault: %s", name, got)
		}
	}
}

func TestCompanyRejectsUnsafeInputsBeforeCreatingAnything(t *testing.T) {
	cases := []CompanyOptions{
		{Slug: ""}, {Slug: "../escape"}, {Slug: "a/b"}, {VCS: "custom:git.example.com"},
		{VCS: "github:"}, {VCS: "github:evil\nProxyCommand cmd"}, {VCS: "github:host/path"}, {VCS: "local:host"},
		{Namespace: "../escape"}, {Namespace: "a//b"}, {Namespace: "a\nkey: value"},
		{Email: "a@example.com\nexport PWN=1"}, {Email: "$(id)@example.com"}, {Email: "Name <a@example.com>"},
	}
	for i, bad := range cases {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			root := sandbox(t)
			opts := CompanyOptions{Slug: "safe", VCS: "local", Namespace: "safe", Email: "a@example.com", SkipExternal: true}
			if i < 3 {
				opts.Slug = bad.Slug
			}
			if bad.VCS != "" {
				opts.VCS = bad.VCS
			}
			if bad.Namespace != "" {
				opts.Namespace = bad.Namespace
			}
			if bad.Email != "" {
				opts.Email = bad.Email
			}
			if err := Company(opts, &bytes.Buffer{}); err == nil {
				t.Fatalf("accepted %+v", opts)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("invalid input left files: %v", entries)
			}
		})
	}
}

func TestCompanyExternalFailureCanBeRetried(t *testing.T) {
	root := sandbox(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ssh-keygen"), []byte("#!/bin/sh\necho simulated failure >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	opts := CompanyOptions{Slug: "acme", VCS: "github", Namespace: "acme"}
	if err := Company(opts, &bytes.Buffer{}); err == nil {
		t.Fatal("expected SSH setup failure")
	}
	if _, err := os.Lstat(filepath.Join(root, "acme")); !os.IsNotExist(err) {
		t.Fatalf("failed setup published incomplete company: %v", err)
	}
	opts.SkipExternal = true
	if err := Company(opts, &bytes.Buffer{}); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

func TestProductRerunPreservesContextAndNamespace(t *testing.T) {
	root := localCompany(t)
	opts := ProductOptions{Company: "solo", Product: "app", Namespace: "org/subgroup"}
	if err := Product(opts, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	prod := filepath.Join(root, "solo", "app")
	want := map[string]string{"AGENTS.md": "# Edited product\n", ".product-config": "slug: app\nnamespace: org/subgroup\ncustom: keep\n", ".envrc": "# custom environment\n"}
	for name, body := range want {
		write(t, filepath.Join(prod, name), body)
	}
	opts.Namespace = ""
	if err := Product(opts, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for name, body := range want {
		if got := read(t, filepath.Join(prod, name)); got != body {
			t.Errorf("overwrote %s: %q", name, got)
		}
	}
	opts.Namespace = "other"
	if err := Product(opts, &bytes.Buffer{}); err == nil {
		t.Fatal("conflicting namespace must fail")
	}
}

func TestProductRejectsAllInvalidReposBeforeWriting(t *testing.T) {
	root := localCompany(t)
	if err := Product(ProductOptions{Company: "solo", Product: "app", Repos: []string{"good", "../escape"}}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted traversal repo")
	}
	if _, err := os.Stat(filepath.Join(root, "solo", "app")); !os.IsNotExist(err) {
		t.Fatalf("invalid request partially created product: %v", err)
	}
}

func TestProductRejectsSymlinkTargets(t *testing.T) {
	for _, target := range []string{"product", "repo", "agents", "envrc", "config"} {
		t.Run(target, func(t *testing.T) {
			root := localCompany(t)
			outside := t.TempDir()
			prod := filepath.Join(root, "solo", "app")
			if target == "product" {
				if err := os.Symlink(outside, prod); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := Product(ProductOptions{Company: "solo", Product: "app"}, &bytes.Buffer{}); err != nil {
					t.Fatal(err)
				}
				name := map[string]string{"repo": "repo", "agents": "AGENTS.md", "envrc": ".envrc", "config": ".product-config"}[target]
				if target != "repo" {
					if err := os.Remove(filepath.Join(prod, name)); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(prod, name)); err != nil {
					t.Fatal(err)
				}
			}
			if err := Product(ProductOptions{Company: "solo", Product: "app", Repos: []string{"repo"}}, &bytes.Buffer{}); err == nil {
				t.Fatal("accepted symlink target")
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("wrote outside root: %v", entries)
			}
		})
	}
}

func TestProductReturnsCloneErrorsAndKeepsSuccessfulRepos(t *testing.T) {
	root := sandbox(t)
	if err := Company(CompanyOptions{Slug: "org", VCS: "github", Namespace: "org", SkipExternal: true}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HQ_TEST_GIT", git)
	bin := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = clone ]; then\ncase \"$2\" in\n*/bad.git) mkdir -p \"$3\"; echo partial > \"$3/partial\"; exit 1;;\n*) exec \"$HQ_TEST_GIT\" init -q \"$3\";;\nesac\nfi\nexec \"$HQ_TEST_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var out bytes.Buffer
	err = Product(ProductOptions{Company: "org", Product: "app", Repos: []string{"good", "bad", "also-good"}}, &out)
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("missing named clone error: %v", err)
	}
	for _, repo := range []string{"good", "also-good"} {
		if _, err := os.Stat(filepath.Join(root, "org", "app", repo, ".git")); err != nil {
			t.Errorf("lost successful repo %s: %v", repo, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "org", "app", "bad")); !os.IsNotExist(err) {
		t.Fatalf("failed clone left final repo directory: %v", err)
	}
	if strings.Contains(out.String(), "✅") {
		t.Errorf("failed product reported success: %s", out.String())
	}
}

func TestRepositoriesAddsWithoutRewritingProductContext(t *testing.T) {
	root := localCompany(t)
	if err := Product(ProductOptions{Company: "solo", Product: "app"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	prod := filepath.Join(root, "solo", "app")
	want := map[string]string{"AGENTS.md": "# Preserved\n", ".product-config": "slug: app\nnamespace: solo\ncustom: keep\n", ".envrc": "# Preserved\n"}
	for name, body := range want {
		write(t, filepath.Join(prod, name), body)
	}
	if err := Repositories(ProductOptions{Company: "solo", Product: "app", Repos: []string{"new-repo"}}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for name, body := range want {
		if got := read(t, filepath.Join(prod, name)); got != body {
			t.Errorf("overwrote %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(prod, "new-repo", ".git")); err != nil {
		t.Fatal(err)
	}
}

func TestExistingNonRepositoryIsNotAdopted(t *testing.T) {
	root := localCompany(t)
	if err := Product(ProductOptions{Company: "solo", Product: "app"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "solo", "app", "notes")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Product(ProductOptions{Company: "solo", Product: "app", Repos: []string{"notes"}}, &bytes.Buffer{}); err == nil {
		t.Fatal("adopted a non-git directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("modified non-repository directory: %v", entries)
	}
}

func TestSafeCompanySlugCanBeUsedAsSSHHostAlias(t *testing.T) {
	sandbox(t)
	if err := Company(CompanyOptions{Slug: "My_Company", VCS: "github", Namespace: "org", SkipExternal: true}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := Product(ProductOptions{Company: "My_Company", Product: "app"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("generated company config cannot be used: %v", err)
	}
}

func TestRepositoriesRejectsMissingProductWithoutCreatingIt(t *testing.T) {
	root := localCompany(t)
	if err := Repositories(ProductOptions{Company: "solo", Product: "absent", Repos: []string{"repo"}}, &bytes.Buffer{}); err == nil {
		t.Fatal("added repo to unregistered product")
	}
	if _, err := os.Stat(filepath.Join(root, "solo", "absent")); !os.IsNotExist(err) {
		t.Fatalf("created missing product: %v", err)
	}
}

func TestLocalRepoInitFailureIsReportedAndCleaned(t *testing.T) {
	root := localCompany(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\necho simulated init failure >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	err := Product(ProductOptions{Company: "solo", Product: "app", Repos: []string{"repo"}}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "git init") {
		t.Fatalf("missing init error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "solo", "app", "repo")); !os.IsNotExist(err) {
		t.Fatalf("partial repo published: %v", err)
	}
}

func TestRepositoriesRejectsSymlinkMetadataWithoutSeeding(t *testing.T) {
	root := localCompany(t)
	if err := Product(ProductOptions{Company: "solo", Product: "app", Repos: []string{"repo"}}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "solo", "app", "repo")
	for _, name := range []string{"AGENTS.md", ".envrc"} {
		if err := os.Remove(filepath.Join(repo, name)); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "external-env")
	if err := os.Symlink(outside, filepath.Join(repo, ".envrc")); err != nil {
		t.Fatal(err)
	}
	if err := Repositories(ProductOptions{Company: "solo", Product: "app", Repos: []string{"repo"}}, &bytes.Buffer{}); err == nil {
		t.Fatal("accepted symlink metadata")
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("wrote outside root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("seeded before rejecting env symlink: %v", err)
	}
}
