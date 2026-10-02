package runner

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRejectsUnregisteredAndSymlinkPaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PROKECTFILES_ROOT", root)
	for _, p := range []string{"acme/.company-config", "acme/prod/.product-config", "acme/prod/repo/.git/HEAD"} {
		path := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(path), 0700)
		os.WriteFile(path, []byte("ref: refs/heads/main\n"), 0600)
	}
	got, err := Resolve(filepath.Join(root, "acme/prod/repo"), "work")
	if err != nil || got.CompanyID != "acme" {
		t.Fatalf("resolve: %+v %v", got, err)
	}
	for _, p := range []string{root, filepath.Join(root, "unknown"), t.TempDir()} {
		if _, err := Resolve(p, "work"); err == nil {
			t.Fatalf("accepted unregistered path %s", p)
		}
	}
	os.Symlink(t.TempDir(), filepath.Join(root, "acme/prod/repo/escape"))
	if _, err := Resolve(filepath.Join(root, "acme/prod/repo/escape"), "work"); err == nil {
		t.Fatal("accepted symlink escape")
	}
	if _, err := Resolve(filepath.Join(root, "acme/prod/repo"), "research"); err == nil {
		t.Fatal("research accepted company repo")
	}
}

func TestProxyDeniesPrivateRebindingAndDisallowedHost(t *testing.T) {
	for _, tc := range []struct {
		host     string
		ips      []net.IP
		internet bool
	}{
		{"api.openai.com:443", []net.IP{net.ParseIP("127.0.0.1")}, false},
		{"api.openai.com:443", []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("10.0.0.1")}, false},
		{"company.example:443", []net.IP{net.ParseIP("8.8.8.8")}, false},
		{"169.254.169.254:443", []net.IP{net.ParseIP("169.254.169.254")}, true},
		{"example.com:22", []net.IP{net.ParseIP("8.8.8.8")}, true},
	} {
		p := Proxy{Internet: tc.internet, Lookup: func(context.Context, string) ([]net.IP, error) { return tc.ips, nil }}
		r := httptest.NewRequest(http.MethodConnect, "http://"+tc.host, nil)
		r.Host = tc.host
		w := httptest.NewRecorder()
		p.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: status %d", tc.host, w.Code)
		}
	}
}

func TestAdmissionRefusesActiveSameWorkspaceAndReleases(t *testing.T) {
	t.Setenv("HQ_DATA_ROOT", t.TempDir())
	b := Binding{CompanyID: "acme", RepoID: "acme/prod/repo", WorktreePath: t.TempDir(), Preset: "work"}
	a, err := acquire(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquire(b); err == nil {
		t.Fatal("concurrent same-workspace admission succeeded")
	}
	a.release("completed", "")
	next, err := acquire(b)
	if err != nil {
		t.Fatal(err)
	}
	next.release("completed", "")
}

func TestProxyUsesValidatedPublicAddressAndBlocksControlHost(t *testing.T) {
	p := Proxy{Lookup: func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("8.8.8.8")}, nil }}
	target, err := p.target(context.Background(), "api.openai.com:443")
	if err != nil || target != "8.8.8.8:443" {
		t.Fatalf("target=%s err=%v", target, err)
	}
	p.BlockedIPs = []net.IP{net.ParseIP("8.8.8.8")}
	if _, err = p.target(context.Background(), "api.openai.com:443"); err == nil {
		t.Fatal("control host allowed")
	}
}

func TestResearchDirectoryDeterminesItsPreset(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("HQ_WIKI_ROOT", root)
	path := filepath.Join(root, "research")
	os.MkdirAll(path, 0700)
	b, err := Resolve(path, "work")
	if err != nil || b.Preset != "research" || b.CompanyID != "" {
		t.Fatalf("%+v %v", b, err)
	}
}
