package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture builds a fake machine: an empty home plus a dotfiles checkout that
// carries the source files the plan links to.
func fixture(t *testing.T) Env {
	t.Helper()

	base := t.TempDir()
	home := filepath.Join(base, "home")
	dotfiles := filepath.Join(home, "dotfiles")

	// An empty remote makes the vault step build a local skeleton instead of
	// cloning, so tests never touch the network.
	t.Setenv(VaultRemoteEnv, "")

	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(dotfiles, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("agent-profiles/BASELINE.md", "# baseline\n")
	write("ghostty/config", "font-size = 14\n")
	write("tmux/tmux.conf", "set -g mouse on\n")
	write("scripts/transcript-scrub.py", "print('x')\n")
	for _, hooks := range profileHooks {
		for _, source := range hooks {
			write("skills-stash/wiki/hooks/"+source, "#!/bin/sh\n")
		}
	}
	for _, source := range codexZoneHooks {
		write("skills-stash/wiki/hooks/"+source, "#!/bin/sh\n")
	}
	for _, skill := range curatorSkills {
		write("skills-stash/wiki/curator/"+skill+"/SKILL.md", "---\n---\n")
	}
	for _, zone := range codexZones {
		write("agent-profiles/codex-"+zone+"/config.toml", "model = \"x\"\n[projects.\"{{HOME}}/vault\"]\n")
	}

	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}

	// The default codex profile as it looks after `codex login`: zone profiles
	// share these two entries instead of duplicating them.
	if err := os.MkdirAll(filepath.Join(home, ".codex", "plugins"), 0o755); err != nil {
		t.Fatalf("mkdir codex plugins: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".codex", "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}

	return Env{
		Home:      home,
		Dotfiles:  dotfiles,
		Workspace: filepath.Join(home, "Desktop", "Prokectfiles"),
		Vault:     filepath.Join(home, "Desktop", "WikiPedik"),
	}
}

// applicableSteps drops the launchd step: applying it would talk to the real
// launchctl of the machine running the tests.
func applicableSteps() []Step {
	// The launchd steps register real jobs in the user domain; a test must
	// not touch the machine it runs on.
	scheduled := map[string]bool{"routines": true, "routines-migrated": true}

	var steps []Step
	for _, step := range Plan() {
		if !scheduled[step.Name] {
			steps = append(steps, step)
		}
	}
	return steps
}

func TestFreshMachineNeedsEverything(t *testing.T) {
	env := fixture(t)

	reports := Check(env, applicableSteps())
	for _, report := range reports {
		// Health checks look at the machine running the test rather than at
		// the fixture, so an empty fixture says nothing about them: no links
		// exist to dangle, and the real PATH still has the real binaries.
		if report.Step.Name == "skill-links" || report.Step.Name == "tooling" {
			continue
		}
		if report.Result.Status == StatusOK {
			t.Errorf("step %q reported ok on an empty machine: %s", report.Step.Name, report.Result.Detail)
		}
	}
}

func TestApplyMakesCheckPass(t *testing.T) {
	env := fixture(t)
	steps := applicableSteps()

	reports, errs := Apply(env, steps)
	if len(errs) > 0 {
		t.Fatalf("Apply errors: %v", errs)
	}

	for _, report := range reports {
		// Check-only steps observe things Apply does not own — whether the
		// binary is built, whether hooks have been firing. Apply cannot make
		// them green and is not supposed to.
		if !report.Step.Applicable() {
			continue
		}
		if report.Result.Status != StatusOK {
			t.Errorf("after Apply, %q = %s (%s)", report.Step.Name, report.Result.Status, report.Result.Detail)
		}
	}

	baseline := filepath.Join(env.Home, ".claude", "CLAUDE.md")
	target, err := os.Readlink(baseline)
	if err != nil {
		t.Fatalf("readlink %s: %v", baseline, err)
	}
	if !strings.HasSuffix(target, "agent-profiles/BASELINE.md") {
		t.Errorf("CLAUDE.md → %s, want the baseline", target)
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	env := fixture(t)
	steps := applicableSteps()

	if _, errs := Apply(env, steps); len(errs) > 0 {
		t.Fatalf("first Apply: %v", errs)
	}

	zshrc := filepath.Join(env.Home, ".zshrc")
	first, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}

	if _, errs := Apply(env, steps); len(errs) > 0 {
		t.Fatalf("second Apply: %v", errs)
	}

	second, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("re-read .zshrc: %v", err)
	}

	if string(first) != string(second) {
		t.Error("second Apply appended the loader block again")
	}
	if got := strings.Count(string(second), loaderBlock); got != 1 {
		t.Errorf("loader block appears %d times, want 1", got)
	}
}

// Only the two default profiles are managed. The retired ones — the -new and
// -setup pairs and the curator's -wiki — must never come back: recreating them
// would rebuild exactly the split that let a GUI-launched editor pick up a
// different profile from the terminal.
func TestOnlyDefaultProfilesAreCreated(t *testing.T) {
	env := fixture(t)

	if _, errs := Apply(env, applicableSteps()); len(errs) > 0 {
		t.Fatalf("Apply: %v", errs)
	}

	for _, profile := range []string{".claude", ".codex"} {
		if _, err := os.Stat(filepath.Join(env.Home, profile)); err != nil {
			t.Errorf("профиль %s не создан: %v", profile, err)
		}
	}

	for _, retired := range []string{".claude-new", ".codex-new", ".claude-setup", ".codex-setup", ".codex-wiki", ".claude-analyst"} {
		if _, err := os.Stat(filepath.Join(env.Home, retired)); err == nil {
			t.Errorf("отменённый профиль %s создан заново", retired)
		}
	}
}

