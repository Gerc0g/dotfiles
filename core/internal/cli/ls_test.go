package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Gerc0g/dotfiles/core/knowledge"
	"github.com/Gerc0g/dotfiles/core/runner"
	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/Gerc0g/dotfiles/core/world"
)

func TestLsJSONCatalog(t *testing.T) {
	root := lsFixture(t)
	repo := filepath.Join(root, "acme", "product", "repo")
	// An unborn repository can already have a managed orphan worktree. The
	// catalog must include both it and a second unborn repo with no worktrees.
	m, err := workspace.New(root, workspace.WithProjectSync(func() {}))
	if err != nil {
		t.Fatal(err)
	}
	w, err := m.Start("acme", "product", "repo", "Fix timeout")
	if err != nil {
		t.Fatal(err)
	}

	out, stderr, err := executeLs("--json")
	if err != nil {
		t.Fatalf("ls --json: %v", err)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	var got any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not one JSON document: %v: %s", err, out)
	}
	// Decode an independent wire expectation, so field names and empty arrays
	// are tested without depending on the implementation's DTO types.
	wantJSON := fmt.Sprintf(`{
		"version":1,"root":%q,"companies":[
			{"slug":"acme","path":%q,"products":[
				{"slug":"empty-product","path":%q,"repos":[]},
				{"slug":"product","path":%q,"repos":[
					{"name":"empty-repo","path":%q,"worktrees":[]},
					{"name":"repo","path":%q,"worktrees":[
						{"id":%q,"path":%q,"branch":%q,"state":"active","task":"fix-timeout"}
					]}
				]}
			]},
			{"slug":"empty-company","path":%q,"products":[]}
		]
	}`, root, filepath.Join(root, "acme"), filepath.Join(root, "acme", "empty-product"),
		filepath.Join(root, "acme", "product"), filepath.Join(root, "acme", "product", "empty-repo"),
		repo, w.ID, w.Path, w.Branch, filepath.Join(root, "empty-company"))
	var want any
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("catalog = %s\nwant %s", out, wantJSON)
	}
}

func TestLsJSONFilters(t *testing.T) {
	lsFixture(t)
	for _, tc := range []struct {
		name     string
		args     []string
		company  string
		products int
		product  string
	}{
		{"company", []string{"--json", "acme"}, "acme", 2, ""},
		{"product", []string{"acme", "product", "--json"}, "acme", 1, "product"},
		{"empty product", []string{"--json", "acme", "empty-product"}, "acme", 1, "empty-product"},
		{"empty company", []string{"--json", "empty-company"}, "empty-company", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := executeLs(tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Companies []struct {
					Slug     string
					Products []struct{ Slug string }
				}
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Companies) != 1 || got.Companies[0].Slug != tc.company || len(got.Companies[0].Products) != tc.products {
				t.Fatalf("unexpected selection: %s", out)
			}
			if tc.product != "" && got.Companies[0].Products[0].Slug != tc.product {
				t.Fatalf("wrong product: %s", out)
			}
		})
	}
}

func TestLsJSONEmptyRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv(world.RootEnv, root)
	t.Setenv("HQ_WIKI_ROOT", t.TempDir())
	out, _, err := executeLs("--json")
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("{\"version\":1,\"root\":%q,\"companies\":[]}\n", root)
	if out != want {
		t.Errorf("empty catalog = %q, want %q", out, want)
	}
}

func TestLsJSONResearchDirectoryMatchesContextAndRunner(t *testing.T) {
	lsFixture(t)
	vault := t.TempDir()
	lsWrite(t, filepath.Join(vault, "research", "index.md"), "# Research")
	alias := filepath.Join(t.TempDir(), "wiki")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HQ_WIKI_ROOT", alias)
	out, _, err := executeLs("--json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Version  int `json:"version"`
		Research *struct {
			Path string `json:"path"`
		} `json:"research"`
	}
	if err := json.Unmarshal([]byte(out), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Version != 1 || catalog.Research == nil {
		t.Fatalf("research missing from catalog: %s", out)
	}
	result, err := knowledge.Dispatch("knowledge.context", json.RawMessage(`{"scope":{"kind":"research"}}`))
	if err != nil {
		t.Fatal(err)
	}
	directory := result.(map[string]any)["directory"].(string)
	if catalog.Research.Path != directory {
		t.Fatalf("catalog %q disagrees with launch context %q", catalog.Research.Path, directory)
	}
	binding, err := runner.Resolve(catalog.Research.Path, "work")
	if err != nil || binding.Preset != "research" || binding.CompanyID != "" {
		t.Fatalf("catalog research does not resolve to isolated runner: %+v, %v", binding, err)
	}
}

func TestLsJSONOmitsUnavailableOrAliasedResearch(t *testing.T) {
	for _, mode := range []string{"absent", "file", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			lsFixture(t)
			vault := t.TempDir()
			t.Setenv("HQ_WIKI_ROOT", vault)
			switch mode {
			case "file":
				lsWrite(t, filepath.Join(vault, "research"), "not a directory")
			case "symlink":
				if err := os.Symlink(t.TempDir(), filepath.Join(vault, "research")); err != nil {
					t.Fatal(err)
				}
			}
			out, _, err := executeLs("--json")
			if err != nil {
				t.Fatal(err)
			}
			var catalog map[string]any
			if err := json.Unmarshal([]byte(out), &catalog); err != nil {
				t.Fatal(err)
			}
			if _, exists := catalog["research"]; exists {
				t.Fatalf("catalog exposed unavailable research: %s", out)
			}
		})
	}
}

