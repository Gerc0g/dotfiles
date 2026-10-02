package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gerc0g/dotfiles/core/entity"
)

func TestEntityCLIUnicodeUpdateAndStaleRevision(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PROKECTFILES_ROOT", root)
	dir := filepath.Join(root, "co")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".company-config"), []byte("slug: co\nnamespace: org\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		cmd := NewRoot()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	raw, err := run("entity", "show", "co", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var card entity.Card
	if err = json.Unmarshal([]byte(raw), &card); err != nil {
		t.Fatal(err)
	}
	content := "# Компания\n\nКонтекст с телефона и ноутбука.\n"
	body, _ := json.Marshal(entity.UpdateRequest{Revision: card.Document.Revision, Content: &content})
	encoded := base64.StdEncoding.EncodeToString(body)
	raw, err = run("entity", "update", "co", "--json-base64", encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(raw), &card); err != nil {
		t.Fatal(err)
	}
	if card.Document.Content != content {
		t.Fatal("Unicode content changed")
	}
	if _, err = run("entity", "update", "co", "--json-base64", encoded); err == nil || !strings.Contains(err.Error(), entity.RevisionConflict) {
		t.Fatalf("stale CLI update: %v", err)
	}
	malformed := base64.StdEncoding.EncodeToString([]byte(`{"revision":"x","settings":{"namespace":"org","host":"other"}}`))
	if _, err = run("entity", "update", "co", "--json-base64", malformed); err == nil {
		t.Fatal("unknown settings accepted")
	}
}

func TestEntityCLITaskUsesCanonicalDirectoryWithoutWritingState(t *testing.T) {
	root := t.TempDir()
	dotfiles := t.TempDir()
	t.Setenv("PROKECTFILES_ROOT", root)
	t.Setenv("DOTFILES", dotfiles)
	for path, content := range map[string]string{
		filepath.Join(root, "co", ".company-config"):                       "slug: co\nnamespace: org\n",
		filepath.Join(dotfiles, "skills", "onboard-agents-md", "SKILL.md"): "# Onboarding\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := NewRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"entity", "task", "co", "--action", "onboard", "--item", "network", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var task entity.AgentTask
	if err := json.Unmarshal(out.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	card, err := entity.Show(root, "co")
	if err != nil {
		t.Fatal(err)
	}
	if task.Directory != card.Path || task.Scope != "co" || task.Kind != "company" || task.ItemID != "network" || task.Prompt == "" {
		t.Fatalf("wrong task: %+v", task)
	}
	for _, path := range []string{"AGENTS.md", ".hq"} {
		if _, err := os.Stat(filepath.Join(root, "co", path)); !os.IsNotExist(err) {
			t.Fatalf("task created %s: %v", path, err)
		}
	}
}