func TestEnsureSymlinkBacksUpRegularFile(t *testing.T) {
	env := fixture(t)

	dst := filepath.Join(env.Home, ".tmux.conf")
	if err := os.WriteFile(dst, []byte("hand-written\n"), 0o644); err != nil {
		t.Fatalf("seed %s: %v", dst, err)
	}

	src := filepath.Join(env.Dotfiles, "tmux", "tmux.conf")
	if got := checkSymlink(src, dst).Status; got != StatusDrifted {
		t.Errorf("regular file should read as drifted, got %s", got)
	}

	if err := ensureSymlink(src, dst); err != nil {
		t.Fatalf("ensureSymlink: %v", err)
	}
	if got := checkSymlink(src, dst).Status; got != StatusOK {
		t.Errorf("after ensureSymlink, status = %s", got)
	}

	entries, err := os.ReadDir(env.Home)
	if err != nil {
		t.Fatalf("read home: %v", err)
	}
	var backups int
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmux.conf.bak.") {
			backups++
		}
	}
	if backups != 1 {
		t.Errorf("backups = %d, want 1 — the hand-written file must survive", backups)
	}
}

func TestDanglingSkillLinkIsReported(t *testing.T) {
	env := fixture(t)

	skills := filepath.Join(env.Home, ".claude", "skills")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.Symlink(filepath.Join(env.Dotfiles, "skills", "gone"), filepath.Join(skills, "gone")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	result := skillLinksStep().Check(env)
	if result.Status != StatusDrifted {
		t.Fatalf("status = %s, want drifted", result.Status)
	}
	if !strings.Contains(result.Detail, "gone") {
		t.Errorf("detail %q should name the dangling link", result.Detail)
	}
}

func TestCheckSymlinkDetectsWrongTarget(t *testing.T) {
	env := fixture(t)

	src := filepath.Join(env.Dotfiles, "tmux", "tmux.conf")
	other := filepath.Join(env.Dotfiles, "ghostty", "config")
	dst := filepath.Join(env.Home, ".tmux.conf")

	if err := os.Symlink(other, dst); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	result := checkSymlink(src, dst)
	if result.Status != StatusDrifted {
		t.Errorf("status = %s, want drifted", result.Status)
	}
}

func TestCombineKeepsWorstStatus(t *testing.T) {
	cases := []struct {
		name    string
		results []Result
		want    Status
	}{
		{"all ok", []Result{ok("a"), ok("b")}, StatusOK},
		{"one missing", []Result{ok("a"), missing("b")}, StatusMissing},
		{"drift beats missing", []Result{missing("a"), drifted("b")}, StatusDrifted},
	}

	for _, tc := range cases {
		if got := combine(tc.results, "%d items").Status; got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestStatusNeedsApply(t *testing.T) {
	if StatusOK.NeedsApply() || StatusSkipped.NeedsApply() {
		t.Error("ok and skipped must not trigger Apply")
	}
	if !StatusMissing.NeedsApply() || !StatusDrifted.NeedsApply() {
		t.Error("missing and drifted must trigger Apply")
	}
}

func TestCheckOnlyStepCannotApply(t *testing.T) {
	if coreBinaryStep().Applicable() {
		t.Error("core-binary must stay check-only")
	}
	if err := coreBinaryStep().Apply(fixture(t)); err == nil {
		t.Error("applying a check-only step should fail loudly")
	}
}

func TestVaultScaffoldedWhenAbsent(t *testing.T) {
	env := fixture(t)

	if got := vaultStep().Check(env).Status; got != StatusMissing {
		t.Fatalf("на пустой машине вольт должен быть missing, получено %s", got)
	}

	if err := vaultStep().Apply(env); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := vaultStep().Check(env).Status; got != StatusOK {
		t.Errorf("после Apply статус %s", got)
	}
	for _, rel := range []string{".git", "dev", "research/10-wiki/index.md", "Personal Brand"} {
		if _, err := os.Stat(filepath.Join(env.Vault, rel)); err != nil {
			t.Errorf("не создано: %s", rel)
		}
	}
}

// A directory with notes in it but no git must never be touched: that is
// somebody's vault, and clobbering it would destroy the only thing here that
// cannot be regenerated.
func TestVaultWithContentIsLeftAlone(t *testing.T) {
	env := fixture(t)

	if err := os.MkdirAll(filepath.Join(env.Vault, "dev"), 0o755); err != nil {
		t.Fatalf("подготовка: %v", err)
	}
	note := filepath.Join(env.Vault, "dev", "note.md")
	if err := os.WriteFile(note, []byte("важная заметка\n"), 0o644); err != nil {
		t.Fatalf("подготовка: %v", err)
	}

	if got := vaultStep().Check(env).Status; got != StatusDrifted {
		t.Fatalf("статус %s, ожидается drifted", got)
	}
	if err := vaultStep().Apply(env); err == nil {
		t.Error("Apply должен отказаться трогать непустой каталог")
	}

	body, err := os.ReadFile(note)
	if err != nil || string(body) != "важная заметка\n" {
		t.Error("заметка не пережила Apply")
	}
}
