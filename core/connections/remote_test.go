package connections

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body string, status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}
func connected(t *testing.T) Config {
	t.Helper()
	c := setup(t)
	v, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "revision": 0, "config": c})
	if err != nil {
		t.Fatal(err)
	}
	c = v.(Config)
	v, err = call(t, "connections.credential.put", map[string]any{"companyId": "alpha", "revision": c.Revision, "connectionId": "main", "token": "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	return v.(Config)
}
func TestVerificationAndPRBodyUseReadOnlyChecksAndLiteralJSON(t *testing.T) {
	c := connected(t)
	prior := httpTransport
	t.Cleanup(func() { httpTransport = prior })
	writes := 0
	body := "Line one\n$(touch /tmp/never) `echo never` \\\"quote\\\""
	httpTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Fatal("missing authorization")
		}
		if r.Method != "GET" {
			writes++
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["body"] != body || payload["draft"] != true || payload["base"] != "main" {
				t.Fatalf("wrong PR body: %v", payload)
			}
			return response(`{"number":7,"html_url":"https://github.com/team/repo/pull/7"}`, 201), nil
		}
		if strings.Contains(r.URL.Path, "/rules/") {
			return response(`[]`, 200), nil
		}
		if strings.Contains(r.URL.Path, "/branches/") {
			return response(`{"protected":false}`, 200), nil
		}
		return response(`{"default_branch":"main"}`, 200), nil
	})
	v, err := call(t, "connections.verify", map[string]any{"companyId": "alpha", "revision": c.Revision, "connectionId": "main"})
	if err != nil {
		t.Fatal(err)
	}
	c = v.(Config)
	if writes != 0 || c.Git[0].Verification.Read != "verified" || c.Git[0].Verification.BranchPush != "unverified" || c.Git[0].Verification.PullRequests != "unverified" {
		t.Fatalf("false verification: %+v", c.Git[0].Verification)
	}
	args, _ := json.Marshal(pullArgs{Branch: "work/change", Title: "Literal", Body: body})
	if _, err = Execute(context.Background(), Binding{"alpha", "alpha/product/repo", "/never/open"}, "git.pr.create", args); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatal("PR was not submitted")
	}
}
func TestRemoteDefaultAndProtectedBranchesCannotBePublished(t *testing.T) {
	connected(t)
	prior := httpTransport
	t.Cleanup(func() { httpTransport = prior })
	for _, branch := range []string{"work/default", "work/protected"} {
		t.Run(branch, func(t *testing.T) {
			httpTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" {
					t.Fatal("write reached hosting")
				}
				if strings.Contains(r.URL.Path, "/branches/") {
					return response(`{"protected":true}`, 200), nil
				}
				return response(`{"default_branch":"work/default"}`, 200), nil
			})
			args, _ := json.Marshal(branchArgs{Branch: branch, BundleBase64: "not-a-bundle"})
			if _, err := Execute(context.Background(), Binding{"alpha", "alpha/product/repo", ""}, "git.push", args); err == nil {
				t.Fatal("unsafe branch accepted")
			}
		})
	}
}
func TestEnvironmentAllowsOnlyNamedConstrainedRequests(t *testing.T) {
	c := setup(t)
	c.Environments = []Environment{{ID: "staging", Tier: "stage", Kind: "http", BaseURL: "https://application.example/api", Requests: []Endpoint{{ID: "health", Method: "GET", Path: "/health", Query: map[string]string{"limit": "[1-9][0-9]?"}, Verify: true}}}}
	v, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "config": c})
	if err != nil {
		t.Fatal(err)
	}
	c = v.(Config)
	_, err = call(t, "connections.credential.put", map[string]any{"companyId": "alpha", "revision": c.Revision, "connectionId": "staging", "token": "stage-secret"})
	if err != nil {
		t.Fatal(err)
	}
	prior := httpTransport
	t.Cleanup(func() { httpTransport = prior })
	requests := 0
	httpTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Method != "GET" || r.URL.String() != "https://application.example/api/health?limit=10" {
			t.Fatal("allowlist bypass")
		}
		return response(`{"ok":true,"echo":"stage-secret"}`, 200), nil
	})
	b := Binding{"alpha", "alpha/product/repo", ""}
	for _, args := range []string{`{"environmentId":"staging","requestId":"shell","query":{}}`, `{"environmentId":"staging","requestId":"health","query":{"limit":"10000"}}`, `{"environmentId":"staging","requestId":"health","query":{"sql":"delete from users"}}`, `{"environmentId":"staging","requestId":"health","method":"POST"}`} {
		if _, err = Execute(context.Background(), b, "environment.request", json.RawMessage(args)); err == nil {
			t.Fatal("unsafe environment request accepted")
		}
	}
	if requests != 0 {
		t.Fatal("denied request touched network")
	}
	result, err := Execute(context.Background(), b, "environment.request", json.RawMessage(`{"environmentId":"staging","requestId":"health","query":{"limit":"10"}}`))
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(result)
	if strings.Contains(string(wire), "stage-secret") || !strings.Contains(string(wire), "unverified") {
		t.Fatal("secret or false credential guarantee returned")
	}
}
func TestConfigurationCannotRelocateCredentialToDifferentHost(t *testing.T) {
	c := connected(t)
	c.Git[0].Host = "https://attacker.example"
	if _, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "revision": c.Revision, "config": c}); err == nil {
		t.Fatal("existing credential silently relocated to a new host")
	}
}
func TestStoreRejectsCredentialDirectorySymlink(t *testing.T) {
	c := connected(t)
	dir := filepath.Join(os.Getenv("HQ_DATA_ROOT"), "connections", "alpha", "credentials")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := os.Symlink(other, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, "connections.credential.put", map[string]any{"companyId": "alpha", "revision": c.Revision, "connectionId": "main", "token": "another-secret"}); err == nil {
		t.Fatal("credential directory symlink accepted")
	}
}

