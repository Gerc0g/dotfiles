package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeContextFixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readContextFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func seedParentContext(t *testing.T, repo string) {
	t.Helper()
	writeContextFixture(t, filepath.Join(repo, "..", "..", "AGENTS.md"), "company rules\n")
	writeContextFixture(t, filepath.Join(repo, "..", "AGENTS.md"), "product rules\n")
	writeContextFixture(t, filepath.Join(repo, "..", "docs", "ARCHITECTURE.md"), "canonical architecture\n")
}

func TestStartPreservesTrackedContextAndReferencesLiveParents(t *testing.T) {
	m, repo := newFixture(t)
	seedParentContext(t, repo)
	const instructions = "read ../AGENTS.md, ../../AGENTS.md and ../docs/ARCHITECTURE.md\n"
	writeContextFixture(t, filepath.Join(repo, "AGENTS.md"), instructions)
	if _, err := git(repo, "add", "AGENTS.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := git(repo, "commit", "-qm", "add repo context"); err != nil {
		t.Fatal(err)
	}
	// A local edit in main must not leak into the branch's tracked document.
	writeContextFixture(t, filepath.Join(repo, "AGENTS.md"), "uncommitted main instructions\n")
	w, err := m.Start("acme", "product", "repo", "context")
	if err != nil {
		t.Fatal(err)
	}
	if got := readContextFixture(t, filepath.Join(w.Path, "AGENTS.md")); got != instructions {
		t.Fatalf("tracked worktree context replaced: %q", got)
	}
	for _, parent := range []struct{ relative, source string }{
		{"../AGENTS.md", filepath.Join(repo, "..", "AGENTS.md")},
		{"../../AGENTS.md", filepath.Join(repo, "..", "..", "AGENTS.md")},
	} {
		reference := readContextFixture(t, filepath.Join(w.Path, parent.relative))
		source, err := filepath.EvalSymlinks(parent.source)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(reference, "(<"+source+">)") || !strings.Contains(reference, "hq-worktree-context v1") {
			t.Fatalf("parent reference does not route to canonical file: %s", reference)
		}
		writeContextFixture(t, source, "updated live parent\n")
		if readContextFixture(t, source) != "updated live parent\n" {
			t.Fatal("canonical parent unavailable")
		}
		if after := readContextFixture(t, filepath.Join(w.Path, parent.relative)); after != reference {
			t.Fatal("parent change should not require regenerating its reference")
		}
	}
	architecture := filepath.Join(w.Path, "..", "docs", "ARCHITECTURE.md")
	if got := readContextFixture(t, architecture); got != "canonical architecture\n" {
		t.Fatal(got)
	}
	writeContextFixture(t, filepath.Join(repo, "..", "docs", "ARCHITECTURE.md"), "updated architecture\n")
	if got := readContextFixture(t, architecture); got != "updated architecture\n" {
		t.Fatal(got)
	}
	if currentBranch(repo) != "main" {
		t.Fatal("main checkout branch changed")
	}
	if clean, err := isClean(w.Path); err != nil || !clean {
		t.Fatalf("tracked checkout is dirty: %v", err)
	}
}

func TestStartSnapshotsUntrackedContextWithoutHidingOrSharingEdits(t *testing.T) {
	m, repo := newFixture(t)
	seedParentContext(t, repo)
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		writeContextFixture(t, filepath.Join(repo, name), "main "+name+"\n")
	}
	w, err := m.Start("acme", "product", "repo", "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		path := filepath.Join(w.Path, name)
		if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("not an isolated snapshot: %s (%v)", path, err)
		}
		if got := readContextFixture(t, path); got != "main "+name+"\n" {
			t.Fatal(got)
		}
		writeContextFixture(t, path, "branch edits\n")
		if got := readContextFixture(t, filepath.Join(repo, name)); got != "main "+name+"\n" {
			t.Fatal("branch edit mutated main")
		}
		if gitOK(w.Path, "check-ignore", "--quiet", "--", name) {
			t.Fatalf("user context hidden: %s", name)
		}
	}
	writeContextFixture(t, filepath.Join(repo, "AGENTS.md"), "later main context\n")
	if _, err := m.RepairContext(w); err != nil {
		t.Fatal(err)
	}
	if got := readContextFixture(t, filepath.Join(w.Path, "AGENTS.md")); got != "branch edits\n" {
		t.Fatal("repair overwrote local context")
	}
	status, err := git(w.Path, "status", "--short")
	if err != nil || !strings.Contains(status, "?? AGENTS.md") || !strings.Contains(status, "?? CLAUDE.md") {
		t.Fatalf("snapshots must remain visible to Git: %q (%v)", status, err)
	}
}

