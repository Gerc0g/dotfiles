package entity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installTaskSkill(t *testing.T) string {
	t.Helper()
	dotfiles := t.TempDir()
	t.Setenv("DOTFILES", dotfiles)
	path := filepath.Join(dotfiles, "skills", "onboard-agents-md", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Onboarding\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTaskIsScopedReadOnlyAndDoesNotNeedExistingContext(t *testing.T) {
	root := fixture(t)
	skillPath := installTaskSkill(t)
	for _, scope := range []string{"co", "co/app", "co/app/api"} {
		doc := filepath.Join(root, scope, "AGENTS.md")
		if err := os.Remove(doc); err != nil {
			t.Fatal(err)
		}
		before, err := Show(root, scope)
		if err != nil {
			t.Fatal(err)
		}
		task, err := Task(root, scope, "onboard", "")
		if err != nil {
			t.Fatal(err)
		}
		if task.Version != 1 || task.Scope != scope || task.Kind != before.Kind || task.Directory != before.Path || task.Action != "onboard" || task.Skill != "onboard-agents-md" || task.ItemID != "" {
			t.Fatalf("wrong task: %+v", task)
		}
		if !strings.Contains(task.Prompt, skillPath) || !strings.Contains(task.Prompt, "hq entity show "+scope+" --json") {
			t.Fatalf("task lost canonical skill or scope: %s", task.Prompt)
		}
		after, err := Show(root, scope)
		if err != nil {
			t.Fatal(err)
		}
		if before.Document.Revision != after.Document.Revision {
			t.Fatal("task changed context/state")
		}
		for _, path := range []string{doc, filepath.Join(root, scope, ".hq")} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("task created %s: %v", path, err)
			}
		}
	}
}

func TestTaskValidatesCapabilityScopeAndManualItem(t *testing.T) {
	root := fixture(t)
	installTaskSkill(t)
	task, err := Task(root, "co/app", "onboard", "purpose")
	if err != nil || task.ItemID != "purpose" || !strings.Contains(task.Prompt, "purpose") {
		t.Fatalf("focused task: %+v %v", task, err)
	}
	for _, input := range [][3]string{{"co", "unknown", ""}, {"co/app", "onboard", "coordinates"}, {"co/app", "onboard", "network"}, {"co/missing", "onboard", ""}, {"co/../app", "onboard", ""}} {
		if _, err := Task(root, input[0], input[1], input[2]); err == nil {
			t.Fatalf("accepted invalid task: %v", input)
		}
	}
	card, err := Show(root, "co/app")
	if err != nil || len(card.AgentActions) != 1 || card.AgentActions[0].ID != "onboard" {
		t.Fatalf("missing capability: %+v %v", card.AgentActions, err)
	}
	t.Setenv("DOTFILES", t.TempDir())
	card, err = Show(root, "co/app")
	if err != nil || len(card.AgentActions) != 0 {
		t.Fatalf("advertised unavailable skill: %+v %v", card.AgentActions, err)
	}
	if _, err = Task(root, "co/app", "onboard", ""); err == nil {
		t.Fatal("task accepted unavailable skill")
	}
}

func TestOnboardingDistinguishesPlaceholdersFromOrdinaryProse(t *testing.T) {
	root := fixture(t)
	for _, text := range []string{"<install>", "<dev cmd>", `<or "none">`, "Unknown", "**Unknown**", "- **Purpose:** Unknown", "- Status: **Unknown**", "`unknown`", "Purpose: not found in repo", "No data", "TODO: purpose"} {
		body := "## What this repo does\n" + text + "\n"
		if err := os.WriteFile(filepath.Join(root, "co/app/api/AGENTS.md"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		card, err := Show(root, "co/app/api")
		if err != nil {
			t.Fatal(err)
		}
		if status(card, "purpose") != "missing" {
			t.Errorf("accepted placeholder %q", text)
		}
		if _, err := Update(root, card.Scope, UpdateRequest{Revision: card.Document.Revision, Confirm: &Confirmation{ID: "purpose", Confirmed: true}}); err == nil {
			t.Errorf("confirmed placeholder %q", text)
		}
	}
	for _, text := range []string{"The decoder rejects unknown fields.", "This service stores no data.", "The optional SDK was not found in repo fixtures."} {
		body := "## What this repo does\n" + text + "\n"
		if err := os.WriteFile(filepath.Join(root, "co/app/api/AGENTS.md"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		card, err := Show(root, "co/app/api")
		if err != nil {
			t.Fatal(err)
		}
		if status(card, "purpose") != "review" || card.Onboarding.Status != "in_progress" {
			t.Errorf("valid prose blocked or progress not reflected: %q %+v", text, card.Onboarding)
		}
		card, err = Update(root, card.Scope, UpdateRequest{Revision: card.Document.Revision, Confirm: &Confirmation{ID: "purpose", Confirmed: true}})
		if err != nil || status(card, "purpose") != "ready" {
			t.Fatalf("cannot confirm real content: %v", err)
		}
	}
}
