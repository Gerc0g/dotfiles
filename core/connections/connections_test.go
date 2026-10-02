package connections

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("PROKECTFILES_ROOT", root)
	t.Setenv("HQ_DATA_ROOT", t.TempDir())
	for _, co := range []string{"alpha", "beta"} {
		for _, p := range []string{filepath.Join(co, ".company-config"), filepath.Join(co, "product", ".product-config"), filepath.Join(co, "product", "repo", ".git")} {
			path := filepath.Join(root, p)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(""), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return Config{CompanyID: "alpha", Git: []GitConnection{{ID: "main", Provider: "github", Host: "https://github.com", Namespace: "team", Policy: Policy{Read: true, BranchPush: true, PullRequests: true, BranchPrefixes: []string{"work/"}, ProtectedBranches: []string{"main"}, TargetBranch: "main", Draft: true}}}, Repositories: []Repository{{RepoID: "alpha/product/repo", ConnectionID: "main", RemotePath: "team/repo"}}, Environments: []Environment{}}
}
func call(t *testing.T, op string, args any) (any, error) {
	t.Helper()
	b, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return Dispatch(op, b)
}
func TestRevisionSecretAndCompanyBoundary(t *testing.T) {
	c := setup(t)
	got, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "revision": 0, "config": c})
	if err != nil {
		t.Fatal(err)
	}
	cfg := got.(Config)
	if cfg.Revision != 1 {
		t.Fatalf("revision: %d", cfg.Revision)
	}
	if _, err = call(t, "connections.save", map[string]any{"companyId": "alpha", "revision": 0, "config": c}); err == nil {
		t.Fatal("stale revision accepted")
	}
	credential, err := call(t, "connections.credential.put", map[string]any{"companyId": "alpha", "revision": 1, "connectionId": "main", "token": "sentinel-secret"})
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(credential)
	if strings.Contains(string(wire), "sentinel-secret") {
		t.Fatal("secret returned")
	}
	cfg = credential.(Config)
	path := filepath.Join(os.Getenv("HQ_DATA_ROOT"), "connections", "alpha", "credentials", cfg.Git[0].CredentialRef+".json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("credential is not private")
	}
	cfg.Repositories[0].RepoID = "beta/product/repo"
	if _, err = call(t, "connections.save", map[string]any{"companyId": "alpha", "revision": cfg.Revision, "config": cfg}); err == nil {
		t.Fatal("external company repository accepted")
	}
}
func TestBoundPolicyAndStrictArguments(t *testing.T) {
	c := setup(t)
	c.Git[0].Policy.BranchPush = false
	c.Git[0].Policy.PullRequests = false
	if _, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "revision": 0, "config": c}); err != nil {
		t.Fatal(err)
	}
	b := Binding{CompanyID: "alpha", RepoID: "alpha/product/repo"}
	for _, tc := range []struct{ op, args string }{{"git.push", `{"branch":"work/test","bundleBase64":""}`}, {"git.pr.create", `{"branch":"work/test","title":"test","body":"$(touch /tmp/no)"}`}, {"git.read", `{"companyId":"beta"}`}, {"git.merge", `{}`}} {
		if _, err := Execute(context.Background(), b, tc.op, json.RawMessage(tc.args)); err == nil {
			t.Fatalf("accepted %s", tc.op)
		}
	}
	b.RepoID = "beta/product/repo"
	if _, err := Execute(context.Background(), b, "git.read", json.RawMessage(`{}`)); err == nil {
		t.Fatal("foreign binding accepted")
	}
}
func TestEnvironmentFailsClosed(t *testing.T) {
	c := setup(t)
	c.Environments = []Environment{{ID: "production", Tier: "prod", Kind: "http", BaseURL: "https://service.example", Requests: []Endpoint{{ID: "health", Method: "POST", Path: "/health"}}}}
	if _, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "revision": 0, "config": c}); err == nil {
		t.Fatal("production write accepted")
	}
	c.Environments[0].Requests[0].Method = "GET"
	c.Environments[0].Requests[0].Path = "//other.example/steal"
	if _, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "revision": 0, "config": c}); err == nil {
		t.Fatal("absolute-path host override accepted")
	}
}

func TestRemovingConnectionRemovesItsPrivateCredential(t *testing.T) {
	c := connected(t)
	ref := c.Git[0].CredentialRef
	c.Git = []GitConnection{}
	c.Repositories = []Repository{}
	if _, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "revision": c.Revision, "config": c}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HQ_DATA_ROOT"), "connections", "alpha", "credentials", ref+".json")); !os.IsNotExist(err) {
		t.Fatal("removed connection left its credential behind")
	}
}