func TestContextRepairExistingWorktreeAndTrackedDeletion(t *testing.T) {
	m, repo := newFixture(t)
	w, err := m.Start("acme", "product", "repo", "old")
	if err != nil {
		t.Fatal(err)
	}
	seedParentContext(t, repo)
	writeContextFixture(t, filepath.Join(repo, "AGENTS.md"), "new main context\n")
	issues, err := m.ContextIssues(w)
	if err != nil || len(issues) < 3 {
		t.Fatalf("missing references not visible: %+v (%v)", issues, err)
	}
	if _, err := os.Lstat(filepath.Join(w.Path, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("check changed files")
	}
	if _, err := m.RepairContext(w); err != nil {
		t.Fatal(err)
	}
	if got := readContextFixture(t, filepath.Join(w.Path, "AGENTS.md")); got != "new main context\n" {
		t.Fatal(got)
	}
	if _, err := git(w.Path, "add", "AGENTS.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := git(w.Path, "commit", "-qm", "keep branch context"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(w.Path, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	issues, err = m.RepairContext(w)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(w.Path, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("repair undid tracked deletion")
	}
	if !strings.Contains(ContextSummary(issues), "tracked") {
		t.Fatal("missing tracked context not reported")
	}
	if _, err := git(w.Path, "add", "-u", "--", "AGENTS.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.RepairContext(w); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(w.Path, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("repair undid a staged tracked deletion")
	}
}

func TestStartWarnsOnMissingContextAndPreservesParentCollision(t *testing.T) {
	m, repo := newFixture(t)
	var warnings bytes.Buffer
	m.warnings = &warnings
	w, err := m.Start("acme", "product", "repo", "missing")
	if err != nil {
		t.Fatalf("incomplete context must not block startup: %v", err)
	}
	if !strings.Contains(warnings.String(), "канонического контекста") || !strings.Contains(warnings.String(), "WikiPedik") {
		t.Fatalf("missing context was silent: %q", warnings.String())
	}
	seedParentContext(t, repo)
	parent := filepath.Join(w.Path, "..", "AGENTS.md")
	writeContextFixture(t, parent, "custom parent instructions\n")
	if _, err := m.RepairContext(w); err == nil {
		t.Fatal("parent collision silently accepted")
	}
	if got := readContextFixture(t, parent); got != "custom parent instructions\n" {
		t.Fatal("custom parent overwritten")
	}
	w2, err := m.Start("acme", "product", "repo", "collision")
	if err == nil || w2.Path == "" {
		t.Fatalf("technical failure must identify created worktree: %+v (%v)", w2, err)
	}
	if _, err := m.Find("acme", "product", "repo", w2.ID); err != nil {
		t.Fatal("failed setup cannot be repaired by ID")
	}
}

func TestKnowledgeRelinkResolvesRelativeTargetsAndProtectsFiles(t *testing.T) {
	m, repo := newFixture(t)
	vault := filepath.Join(m.root, "vault", "repo-memory")
	writeContextFixture(t, filepath.Join(vault, "hot.md"), "memory\n")
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(filepath.Join(repo, "docs"), vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(relative, filepath.Join(repo, "docs", "knowledge")); err != nil {
		t.Fatal(err)
	}
	w, err := m.Start("acme", "product", "repo", "memory")
	if err != nil {
		t.Fatal(err)
	}
	if got := readContextFixture(t, filepath.Join(w.Path, "docs", "knowledge", "hot.md")); got != "memory\n" {
		t.Fatal(got)
	}
	link := filepath.Join(w.Path, "docs", "knowledge")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	writeContextFixture(t, link, "user file\n")
	if issues, err := m.ContextIssues(w); err == nil || !strings.Contains(ContextSummary(issues), "путь контекста занят") {
		t.Fatalf("check did not expose memory collision: %+v (%v)", issues, err)
	}
	if _, err := m.RepairContext(w); err == nil {
		t.Fatal("memory collision ignored")
	}
	if got := readContextFixture(t, link); got != "user file\n" {
		t.Fatal("memory repair overwrote user file")
	}
}
