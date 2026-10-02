package knowledge_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Gerc0g/dotfiles/core/knowledge"
	"github.com/Gerc0g/dotfiles/core/runner"
)

func TestResearchDirectoryMatchesContextAndRunner(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HQ_WIKI_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "research"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "vault")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HQ_WIKI_ROOT", alias)
	directory, err := knowledge.ResearchDirectory()
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(root, "research"))
	if err != nil || directory != want {
		t.Fatalf("directory = %q, want %q (%v)", directory, want, err)
	}
	result, err := knowledge.Dispatch("knowledge.context", json.RawMessage(`{"scope":{"kind":"research"}}`))
	if err != nil {
		t.Fatal(err)
	}
	context := result.(map[string]any)
	if context["directory"] != directory || context["preset"] != "research" {
		t.Fatalf("context = %+v", context)
	}
	binding, err := runner.Resolve(directory, "work")
	if err != nil || binding.Preset != "research" || binding.CompanyID != "" {
		t.Fatalf("context does not bind the isolated research runner: %+v, %v", binding, err)
	}
}

func TestResearchDirectoryUsesRunnerHomeDefaultAndRejectsZoneAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HQ_WIKI_ROOT", "")
	root := filepath.Join(home, "Desktop", "WikiPedik")
	if err := os.MkdirAll(filepath.Join(root, "research"), 0700); err != nil {
		t.Fatal(err)
	}
	directory, err := knowledge.ResearchDirectory()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := runner.Resolve(directory, "research")
	if err != nil || binding.WorktreePath != directory {
		t.Fatalf("default research directory differs from runner: %+v, %v", binding, err)
	}
	if err := os.Rename(filepath.Join(root, "research"), filepath.Join(root, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("original", filepath.Join(root, "research")); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge.ResearchDirectory(); err == nil {
		t.Fatal("research directory accepted a zone alias")
	}
}
