package runner

import (
	"encoding/json"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

func TestContextArchitectureCannotAliasMaskedCredential(t *testing.T) {
	source, dest := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".env.private"), []byte("private fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../.env.private", filepath.Join(source, "docs/ARCHITECTURE.md")); err != nil {
		t.Fatal(err)
	}
	if err := copyContextDocument(source, dest, "docs/ARCHITECTURE.md"); err == nil {
		t.Fatal("masked credential aliased as architecture was copied into task workspace")
	}
	if _, err := os.Stat(filepath.Join(dest, "docs/ARCHITECTURE.md")); !os.IsNotExist(err) {
		t.Fatal("private source bytes reached scratch")
	}
}

func TestAnalysisDocumentRejectsFIFOAndDirectoryAliases(t *testing.T) {
	root := t.TempDir()
	if err := unix.Mkfifo(filepath.Join(root, "design.md"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readAnalysisDocument(root, "design.md"); err == nil {
		t.Fatal("FIFO accepted")
	}
	os.Mkdir(filepath.Join(root, "private"), 0700)
	os.WriteFile(filepath.Join(root, "private/ARCHITECTURE.md"), []byte("hidden"), 0600)
	os.Symlink("private", filepath.Join(root, "docs"))
	if _, _, err := readAnalysisDocument(root, "docs/ARCHITECTURE.md"); err == nil {
		t.Fatal("directory alias accepted")
	}
}

func TestDocumentPublicationRecoversAfterInterruptedStage(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("PROKECTFILES_ROOT", root)
	scope := filepath.Join(root, "acme/product")
	os.MkdirAll(filepath.Join(scope, "docs"), 0700)
	os.WriteFile(filepath.Join(root, "acme/.company-config"), nil, 0600)
	os.WriteFile(filepath.Join(scope, ".product-config"), nil, 0600)
	b, err := Resolve(scope, "work")
	if err != nil {
		t.Fatal(err)
	}
	got, err := documentOperation(b, "document.get", json.RawMessage(`{"name":"docs/ARCHITECTURE.md"}`))
	if err != nil {
		t.Fatal(err)
	}
	doc := got.(taskDocument)
	// A killed publisher can leave its staging file after its lock is released.
	os.WriteFile(filepath.Join(scope, "docs/ARCHITECTURE.md.hq-pending"), []byte("interrupted draft"), 0600)
	req, _ := json.Marshal(map[string]string{"name": doc.Name, "revision": doc.Revision, "content": "# Recovered"})
	if _, err := documentOperation(b, "document.save", req); err != nil {
		t.Fatalf("interrupted stage blocks future canonical publications: %v", err)
	}
}