func TestLsJSONRejectsConflictingModesAndUnknownFilters(t *testing.T) {
	lsFixture(t)
	for _, args := range [][]string{
		{"--json", "--flat"}, {"--json", "--paths"},
		{"--json", "missing"}, {"--json", "acme", "missing"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, _, err := executeLs(args...)
			if err == nil || out != "" {
				t.Fatalf("expected error without stdout, got %q, %v", out, err)
			}
		})
	}
}

func TestLsJSONWarningsStayOnStderr(t *testing.T) {
	root := lsFixture(t)
	// The scanner's line limit provides a portable malformed-marker warning.
	lsWrite(t, filepath.Join(root, "broken", ".company-config"), strings.Repeat("x", 100000))
	out, stderr, err := executeLs("--json")
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(out)) || !strings.Contains(stderr, "предупреждение:") {
		t.Fatalf("stdout = %q, stderr = %q", out, stderr)
	}
}

func TestLsPreservesTextModes(t *testing.T) {
	root := lsFixture(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--flat"}, "acme/product/empty-repo\nacme/product/repo\n"},
		{[]string{"--paths"}, filepath.Join(root, "acme", "product", "empty-repo") + "\n" + filepath.Join(root, "acme", "product", "repo") + "\n"},
		{[]string{"--flat", "--paths"}, filepath.Join(root, "acme", "product", "empty-repo") + "\n" + filepath.Join(root, "acme", "product", "repo") + "\n"},
	} {
		out, _, err := executeLs(tc.args...)
		if err != nil || out != tc.want {
			t.Errorf("ls %v = %q, %v; want %q", tc.args, out, err, tc.want)
		}
	}
	out, _, err := executeLs("acme", "product")
	if err != nil || !strings.Contains(out, "empty-repo") || !strings.Contains(out, "2 репозитория") {
		t.Errorf("tree output = %q, %v", out, err)
	}
}

func executeLs(args ...string) (string, string, error) {
	cmd := NewRoot()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append([]string{"ls"}, args...))
	err := cmd.Execute()
	return out.String(), stderr.String(), err
}

func lsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv(world.RootEnv, root)
	t.Setenv("HQ_WIKI_ROOT", t.TempDir())
	t.Setenv("HQ_DATA_ROOT", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	lsWrite(t, filepath.Join(root, "acme", ".company-config"), "git_email: dev@acme.test\n")
	lsWrite(t, filepath.Join(root, "acme", "product", ".product-config"), "slug: product\n")
	lsWrite(t, filepath.Join(root, "acme", "empty-product", ".product-config"), "")
	lsWrite(t, filepath.Join(root, "empty-company", ".company-config"), "")
	lsWrite(t, filepath.Join(root, "scratch", "ignored"), "")
	for _, name := range []string{"empty-repo", "repo"} {
		repo := filepath.Join(root, "acme", "product", name)
		if err := os.MkdirAll(repo, 0o755); err != nil {
			t.Fatal(err)
		}
		lsGit(t, repo, "init", "-q", "-b", "main")
		lsGit(t, repo, "config", "user.name", "test")
		lsGit(t, repo, "config", "user.email", "test@acme.test")
	}
	return root
}

func lsWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lsGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func TestLsCatalogIncludesOpaqueWorktreeCreationIdentity(t *testing.T) {
	root := lsFixture(t)
	m, err := workspace.New(root, workspace.WithProjectSync(func() {}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.StartWithRequestID("acme", "product", "repo", "Fix bug!", "creation-123"); err != nil {
		t.Fatal(err)
	}
	out, _, err := executeLs("--json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog map[string]any
	if err := json.Unmarshal([]byte(out), &catalog); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"creationRequestId":"creation-123"`) && !strings.Contains(out, `"creationRequestId": "creation-123"`) {
		t.Fatalf("request identity absent from catalog: %s", out)
	}
}
