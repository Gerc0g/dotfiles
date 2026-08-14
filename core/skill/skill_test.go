package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makePaths(t *testing.T) Paths {
	t.Helper()
	base := t.TempDir()
	p := Paths{
		Source: filepath.Join(base, "skills"),
		Targets: []string{
			filepath.Join(base, "claude-skills"),
			filepath.Join(base, "codex-skills"),
		},
	}
	if err := os.MkdirAll(p.Source, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNewInstallDoctorRoundTrip(t *testing.T) {
	p := makePaths(t)

	file, err := New(p, "my-skill")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(file, filepath.Join("my-skill", "SKILL.md")) {
		t.Errorf("template path: %s", file)
	}

	if _, err := New(p, "My_Skill"); err == nil {
		t.Error("invalid name must be rejected")
	}
	if _, err := New(p, "my-skill"); err == nil {
		t.Error("duplicate skill must be rejected")
	}

	// Before install: doctor reports missing links.
	problems, err := Doctor(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 2 {
		t.Errorf("want 2 missing-link problems, got %v", problems)
	}

	linked, err := Install(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(linked) != 1 || linked[0] != "my-skill" {
		t.Errorf("linked: %v", linked)
	}

	problems, err = Doctor(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("clean install must pass doctor, got %v", problems)
	}

	infos, err := List(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range p.Targets {
		if infos[0].Status[target] != StatusLinked {
			t.Errorf("status in %s: %s", target, infos[0].Status[target])
		}
	}
}

func TestDoctorCatchesContractViolations(t *testing.T) {
	p := makePaths(t)

	// Frontmatter violations: name mismatch and unquoted description.
	dir := filepath.Join(p.Source, "bad-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: other-name\ndescription: unquoted\n---\n\n# bad\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(p); err != nil {
		t.Fatal(err)
	}

	problems, err := Doctor(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "name") {
		t.Errorf("want name-mismatch problem, got %v", problems)
	}

	// A dangling link in a profile — the orphan case.
	orphan := filepath.Join(p.Targets[0], "ghost")
	if err := os.Symlink(filepath.Join(p.Source, "deleted"), orphan); err != nil {
		t.Fatal(err)
	}
	problems, err = Doctor(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, problem := range problems {
		if strings.Contains(problem, "висячая") {
			found = true
		}
	}
	if !found {
		t.Errorf("orphan link must be reported, got %v", problems)
	}
}
