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

	for _, hooks := range profileHooks {
		for _, source := range hooks {
			path := filepath.Join(env.Dotfiles, "skills-stash", "wiki", "hooks", source)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
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
	if !strings.Contains(content, "[hooks]") {
		t.Errorf("hook block missing:\n%s", content)
	}
	for _, script := range sessionEvents {
		if !strings.Contains(content, script) {
			t.Errorf("%s missing:\n%s", script, content)
		}
	}
	if strings.Count(content, "trust_level") != len(vaultTrustZones(env)) {
		t.Errorf("trust entries wrong:\n%s", content)
	}

	// Foreign [hooks] section that already owns the event: refuse, do not mangle.
	if err := os.WriteFile(codexConfigPath(env), []byte("[hooks]\nSessionStart = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := step.Apply(env); err == nil {
		t.Error("foreign hooks section must refuse apply")
	}
}

// The real config.toml carries hooks from other tools. Our events must slot
// into that table instead of appending a second [hooks] header, which TOML
// forbids.
func TestCodexConfigStepJoinsForeignHooksTable(t *testing.T) {
	env := makeRuntimeEnv(t)
	step := codexConfigStep()

	foreign := "model = \"x\"\n\n[hooks]\nNotification = [\n" +
		"  { matcher = \"*\", hooks = [{ type = \"command\", command = \"/other/tool\" }] },\n]\n"
	if err := os.MkdirAll(filepath.Dir(codexConfigPath(env)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexConfigPath(env), []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := step.Apply(env); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(codexConfigPath(env))
	content := string(raw)

	if strings.Count(content, "[hooks]") != 1 {
		t.Errorf("[hooks] duplicated:\n%s", content)
	}
	if !strings.Contains(content, "/other/tool") {
		t.Errorf("foreign hook dropped:\n%s", content)
	}
	for _, script := range sessionEvents {
		if !strings.Contains(content, script) {
			t.Errorf("%s missing:\n%s", script, content)
		}
	}

	// Second apply must be a no-op.
	if err := step.Apply(env); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(codexConfigPath(env))
	if string(again) != content {
		t.Errorf("apply is not idempotent:\n%s", string(again))
	}
}
