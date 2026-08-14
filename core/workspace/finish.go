package workspace

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// nowStamp renders timestamps the way the metadata file always carried them.
func nowStamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// FinishOptions controls how a task branch leaves the machine.
type FinishOptions struct {
	// Base overrides the review target branch.
	Base string
	// Title overrides the PR/MR title (default: the task slug).
	Title string
	// ReadyWithoutReview marks the workspace ready when no review URL could
	// be produced — an explicit opt-in, never the default.
	ReadyWithoutReview bool
	// NoReview pushes the branch without opening a review — the low-level
	// fallback that used to be agent-task-push.sh.
	NoReview bool
	// AllowDirty skips the clean-tree gate. Only honoured with NoReview.
	AllowDirty bool
	// Out receives command output and progress; nil discards it.
	Out io.Writer
}

// integrationPushEnv unlocks pushing dev/main in NoReview mode — for the
// vault/dotfiles flows that legitimately live on an integration branch.
const integrationPushEnv = "AGENT_ALLOW_INTEGRATION_PUSH"

// Finish completes a task: verify, push the branch, open a draft PR/MR, and
// record the review state in the workspace metadata. It never merges.
func Finish(dir string, opts FinishOptions) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	repoRoot, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("не git-репозиторий: %s", dir)
	}

	metaPath := filepath.Join(repoRoot, MetaFile)
	hasMeta := false
	if _, err := os.Stat(metaPath); err == nil {
		hasMeta = true
		_ = ensureExcludes(repoRoot)
	}
	if !hasMeta && !opts.NoReview {
		return fmt.Errorf("finish работает только в управляемом worktree (%s);\n"+
			"задачи начинаются с ярлыков, например: agents synapse ticket-quality", MetaFile)
	}

	branch := currentBranch(repoRoot)
	if branch == "" {
		return fmt.Errorf("detached HEAD — нечего завершать")
	}
	if err := checkBranchClass(branch, opts, out); err != nil {
		return err
	}

	if !(opts.NoReview && opts.AllowDirty) {
		clean, err := isClean(repoRoot)
		if err != nil {
			return err
		}
		if !clean {
			_ = gitStream(repoRoot, out, "status", "--short")
			return fmt.Errorf("рабочее дерево грязное — закоммить завершённые изменения:\n" +
				"  hq commit \"type(scope): русское описание\" -- <явные пути>")
		}
	}

	if !opts.NoReview {
		if err := runMakeCheck(repoRoot, out); err != nil {
			return err
		}
	}

	if !gitOK(repoRoot, "remote", "get-url", "origin") {
		return fmt.Errorf("нет remote origin")
	}

	var w Workspace
	if hasMeta {
		if w, err = readMeta(metaPath); err != nil {
			return err
		}
	}

	fmt.Fprintln(out, "\n--- состояние ветки ---")
	_ = gitStream(repoRoot, out, "status", "--short", "--branch")
	fmt.Fprintln(out, "\n--- push ---")
	if err := gitStream(repoRoot, out, "push", "-u", "origin", "HEAD"); err != nil {
		return err
	}

	now := nowStamp()
	if hasMeta {
		w.State = StatePushed
		w.PushedAt = now
		w.PushedBranch = branch
		if err := writeMeta(w); err != nil {
			return err
		}
	}

	if opts.NoReview {
		fmt.Fprintf(out, "\n✓ ветка запушена: %s\n", branch)
		fmt.Fprintln(out, "Ревью не открывалось (--no-review). Открой его через hq finish, когда задача завершена.")
		return nil
	}

	base := resolveBase(repoRoot, w, opts.Base)
	w.ReviewBase = base
	if err := writeMeta(w); err != nil {
		return err
	}

	title := opts.Title
	if title == "" {
		title = w.Task
	}
	if title == "" {
		title = "agent task"
	}
	body := reviewBody(w, branch, base)

	reviewURL := openReview(repoRoot, branch, base, title, body, out)

	switch {
	case reviewURL != "":
		w.State = StateReview
		w.ReviewURL = reviewURL
		w.ReviewOpenedAt = nowStamp()
		if err := writeMeta(w); err != nil {
			return err
		}
		fmt.Fprintf(out, "\n✓ ревью открыто: %s\n", reviewURL)
		fmt.Fprintln(out, "Состояние: review. Уборка — после merge или ручного подтверждения.")
	case opts.ReadyWithoutReview:
		w.State = StateReady
		w.ReadyWithoutReviewAt = nowStamp()
		if err := writeMeta(w); err != nil {
			return err
		}
		fmt.Fprintln(out, "\n✓ ветка запушена без ревью; помечена ready по явному флагу.")
	default:
		fmt.Fprintf(out, "\n✓ ветка запушена: %s\n", branch)
		fmt.Fprintln(out, "Ревью не открылось автоматически. Открой вручную и обнови метаданные, либо повтори hq finish.")
	}
	return nil
}

