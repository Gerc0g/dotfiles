package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryCaptureAppendsOnlyBoundInbox(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("HQ_WIKI_ROOT", root)
	repo := filepath.Join(root, "dev/20-projects/acme/prod/repos/repo")
	os.MkdirAll(repo, 0700)
	os.WriteFile(filepath.Join(repo, "_inbox.md"), []byte("existing\n"), 0600)
	b := Binding{Preset: "work", CompanyID: "acme", RepoID: "acme/prod/repo"}
	args := json.RawMessage(`{"title":"A durable lesson","body":"Evidence and reusable rule"}`)
	if _, err := captureMemory(b, args); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(repo, "_inbox.md"))
	if !strings.HasPrefix(string(raw), "existing\n") || !strings.Contains(string(raw), "Status: candidate") {
		t.Fatal(string(raw))
	}
	b.Preset = "research"
	if _, err := captureMemory(b, args); err == nil {
		t.Fatal("research captured company memory")
	}
	b.Preset = "work"
	os.Remove(filepath.Join(repo, "_inbox.md"))
	other := filepath.Join(t.TempDir(), "other.md")
	os.WriteFile(other, []byte("private"), 0600)
	os.Symlink(other, filepath.Join(repo, "_inbox.md"))
	if _, err := captureMemory(b, args); err == nil {
		t.Fatal("inbox symlink escaped its scope")
	}
}
