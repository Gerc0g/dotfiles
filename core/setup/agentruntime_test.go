package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeRuntimeEnv builds a temp Env with the hook/skill sources present.
func makeRuntimeEnv(t *testing.T) Env {
	t.Helper()
	home := t.TempDir()
	env := Env{
		Home:      home,
		Dotfiles:  filepath.Join(home, "dotfiles"),
		Workspace: filepath.Join(home, "work"),
		Vault:     filepath.Join(home, "vault"),
	}

	for _, source := range hookSources {
		path := filepath.Join(env.Dotfiles, "skills-stash", "wiki", "hooks", source)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, skill := range curatorSkills {
		dir := filepath.Join(env.Dotfiles, "skills-stash", "wiki", "curator", skill)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return env
}

func TestWikiHooksAndCuratorSkillsSteps(t *testing.T) {
	env := makeRuntimeEnv(t)

	for _, step := range []Step{wikiHooksStep(), curatorSkillsStep()} {
		if result := step.Check(env); !result.Status.NeedsApply() {
			t.Fatalf("%s: fresh env must need apply, got %s", step.Name, result.Status)
		}
		if err := step.Apply(env); err != nil {
			t.Fatalf("%s: %v", step.Name, err)
		}
		if result := step.Check(env); result.Status != StatusOK {
			t.Errorf("%s after apply: %s (%s)", step.Name, result.Status, result.Detail)
		}
	}
}

func TestClaudeSettingsStepMergesWithoutDestroying(t *testing.T) {
	env := makeRuntimeEnv(t)
	step := claudeSettingsStep()

	// Pre-existing user settings must survive the merge.
	path := claudeSettingsPath(env)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := step.Apply(env); err != nil {
		t.Fatal(err)
	}
	if result := step.Check(env); result.Status != StatusOK {
		t.Fatalf("after apply: %s (%s)", result.Status, result.Detail)
	}

	var data map[string]any
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data["theme"] != "dark" {
		t.Error("user setting must survive")
	}
	if v, _ := data["includeCoAuthoredBy"].(bool); v {
		t.Error("includeCoAuthoredBy must be false")
	}

	// Idempotent: no duplicate hook entries on re-apply.
	if err := step.Apply(env); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if strings.Count(string(raw), "SessionStart.sh") != 1 {
		t.Errorf("hook entry duplicated:\n%s", raw)
	}

	// Invalid JSON is refused, never overwritten.
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := step.Apply(env); err == nil {
		t.Error("invalid JSON must refuse apply")
	}
}

func TestCodexConfigStep(t *testing.T) {
	env := makeRuntimeEnv(t)
	step := codexConfigStep()

	if err := step.Apply(env); err != nil {
		t.Fatal(err)
	}
	if result := step.Check(env); result.Status != StatusOK {
		t.Fatalf("after apply: %s (%s)", result.Status, result.Detail)
	}

	raw, _ := os.ReadFile(codexConfigPath(env))
	content := string(raw)
	if !strings.Contains(content, "[hooks]") || !strings.Contains(content, "SessionStart.sh") {
		t.Errorf("hook block missing:\n%s", content)
	}
	if strings.Count(content, "trust_level") != len(vaultTrustZones(env)) {
		t.Errorf("trust entries wrong:\n%s", content)
	}

	// Foreign [hooks] section without our hook: refuse, do not mangle.
	if err := os.WriteFile(codexConfigPath(env), []byte("[hooks]\nSessionStart = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := step.Apply(env); err == nil {
		t.Error("foreign hooks section must refuse apply")
	}
}
