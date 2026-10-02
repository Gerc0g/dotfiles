package entity

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Gerc0g/dotfiles/core/workspace"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"co/.company-config":     "slug: co\nnamespace: org\nvcs: github\nhost: github.com\ngit_email: dev@example.com\n",
		"co/AGENTS.md":           "# Company\n## Coordinates\nNetwork: private VPN\n## Stance\nReview changes.\n## Git workflow\nFeature branches.\n## Stack overrides\nNo overrides.\n## Secrets\nNo sensitive data.\n## External references\nNo external references yet.\n",
		"co/app/.product-config": "slug: app\nnamespace: org/app\ncustom: preserve\n",
		"co/app/AGENTS.md":       "# Product\n## What this is\nAn app for customers. Status: experimental.\n## Repos and roles\napi: backend\n## Cross-repo conventions\nNo shared schemas.\n## Data flow (only if non-obvious)\nNo cross-repo flow.\n## Boundaries (product-specific)\nAsk before migrations.\n",
		"co/app/api/.git":        "broken git marker\n",
		"co/app/api/AGENTS.md":   "# Repo\n## What this repo does\nTODO: describe it\n",
	}
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestShowReadOnlyAndOwnHealthSeparate(t *testing.T) {
	root := fixture(t)
	c, err := Show(root, "co/app")
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != 1 || c.Kind != "product" || len(c.Children) != 1 {
		t.Fatalf("bad card: %+v", c)
	}
	if c.Health.Status == "error" || c.Children[0].HealthStatus != "error" {
		t.Fatalf("child errors contaminated own health: %+v / %+v", c.Health, c.Children)
	}
	if _, err := os.Stat(filepath.Join(root, "co/app/.hq")); !os.IsNotExist(err) {
		t.Fatalf("read created state: %v", err)
	}
	for _, check := range c.Health.Checks {
		if check.ID == "onboarding" && check.Blocking {
			t.Fatal("incomplete onboarding must not block")
		}
	}
}

func TestConfirmationUsesSectionRevisionAndConflict(t *testing.T) {
	root := fixture(t)
	c, err := Show(root, "co/app")
	if err != nil {
		t.Fatal(err)
	}
	c, err = Update(root, "co/app", UpdateRequest{Revision: c.Document.Revision, Confirm: &Confirmation{ID: "purpose", Confirmed: true}})
	if err != nil {
		t.Fatal(err)
	}
	if status(c, "purpose") != "ready" {
		t.Fatalf("purpose not ready: %+v", c.Onboarding)
	}
	stale := c.Document.Revision
	changed := strings.Replace(c.Document.Content, "No shared schemas.", "Shared schemas live in api.", 1)
	c, err = Update(root, "co/app", UpdateRequest{Revision: stale, Content: &changed})
	if err != nil {
		t.Fatal(err)
	}
	if status(c, "purpose") != "ready" {
		t.Fatal("unrelated edit invalidated confirmation")
	}
	if _, err = Update(root, "co/app", UpdateRequest{Revision: stale, Content: &changed}); err == nil || !strings.Contains(err.Error(), "HQ_ENTITY_REVISION_CONFLICT") {
		t.Fatalf("stale save: %v", err)
	}
	changed = strings.Replace(changed, "An app for customers.", "A different product.", 1)
	c, err = Update(root, "co/app", UpdateRequest{Revision: c.Document.Revision, Content: &changed})
	if err != nil {
		t.Fatal(err)
	}
	if status(c, "purpose") != "review" {
		t.Fatalf("changed section remains confirmed: %+v", c.Onboarding)
	}
	if len(c.History) != 3 {
		t.Fatalf("missing history: %+v", c.History)
	}
}

func TestPlaceholdersCannotBeConfirmed(t *testing.T) {
	root := fixture(t)
	c, err := Show(root, "co/app/api")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Update(root, c.Scope, UpdateRequest{Revision: c.Document.Revision, Confirm: &Confirmation{ID: "purpose", Confirmed: true}}); err == nil {
		t.Fatal("confirmed placeholder")
	}
}

