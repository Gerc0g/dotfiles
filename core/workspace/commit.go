package workspace

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

// CommitOptions describes one logical commit.
type CommitOptions struct {
	Message string
	// Paths are staged explicitly, relative to the repository root. Broad
	// selectors (".", ":/", "-A") are rejected — that is the whole point.
	Paths []string
	// Verify runs `make verify` (or `make test`) before committing.
	Verify bool
	// Out receives command output and progress; nil discards it.
	Out io.Writer
}

// conventionalRe accepts the Conventional Commit prefixes the platform uses.
var conventionalRe = regexp.MustCompile(
	`^(feat|fix|refactor|build|ci|chore|docs|style|perf|test)(\([^)]*\))?:`)

// ErrNothingToCommit reports that the selected paths carry no changes. It is
// an outcome, not a failure: the caller decides whether that is fine.
var ErrNothingToCommit = fmt.Errorf("нет изменений в указанных путях")

// Commit stages exactly the given paths and commits one logical change. It
// never pushes. The git identity is checked against the company config when
// the repository lives inside the workspace root — including worktrees, which
// the bash predecessor silently skipped.
func Commit(root, dir string, opts CommitOptions) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	if !conventionalRe.MatchString(opts.Message) {
		return fmt.Errorf("сообщение должно начинаться с Conventional Commit префикса,\n" +
			"например: feat(storage): добавить репозиторий daily logs")
	}
	if len(opts.Paths) == 0 {
		return fmt.Errorf("нужны явные пути: hq commit \"<сообщение>\" -- <путь> [путь...]")
	}

	repoRoot, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("не git-репозиторий: %s", dir)
	}

	if currentBranch(repoRoot) == "" {
		return fmt.Errorf("detached HEAD — коммитить некуда")
	}

	if err := checkIdentity(root, repoRoot); err != nil {
		return err
	}

	// Reset only the index, not the working tree: paths staged by another
	// agent must not ride along.
	_, _ = git(repoRoot, "restore", "--staged", ":/")

	for _, path := range opts.Paths {
		switch path {
		case ".", "./", "-A", "--all", ":/":
			return fmt.Errorf("широкий путь %q запрещён — только явные файлы и каталоги", path)
		}
		if _, err := os.Stat(filepath.Join(repoRoot, path)); err != nil {
			if !gitOK(repoRoot, "ls-files", "--error-unmatch", "--", path) {
				return fmt.Errorf("путь не существует и не отслеживается: %s", path)
			}
		}
	}

	pathArgs := append([]string{"--"}, opts.Paths...)
	unstaged := !gitOK(repoRoot, append([]string{"diff", "--quiet"}, pathArgs...)...)
	staged := !gitOK(repoRoot, append([]string{"diff", "--cached", "--quiet"}, pathArgs...)...)
	if !unstaged && !staged {
		untracked, _ := git(repoRoot, append([]string{"ls-files", "--others", "--exclude-standard"}, pathArgs...)...)
		if untracked == "" {
			return ErrNothingToCommit
		}
	}

	if opts.Verify {
		if err := runMakeCheck(repoRoot, out); err != nil {
			return err
		}
	}

	if err := gitStream(repoRoot, out, append([]string{"add"}, pathArgs...)...); err != nil {
		return err
	}
	if gitOK(repoRoot, "diff", "--cached", "--quiet") {
		return ErrNothingToCommit
	}

	fmt.Fprintln(out, "\n--- staged diffstat ---")
	_ = gitStream(repoRoot, out, "diff", "--cached", "--stat")
	fmt.Fprintln(out, "\n--- committing ---")
	if err := gitStream(repoRoot, out, append([]string{"commit", "-m", opts.Message}, pathArgs...)...); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n✓ закоммичено локально: %s\n", opts.Message)
	return nil
}

// checkIdentity compares git user.email with the company's git_email. Outside
// the workspace root there is no company to check against, so it passes.
func checkIdentity(root, repoRoot string) error {
	ctx, err := Locate(root, repoRoot)
	if err != nil || ctx.Company == "" {
		return nil
	}

	tree, err := world.Scan(root)
	if err != nil {
		return nil
	}

	expected := ""
	for _, c := range tree.Companies() {
		if c.Slug == ctx.Company {
			expected = c.Config.Get("git_email")
		}
	}
	if expected == "" {
		return nil
	}

	current, _ := git(repoRoot, "config", "--get", "user.email")
	if current != expected {
		return fmt.Errorf("git user.email не совпадает с компанией %s:\n"+
			"  ожидается: %s\n  сейчас:    %s\nфикс: git config user.email '%s'",
			ctx.Company, expected, orUnset(current), expected)
	}
	return nil
}

func orUnset(s string) string {
	if s == "" {
		return "<не задан>"
	}
	return s
}

// runMakeCheck runs the repository's own gate: `make verify` when the target
// exists, `make test` otherwise. No Makefile — no gate.
func runMakeCheck(repoRoot string, out io.Writer) error {
	target := makeTarget(repoRoot)
	if target == "" {
		return nil
	}

	cmd := exec.Command("make", target)
	cmd.Dir = repoRoot
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("make %s: %w", target, err)
	}
	return nil
}

var makeVerifyRe = regexp.MustCompile(`(?m)^verify:`)
var makeTestRe = regexp.MustCompile(`(?m)^test:`)

func makeTarget(repoRoot string) string {
	data, err := os.ReadFile(filepath.Join(repoRoot, "Makefile"))
	if err != nil {
		return ""
	}
	switch {
	case makeVerifyRe.Match(data):
		return "verify"
	case makeTestRe.Match(data):
		return "test"
	}
	return ""
}

// gitStream runs git with output wired to the caller's writer — for the
// commands whose output is the user's feedback (status, diffstat, push).
func gitStream(dir string, out io.Writer, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