func TestPRUpdateRejectsCrossRepositoryHeadBeforeWriting(t *testing.T) {
	connected(t)
	prior := httpTransport
	t.Cleanup(func() { httpTransport = prior })
	httpTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			t.Fatal("cross-repository PR update reached hosting")
		}
		if strings.Contains(r.URL.Path, "/pulls/") {
			return response(`{"head":{"ref":"work/change","repo":{"full_name":"foreign/secret"}},"base":{"ref":"main"}}`, 200), nil
		}
		return response(`{"default_branch":"main"}`, 200), nil
	})
	_, err := Execute(context.Background(), Binding{"alpha", "alpha/product/repo", ""}, "git.pr.update", json.RawMessage(`{"number":7,"title":"Updated","body":"literal"}`))
	if err == nil {
		t.Fatal("cross-repository PR accepted")
	}
}
func TestNetworkRedirectNeverReceivesCredentials(t *testing.T) {
	prior := httpTransport
	t.Cleanup(func() { httpTransport = prior })
	calls := 0
	httpTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		res := response("", 302)
		res.Header.Set("Location", "https://other.example/collect")
		return res, nil
	})
	if _, _, err := request(context.Background(), "GET", "https://host.example/repo", "private-token", nil); err == nil {
		t.Fatal("redirect accepted")
	}
	if calls != 1 {
		t.Fatal("credential-bearing redirect followed")
	}
}

func TestVerificationDoesNotConfuseMetadataWithCodeAccess(t *testing.T) {
	c := connected(t)
	prior := httpTransport
	t.Cleanup(func() { httpTransport = prior })
	httpTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/git/trees/") {
			return response(`{}`, 403), nil
		}
		return response(`{"default_branch":"main"}`, 200), nil
	})
	result, err := call(t, "connections.verify", map[string]any{"companyId": "alpha", "revision": c.Revision, "connectionId": "main"})
	if err != nil {
		t.Fatal(err)
	}
	if result.(Config).Git[0].Verification.Read == "verified" {
		t.Fatal("metadata-only credential falsely verified for reading code")
	}
}

func TestGitLabChecksAndLiteralMergeRequest(t *testing.T) {
	c := setup(t)
	c.Git[0].Provider = "gitlab"
	c.Git[0].Host = "https://gitlab.example"
	c.Git[0].Policy.RequiredChecks = []string{"tests"}
	v, err := call(t, "connections.save", map[string]any{"companyId": "alpha", "config": c})
	if err != nil {
		t.Fatal(err)
	}
	c = v.(Config)
	_, err = call(t, "connections.credential.put", map[string]any{"companyId": "alpha", "revision": c.Revision, "connectionId": "main", "token": "gitlab-token"})
	if err != nil {
		t.Fatal(err)
	}
	prior := httpTransport
	t.Cleanup(func() { httpTransport = prior })
	sha := strings.Repeat("a", 40)
	passed := false
	writes := 0
	httpTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(r.URL.Path, "/protected_branches"):
			return response(`[]`, 200), nil
		case strings.Contains(r.URL.Path, "/repository/branches/"):
			return response(`{"protected":false,"commit":{"id":"`+sha+`"}}`, 200), nil
		case strings.Contains(r.URL.Path, "/statuses"):
			state := "failed"
			if passed {
				state = "success"
			}
			return response(`[{"name":"tests","status":"`+state+`","ref":"work/test","sha":"`+sha+`"}]`, 200), nil
		case r.Method == "POST":
			writes++
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["description"] != "$(touch /never)\ntext" || payload["title"] != "Draft: Example" || payload["target_branch"] != "main" {
				t.Fatal("MR content was not literal or policy was lost")
			}
			return response(`{"iid":8,"web_url":"https://gitlab.example/team/repo/-/merge_requests/8"}`, 201), nil
		default:
			return response(`{"default_branch":"main"}`, 200), nil
		}
	})
	b := Binding{"alpha", "alpha/product/repo", ""}
	args := json.RawMessage(`{"branch":"work/test","title":"Example","body":"$(touch /never)\ntext"}`)
	if _, err = Execute(context.Background(), b, "git.pr.create", args); err == nil || writes != 0 {
		t.Fatal("failed checks allowed MR creation")
	}
	passed = true
	if _, err = Execute(context.Background(), b, "git.pr.create", args); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatal("successful checks did not permit MR")
	}
}

func TestProdConnectorCannotForwardUnverifiedCredential(t *testing.T) {
	prior := httpTransport
	t.Cleanup(func() { httpTransport = prior })
	requests := 0
	httpTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { requests++; return response(`{"ok":true}`, 200), nil })
	for _, permission := range []string{"unverified", "verified"} {
		e := Environment{Tier: "prod", Kind: "http", BaseURL: "https://production.example", Verification: Verification{State: "read_verified", CredentialPermissions: permission}, Requests: []Endpoint{{ID: "health", Method: "GET", Path: "/health"}}}
		if _, err := environmentRequest(context.Background(), e, "health", nil, "possibly-admin-token"); err == nil {
			t.Errorf("generic Prod request accepted with claimed %s permissions", permission)
		}
	}
	if requests != 0 {
		t.Fatal("Prod credential was sent without provider-backed readonly proof")
	}
}