func TestNetworkLabelWithoutAnswerCannotBeConfirmed(t *testing.T) {
	root := fixture(t)
	path := filepath.Join(root, "co/AGENTS.md")
	body, _ := os.ReadFile(path)
	body = []byte(strings.Replace(string(body), "Network: private VPN", "- **Network:** <!-- answer later -->", 1))
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
	c, err := Show(root, "co")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Update(root, c.Scope, UpdateRequest{Revision: c.Document.Revision, Confirm: &Confirmation{ID: "network", Confirmed: true}}); err == nil {
		t.Fatal("confirmed a label without an answer")
	}
}

func TestConcurrentUpdatesOnlyOneWins(t *testing.T) {
	root := fixture(t)
	c, err := Show(root, "co/app")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, value := range []string{"first", "second"} {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			_, e := Update(root, c.Scope, UpdateRequest{Revision: c.Document.Revision, Content: &s})
			errs <- e
		}(value)
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		} else if !strings.Contains(err.Error(), "HQ_ENTITY_REVISION_CONFLICT") {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("successful concurrent writes: %d", success)
	}
}

func TestPathsAndSettingsPreserveOtherConfig(t *testing.T) {
	root := fixture(t)
	if _, err := Show(root, "../co"); err == nil {
		t.Fatal("traversal accepted")
	}
	c, err := Show(root, "co/app")
	if err != nil {
		t.Fatal(err)
	}
	c, err = Update(root, c.Scope, UpdateRequest{Revision: c.Document.Revision, Settings: &SettingsUpdate{Namespace: "org/new-app"}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "co/app/.product-config"))
	if !strings.Contains(string(b), "custom: preserve") || c.Settings.Namespace != "org/new-app" {
		t.Fatalf("settings save lost config: %s", b)
	}
	for _, ns := range []string{"../evil", "org\ninjected: yes", "$(whoami)", "-option", "org//bad"} {
		if _, err = Update(root, c.Scope, UpdateRequest{Revision: c.Document.Revision, Settings: &SettingsUpdate{Namespace: ns}}); err == nil {
			t.Fatalf("accepted namespace %q", ns)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, []byte("keep"), 0600)
	os.Remove(filepath.Join(root, "co/app/AGENTS.md"))
	os.Symlink(outside, filepath.Join(root, "co/app/AGENTS.md"))
	if _, err = Show(root, c.Scope); err == nil {
		t.Fatal("followed document symlink")
	}
}

func status(c Card, id string) string {
	for _, i := range c.Onboarding.Items {
		if i.ID == id {
			return i.Status
		}
	}
	return ""
}

func TestHealthPreservesWorktreeContextCollision(t *testing.T) {
	root := fixture(t)
	repo := filepath.Join(root, "co", "app", "api")
	if err := os.Remove(filepath.Join(repo, ".git")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.invalid"},
		{"config", "user.name", "test"},
		{"add", "AGENTS.md"},
		{"commit", "-qm", "seed context"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	manager, err := workspace.New(root, workspace.WithProjectSync(func() {}), workspace.WithWarnings(io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	w, err := manager.Start("co", "app", "api", "context-health")
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(w.Path, "..", "AGENTS.md")
	const custom = "Custom parent content; preserve this file.\n"
	if err := os.WriteFile(parent, []byte(custom), 0644); err != nil {
		t.Fatal(err)
	}
	if issues, err := manager.ContextIssues(w); err == nil || len(issues) == 0 {
		t.Fatalf("fixture did not produce a context collision: %+v (%v)", issues, err)
	}
	card, err := Show(root, "co/app/api")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, check := range card.Health.Checks {
		if check.Path == parent && check.Status == "error" {
			found = true
		}
	}
	if !found || card.Health.Status != "error" {
		t.Fatalf("material worktree error disappeared from health: %+v", card.Health)
	}
	company, err := Show(root, "co")
	if err != nil {
		t.Fatal(err)
	}
	if company.Health.Status == "error" || len(company.Children) != 1 || company.Children[0].ErrorCount == 0 {
		t.Fatalf("descendant failure must remain separate but visible: %+v / %+v", company.Health, company.Children)
	}
	if data, err := os.ReadFile(parent); err != nil || string(data) != custom {
		t.Fatalf("read-only health changed custom adapter: %q (%v)", data, err)
	}
}