func checkBranchClass(branch string, opts FinishOptions, out io.Writer) error {
	switch {
	case strings.HasPrefix(branch, "agent/"), strings.HasPrefix(branch, "feat/"),
		strings.HasPrefix(branch, "fix/"), strings.HasPrefix(branch, "docs/"),
		strings.HasPrefix(branch, "chore/"):
		return nil
	case branch == "main" || branch == "dev":
		if opts.NoReview && os.Getenv(integrationPushEnv) == "1" {
			return nil
		}
		if opts.NoReview {
			return fmt.Errorf("отказ пушить интеграционную ветку %q из задачи;\n"+
				"%s=1 — только для WikiPedik/dotfiles/ручных синков", branch, integrationPushEnv)
		}
		return fmt.Errorf("отказ завершать с интеграционной ветки %q", branch)
	default:
		fmt.Fprintf(out, "предупреждение: нестандартная ветка %q\n", branch)
		return nil
	}
}

// resolveBase picks the review target: explicit flag, then the workspace's
// recorded base, then origin/dev, origin/main, main.
func resolveBase(repoRoot string, w Workspace, override string) string {
	if override != "" {
		return override
	}
	switch {
	case strings.HasPrefix(w.BaseRef, "origin/"):
		return strings.TrimPrefix(w.BaseRef, "origin/")
	case w.BaseRef == "dev" || w.BaseRef == "main":
		return w.BaseRef
	}
	if gitOK(repoRoot, "show-ref", "--verify", "--quiet", "refs/remotes/origin/dev") {
		return "dev"
	}
	return "main"
}

func reviewBody(w Workspace, branch, base string) string {
	task := w.Task
	if task == "" {
		task = "unknown"
	}
	id := w.ID
	if id == "" {
		id = "unknown"
	}
	return fmt.Sprintf(`Automated agent task finish.

- Task: %s
- Worktree id: %s
- Branch: %s
- Base: %s

Review before merge. Do not squash unrelated changes.
`, task, id, branch, base)
}

// openReview opens (or finds) a draft PR/MR through the forge CLI matching
// the origin URL. An empty result is not an error — the caller reports it.
func openReview(repoRoot, branch, base, title, body string, out io.Writer) string {
	origin, err := git(repoRoot, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}

	switch {
	case hasCLI("gh") && strings.Contains(strings.ToLower(origin), "github.com"):
		if url := cliOutput(repoRoot, "gh", "pr", "view", branch, "--json", "url", "-q", ".url"); url != "" {
			return url
		}
		return cliOutput(repoRoot, "gh", "pr", "create", "--draft",
			"--base", base, "--head", branch, "--title", title, "--body", body)

	case hasCLI("glab") && (strings.Contains(strings.ToLower(origin), "gitlab") ||
		strings.Contains(strings.ToLower(origin), "git.")):
		if url := glabViewURL(repoRoot, branch); url != "" {
			return url
		}
		return cliOutput(repoRoot, "glab", "mr", "create", "--draft",
			"--target-branch", base, "--source-branch", branch, "--title", title, "--description", body)

	default:
		fmt.Fprintf(out, "предупреждение: нет поддерживаемого PR/MR CLI для origin %s\n", origin)
		return ""
	}
}

func hasCLI(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// cliOutput runs a forge command and returns trimmed stdout, or "" on error.
func cliOutput(dir string, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// glabViewURL extracts web_url from `glab mr view --output json` without
// trusting the full JSON shape.
func glabViewURL(dir, branch string) string {
	raw := cliOutput(dir, "glab", "mr", "view", branch, "--output", "json")
	_, after, found := strings.Cut(raw, `"web_url":"`)
	if !found {
		return ""
	}
	url, _, found := strings.Cut(after, `"`)
	if !found {
		return ""
	}
	return url
}
