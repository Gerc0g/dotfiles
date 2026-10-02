package runner

import (
	"context"
	"encoding/json"
	"github.com/Gerc0g/dotfiles/core/entity"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestContextBrokerBindsReadsAndCASWritesWithoutHumanConfirmation(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("PROKECTFILES_ROOT", root)
	for _, path := range []string{"acme/.company-config", "acme/prod/.product-config", "other/.company-config"} {
		p := filepath.Join(root, path)
		os.MkdirAll(filepath.Dir(p), 0700)
		os.WriteFile(p, []byte("name: test\n"), 0600)
	}
	os.WriteFile(filepath.Join(root, "acme/AGENTS.md"), []byte("# Acme\n"), 0600)
	b, err := Resolve(filepath.Join(root, "acme"), "work")
	if err != nil || b.Kind != "company" {
		t.Fatalf("%+v %v", b, err)
	}
	result, err := contextOperation(b, "entity.show", json.RawMessage(`{"scope":"acme"}`))
	if err != nil {
		t.Fatal(err)
	}
	card := result.(entity.Card)
	content := "# Changed context\n"
	req := map[string]any{"scope": "acme", "request": entity.UpdateRequest{Revision: card.Document.Revision, Content: &content}}
	raw, _ := json.Marshal(req)
	if _, err = contextOperation(b, "entity.update", raw); err != nil {
		t.Fatal(err)
	}
	if _, err = contextOperation(b, "entity.update", raw); err == nil {
		t.Fatal("stale CAS accepted")
	}
	for _, raw := range []string{`{"scope":"other"}`, `{"scope":"acme/prod","request":{"revision":"x","content":"forbidden"}}`, `{"scope":"acme","request":{"revision":"x","confirm":{"id":"purpose","confirmed":true}}}`} {
		op := "entity.update"
		if raw == `{"scope":"other"}` {
			op = "entity.show"
		}
		if _, err = contextOperation(b, op, json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestProductContextCopiesOnlyContextAndListsRegisteredSources(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("PROKECTFILES_ROOT", root)
	for _, path := range []string{"acme/.company-config", "acme/prod/.product-config", "acme/prod/repo/.git/HEAD", "acme/prod/not-a-repo/private", "acme/other/.product-config", "acme/other/repo/.git/HEAD"} {
		p := filepath.Join(root, path)
		os.MkdirAll(filepath.Dir(p), 0700)
		os.WriteFile(p, nil, 0600)
	}
	os.WriteFile(filepath.Join(root, "acme/prod/AGENTS.md"), []byte("# Product\n"), 0600)
	os.WriteFile(filepath.Join(root, "acme/prod/.env.secret"), []byte("secret"), 0600)
	b, err := Resolve(filepath.Join(root, "acme/prod"), "work")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := filepath.EvalSymlinks(t.TempDir())
	workspace, repos, err := prepareContextWorkspace(b, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0] != filepath.Join(root, "acme/prod/repo") {
		t.Fatalf("%v", repos)
	}
	if _, err = os.Stat(filepath.Join(workspace, ".env.secret")); !os.IsNotExist(err) {
		t.Fatal("company secret copied")
	}
	if _, err = contextOperation(b, "entity.show", json.RawMessage(`{"scope":"acme/prod/repo"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err = contextOperation(b, "entity.show", json.RawMessage(`{"scope":"acme/other/repo"}`)); err == nil {
		t.Fatal("unrelated product accepted")
	}
	if _, err = contextOperation(b, "entity.show", json.RawMessage(`{"scope":"acme"}`)); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(workspace)
	os.Symlink(t.TempDir(), workspace)
	if _, _, err = prepareContextWorkspace(b, home); err == nil {
		t.Fatal("persistent scratch symlink accepted")
	}
}

func TestSandboxCLIBlocksOwnerCommandsAndPreservesHostCLI(t *testing.T) {
	t.Setenv("HQ_RUNNER_SANDBOX", "")
	if handled, _ := SandboxCLI(context.Background(), []string{"setup"}, io.Discard); handled {
		t.Fatal("host CLI changed")
	}
	t.Setenv("HQ_RUNNER_SANDBOX", "1")
	if handled, err := SandboxCLI(context.Background(), []string{"control", "request"}, io.Discard); !handled || err == nil {
		t.Fatal("owner control accepted")
	}
	if handled, _ := SandboxCLI(context.Background(), []string{"runner", "inside"}, io.Discard); handled {
		t.Fatal("inside runner blocked")
	}
}
