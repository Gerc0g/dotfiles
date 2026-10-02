package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Gerc0g/dotfiles/core/editor"
	"github.com/Gerc0g/dotfiles/core/world"
)

func TestOnboardRepoUsesExistingProductWithoutRewritingContext(t *testing.T) {
	root := t.TempDir()
	t.Setenv(world.RootEnv, root)
	t.Setenv(editor.ProjectsFileEnv, filepath.Join(t.TempDir(), "projects.json"))
	prod := filepath.Join(root, "org", "app")
	repo := filepath.Join(prod, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(root, "org", ".company-config"): "slug: org\nvcs: local\nhost: local\nnamespace: org\n",
		filepath.Join(prod, ".product-config"):        "slug: app\nnamespace: org\ncustom: preserved\n",
		filepath.Join(prod, "AGENTS.md"):              "# User product instructions\n",
		filepath.Join(prod, ".envrc"):                 "# User environment\n",
		filepath.Join(repo, "AGENTS.md"):              "# Upstream instructions\n",
		filepath.Join(repo, ".envrc"):                 "# Existing repo environment\n",
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	cmd := NewRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"onboard", "repo", "org", "app", "repo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("onboard repo: %v: %s", err, out.String())
	}
	for path, body := range files {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != body {
			t.Errorf("changed existing %s", path)
		}
	}
}

func TestOnboardCLIRejectsInvalidCreateArguments(t *testing.T) {
	root := t.TempDir()
	t.Setenv(world.RootEnv, root)
	t.Setenv(editor.ProjectsFileEnv, filepath.Join(t.TempDir(), "projects.json"))
	for _, args := range [][]string{
		{"onboard", "company", "../escape", "local", "org"},
		{"onboard", "product", "org", "../escape"},
		{"onboard", "repo", "org", "app"},
	} {
		cmd := NewRoot()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("invalid requests left files: %v", entries)
	}
}
