package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) {
	t.Helper()
	t.Setenv("HQ_DATA_ROOT", t.TempDir())
	skills := t.TempDir()
	t.Setenv("HQ_SKILLS_ROOT", skills)
	if err := os.MkdirAll(filepath.Join(skills, "source-ingest"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skills, "source-ingest", "SKILL.md"), []byte("---\nname: source-ingest\ndescription: Capture sources\n---\nUse scoped sources."), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSaveUsesRevisionAndMaterializesOnlySelectedSkills(t *testing.T) {
	fixture(t)
	current, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	work := current.Presets["work"]
	work.Model = "gpt-5.6-sol"
	work.Skills = []string{"source-ingest"}
	current.Presets["work"] = work
	saved, err := Save(current, current.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Save(current, current.Revision); err == nil {
		t.Fatal("stale update accepted")
	}
	dest := filepath.Join(t.TempDir(), "codex")
	effective, err := Materialize("work", dest)
	if err != nil {
		t.Fatal(err)
	}
	if effective.Revision != saved.Revision || effective.Model != work.Model {
		t.Fatalf("wrong effective: %+v", effective)
	}
	config, err := os.ReadFile(filepath.Join(dest, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), "danger-full-access") || strings.Contains(string(config), "/Users/") {
		t.Fatal("unsafe imported config")
	}
	if _, err := os.Stat(filepath.Join(dest, "skills", "source-ingest", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "auth.json")); !os.IsNotExist(err) {
		t.Fatal("auth unexpectedly materialized")
	}
}

func TestImportIsAllowlistedAndPreviewDoesNotWrite(t *testing.T) {
	fixture(t)
	before, _ := Read()
	source := `model = "gpt-5.6-sol"
model_reasoning_effort = "xhigh"
sandbox_mode = "danger-full-access"
[mcp_servers.local]
command = "/Users/me/private/tool"
[hooks]
Stop = [{ hooks = [{ type = "command", command = "git push" }] }]
`
	preview, err := PreviewImport("work", source)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Preset.Model != "gpt-5.6-sol" || preview.Preset.Effort != "xhigh" {
		t.Fatal("preferences lost")
	}
	if len(preview.Preset.MCP) != 0 || len(preview.Replaced) == 0 || len(preview.Skipped) == 0 {
		t.Fatalf("unsafe import: %+v", preview)
	}
	after, _ := Read()
	if after.Revision != before.Revision {
		t.Fatal("preview wrote settings")
	}
	raw, _ := json.Marshal(preview)
	if strings.Contains(string(raw), "/Users/me/private/tool") || strings.Contains(string(raw), "git push") {
		t.Fatal("raw command leaked through report")
	}
}

func TestRejectsSkillTraversalAndSymlinkEscape(t *testing.T) {
	fixture(t)
	s, _ := Read()
	p := s.Presets["work"]
	p.Skills = []string{"../secret"}
	s.Presets["work"] = p
	if _, err := Save(s, s.Revision); err == nil {
		t.Fatal("traversal accepted")
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte("secret"), 0600)
	if err := os.Symlink(outside, filepath.Join(os.Getenv("HQ_SKILLS_ROOT"), "escape")); err != nil {
		t.Fatal(err)
	}
	s, _ = Read()
	p = s.Presets["work"]
	p.Skills = []string{"escape"}
	s.Presets["work"] = p
	if _, err := Save(s, s.Revision); err == nil {
		t.Fatal("external symlink skill accepted")
	}
}
