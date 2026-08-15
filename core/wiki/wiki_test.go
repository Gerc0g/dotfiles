package wiki

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseScope(t *testing.T) {
	scope, err := ParseScope("acme/shop/backend")
	if err != nil {
		t.Fatal(err)
	}
	if scope.Company != "acme" || scope.Product != "shop" || scope.Repo != "backend" {
		t.Errorf("scope: %+v", scope)
	}
	if scope.String() != "acme/shop/backend" {
		t.Errorf("string: %s", scope)
	}

	if _, err := ParseScope(""); err == nil {
		t.Error("empty scope must fail")
	}
	if _, err := ParseScope("a/b/c/d"); err == nil {
		t.Error("too deep scope must fail")
	}
}

func TestPorcelainPath(t *testing.T) {
	cases := map[string]string{
		" M dev/20-projects/acme/log.md": "dev/20-projects/acme/log.md",
		"?? dev/new.md":                  "dev/new.md",
		"R  dev/old.md -> dev/new.md":    "dev/new.md",
		"C  dev/a.md -> dev/b.md":        "dev/b.md",
	}
	for line, want := range cases {
		if got := porcelainPath(line); got != want {
			t.Errorf("porcelainPath(%q) = %q, want %q", line, got, want)
		}
	}
}

// Dirty must preserve the leading status columns: trimming them shifts every
// path by one character, which once produced a commit named "изменения —
// esearch" and could mis-scope the dirty-outside-scope guard.
func TestDirtyKeepsLeadingSpace(t *testing.T) {
	vault := vaultEnv(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", vault}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")

	if err := os.MkdirAll(filepath.Join(vault, "research"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "research", "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "research/a.md")
	run("commit", "-qm", "init")
	if err := os.WriteFile(filepath.Join(vault, "research", "a.md"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	lines := Dirty(vault)
	if len(lines) != 1 {
		t.Fatalf("want 1 dirty line, got %v", lines)
	}
	if got := porcelainPath(lines[0]); got != "research/a.md" {
		t.Errorf("path mangled: %q (line %q)", got, lines[0])
	}
	if msg := AutoMessage(vault); !strings.Contains(msg, "research") {
		t.Errorf("commit message lost the zone name: %q", msg)
	}
}

// vaultEnv points the package at a temp vault for the duration of a test.
func vaultEnv(t *testing.T) string {
	t.Helper()
	vault := t.TempDir()
	t.Setenv(RootEnv, vault)
	return vault
}

func TestCountStatusAndStamps(t *testing.T) {
	vault := vaultEnv(t)
	repoMem := filepath.Join(vault, "dev", "20-projects", "acme", "shop", "repos", "backend")
	if err := os.MkdirAll(repoMem, 0o755); err != nil {
		t.Fatal(err)
	}

	inbox := "# inbox\n\nStatus: candidate\n\nStatus: drained\n\nStatus: candidate\n"
	if err := os.WriteFile(filepath.Join(repoMem, "_inbox.md"), []byte(inbox), 0o644); err != nil {
		t.Fatal(err)
	}
	hot := "# hot\n<!-- last refreshed: 2026-08-01 10:00 -->\n"
	if err := os.WriteFile(filepath.Join(repoMem, "hot.md"), []byte(hot), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := CountStatus(repoMem, "candidate"); got != 2 {
		t.Errorf("candidate count: %d", got)
	}
	if got := CountStatus(repoMem, "drained"); got != 1 {
		t.Errorf("drained count: %d", got)
	}
	if stamps := HotStamps(repoMem); len(stamps) != 1 || !strings.Contains(stamps[0], "last refreshed") {
		t.Errorf("stamps: %v", stamps)
	}
}

func TestBuildHot(t *testing.T) {
	vault := vaultEnv(t)
	repoMem := filepath.Join(vault, "dev", "20-projects", "acme", "shop", "repos", "backend")
	if err := os.MkdirAll(filepath.Join(repoMem, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}

	lessons := "# lessons\n\n## Первый урок\n\ntext\n\n## Второй урок\n\ntext\n"
	if err := os.WriteFile(filepath.Join(repoMem, "lessons.md"), []byte(lessons), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := "---\npaths:\n  - src/**\n---\n\n# Правило раз\n"
	if err := os.WriteFile(filepath.Join(repoMem, "rules", "one.md"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}
	noPaths := "---\ndescription: x\n---\n\n# Не правило\n"
	if err := os.WriteFile(filepath.Join(repoMem, "rules", "two.md"), []byte(noPaths), 0o644); err != nil {
		t.Fatal(err)
	}

	hot := BuildHot(repoMem, "backend")
	if !strings.Contains(hot, "# backend hot context") {
		t.Error("missing title")
	}
	// Newest lesson first.
	first := strings.Index(hot, "Второй урок")
	second := strings.Index(hot, "Первый урок")
	if first == -1 || second == -1 || first > second {
		t.Errorf("lesson order wrong:\n%s", hot)
	}
	if !strings.Contains(hot, "[rules] Правило раз") {
		t.Errorf("binding rule missing:\n%s", hot)
	}
	if strings.Contains(hot, "Не правило") {
		t.Error("rule without paths: must be excluded")
	}
	if !strings.Contains(hot, "lessons.md#второй-урок") {
		t.Errorf("cyrillic slug anchor missing:\n%s", hot)
	}
}

func TestBootstrapSkeletonIdempotent(t *testing.T) {
	vault := vaultEnv(t)
	workRoot := t.TempDir()
	t.Setenv("PROKECTFILES_ROOT", workRoot)

	// One repo checkout in the world.
	repoPath := filepath.Join(workRoot, "acme", "shop", "backend")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := BootstrapRoot(); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapCompany("acme", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapProduct("acme", "shop", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapRepo("acme", "shop", "backend", io.Discard); err != nil {
		t.Fatal(err)
	}

	projects := filepath.Join(vault, "dev", "20-projects")

	// Index registrations.
	rootIndex, _ := os.ReadFile(filepath.Join(projects, "index.md"))
	if !strings.Contains(string(rootIndex), "- acme: [[acme/acme]]") {
		t.Errorf("root index registration missing:\n%s", rootIndex)
	}
	coIndex, _ := os.ReadFile(filepath.Join(projects, "acme", "index.md"))
	if !strings.Contains(string(coIndex), "- shop: [[shop/shop]]") {
		t.Errorf("company index registration missing:\n%s", coIndex)
	}
	prodIndex, _ := os.ReadFile(filepath.Join(projects, "acme", "shop", "index.md"))
	if !strings.Contains(string(prodIndex), "- backend: [[repos/backend/backend]]") {
		t.Errorf("product index registration missing:\n%s", prodIndex)
	}

	// The entry lands under the section heading, not at the end of file.
	lines := strings.Split(string(prodIndex), "\n")
	for i, line := range lines {
		if line == "## Repos" {
			if lines[i+1] != "- backend: [[repos/backend/backend]]" {
				t.Errorf("repo entry must follow the heading, got %q", lines[i+1])
			}
		}
	}

	// Symlinks + excludes.
	dest, err := os.Readlink(filepath.Join(repoPath, "docs", "company-knowledge"))
	if err != nil || dest != filepath.Join(projects, "acme") {
		t.Errorf("company link: %s, %v", dest, err)
	}
	exclude, _ := os.ReadFile(filepath.Join(repoPath, ".git", "info", "exclude"))
	if !strings.Contains(string(exclude), "docs/knowledge") {
		t.Errorf("exclude missing:\n%s", exclude)
	}

	// A user note in an ensured file must survive a re-run.
	custom := filepath.Join(projects, "acme", "shop", "repos", "backend", "lessons.md")
	if err := os.WriteFile(custom, []byte("# custom content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapRepo("acme", "shop", "backend", io.Discard); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(custom)
	if string(after) != "# custom content\n" {
		t.Error("ensureFile must not overwrite existing pages")
	}

	// Re-registration must not duplicate the index entry.
	prodIndex2, _ := os.ReadFile(filepath.Join(projects, "acme", "shop", "index.md"))
	if strings.Count(string(prodIndex2), "- backend:") != 1 {
		t.Errorf("duplicate index entry:\n%s", prodIndex2)
	}
}
