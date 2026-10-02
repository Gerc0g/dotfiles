package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentBrokerPublishesOnlyBoundArchitectureWithCAS(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("PROKECTFILES_ROOT", root)
	os.MkdirAll(filepath.Join(root, "acme/prod"), 0700)
	os.WriteFile(filepath.Join(root, "acme/.company-config"), nil, 0600)
	os.WriteFile(filepath.Join(root, "acme/prod/.product-config"), nil, 0600)
	b, err := Resolve(filepath.Join(root, "acme/prod"), "work")
	if err != nil {
		t.Fatal(err)
	}
	got, err := documentOperation(b, "document.get", json.RawMessage(`{"name":"docs/ARCHITECTURE.md"}`))
	if err != nil {
		t.Fatal(err)
	}
	doc := got.(taskDocument)
	content := "# Architecture\n"
	raw, _ := json.Marshal(map[string]any{"name": doc.Name, "revision": doc.Revision, "content": content})
	if _, err = documentOperation(b, "document.save", raw); err != nil {
		t.Fatal(err)
	}
	if _, err = documentOperation(b, "document.save", raw); err == nil {
		t.Fatal("stale revision accepted")
	}
	actual, _ := os.ReadFile(filepath.Join(root, "acme/prod/docs/ARCHITECTURE.md"))
	if string(actual) != content {
		t.Fatal("canonical document not published")
	}
	for _, name := range []string{"../other/AGENTS.md", ".hq/entity.json", "AGENTS.md", "repo/design.md"} {
		raw, _ = json.Marshal(map[string]string{"name": name})
		if _, err = documentOperation(b, "document.get", raw); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	os.Remove(filepath.Join(root, "acme/prod/docs/ARCHITECTURE.md"))
	os.Symlink(filepath.Join(root, "acme/AGENTS.md"), filepath.Join(root, "acme/prod/docs/ARCHITECTURE.md"))
	if _, err = documentOperation(b, "document.get", json.RawMessage(`{"name":"docs/ARCHITECTURE.md"}`)); err == nil {
		t.Fatal("symlink accepted")
	}
}
