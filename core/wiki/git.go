package wiki

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/world"
)

// gitOut runs git in the vault and returns trimmed stdout.
func gitOut(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// Dirty reports `git status --porcelain` lines of the vault.
//
// Porcelain encodes the status in the first two columns, and an unstaged
// change starts with a space (" M path"). Trimming that leading space shifts
// every path by one character, which silently mangled both the commit message
// and the "dirty outside scope" check — so this reads raw output.
func Dirty(root string) []string {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}

	var lines []string
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// porcelainPath extracts the path of one porcelain line; renames and copies
// report the destination.
func porcelainPath(line string) string {
	if len(line) < 4 {
		return ""
	}
	path := line[3:]
	if line[0] == 'R' || line[0] == 'C' {
		if _, after, found := strings.Cut(path, " -> "); found {
			path = after
		}
	}
	return path
}

// DirtyOutsideScope lists dirty vault files that a scope-atomic commit would
// not cover: everything except the scope directory, the product log and the
// auto-managed root project index.
func DirtyOutsideScope(root string, scope Scope) ([]string, error) {
	dir, err := scope.Path()
	if err != nil {
		return nil, err
	}
	memoryRoot, err := MemoryRoot()
	if err != nil {
		return nil, err
	}

	rel := strings.TrimPrefix(dir, root+"/")
	productLog, err := scope.ProductLog()
	if err != nil {
		return nil, err
	}
	productLogRel := strings.TrimPrefix(productLog, root+"/")
	rootIndexRel := filepath.Join(strings.TrimPrefix(memoryRoot, root+"/"), "20-projects", "index.md")

	var outside []string
	for _, line := range Dirty(root) {
		path := porcelainPath(line)
		switch {
		case path == "":
		case path == rel || strings.HasPrefix(path, rel+"/"):
		case productLog != "" && path == productLogRel:
		case path == rootIndexRel:
		default:
			outside = append(outside, line)
		}
	}
	return outside, nil
}

// Commit records the scope's memory changes as one commit, authored with the
// company's git_email when the company config declares one. Refuses when
// dirty files exist outside the scope.
func Commit(scope Scope, push bool, out io.Writer) error {
	root, err := VaultRoot()
	if err != nil {
		return err
	}

	if len(Dirty(root)) == 0 {
		fmt.Fprintln(out, "wiki commit: нечего коммитить")
		return nil
	}

	outside, err := DirtyOutsideScope(root, scope)
	if err != nil {
		return err
	}
	if len(outside) > 0 {
		return fmt.Errorf("отказ от wiki commit: грязные файлы вне скоупа %q:\n  %s",
			scope, strings.Join(outside, "\n  "))
	}

	dir, err := scope.Path()
	if err != nil {
		return err
	}
	memoryRoot, err := MemoryRoot()
	if err != nil {
		return err
	}

	if _, err := gitOut(root, "add", "-A", "--", strings.TrimPrefix(dir, root+"/")); err != nil {
		return err
	}
	// The root project index is auto-managed by bootstrap and safe to commit
	// with any scope: it holds only company links.
	rootIndex := filepath.Join(memoryRoot, "20-projects", "index.md")
	if _, err := os.Stat(rootIndex); err == nil {
		_, _ = gitOut(root, "add", "-A", "--", strings.TrimPrefix(rootIndex, root+"/"))
	}
	if productLog, _ := scope.ProductLog(); productLog != "" {
		_, _ = gitOut(root, "add", "-A", "--", strings.TrimPrefix(productLog, root+"/"))
	}

	message := fmt.Sprintf("docs(wiki): синхронизировать память %s", scope)
	cmd := exec.Command("git", "-C", root, "commit", "-m", message)
	if email := companyGitEmail(scope.Company); email != "" {
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_EMAIL="+email, "GIT_COMMITTER_EMAIL="+email)
	}
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	if push {
		if _, err := gitOut(root, "push"); err != nil {
			return err
		}
		fmt.Fprintln(out, "wiki commit: запушено")
		return nil
	}
	fmt.Fprintln(out, "Wiki commit создан. Пуш вручную:")
	fmt.Fprintf(out, "  git -C %s push\n", root)
	return nil
}

// companyGitEmail reads git_email from the company's workspace config.
func companyGitEmail(company string) string {
	root, err := world.Root()
	if err != nil {
		return ""
	}
	tree, err := world.Scan(root)
	if err != nil {
		return ""
	}
	for _, c := range tree.Companies() {
		if c.Slug == company {
			return c.Config.Get("git_email")
		}
	}
	return ""
}

// secretRe are the high-confidence patterns that abort an autocommit: the
// vault must never enshrine credentials.
var secretRe = regexp.MustCompile(
	`(BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9]{20,}|glpat-[A-Za-z0-9_-]{20,}|sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|xox[bap]-[A-Za-z0-9-]{10,})`)

// defaultAutocommitMessage is used when the caller has nothing better to say.
const defaultAutocommitMessage = "chore(vault): автокоммит несинхронизированных изменений"

// conventionalPrefixRe matches the platform's commit convention: an English
// Conventional Commit type/scope followed by a Russian description.
var conventionalPrefixRe = regexp.MustCompile(
	`^(feat|fix|refactor|build|ci|chore|docs|style|perf|test)(\([^)]*\))?:`)

// AutoMessage builds a commit message from what actually changed, for the
// safety net that runs when the agent forgot to commit itself. It names the
// touched topics, because "автокоммит" tells nobody anything a year later.
func AutoMessage(root string) string {
	var topics, zones []string
	seenTopic := map[string]bool{}
	seenZone := map[string]bool{}

	for _, line := range Dirty(root) {
		path := porcelainPath(line)
		switch {
		case strings.HasPrefix(path, "research/topics/") && strings.HasSuffix(path, ".md"):
			name := strings.TrimSuffix(filepath.Base(path), ".md")
			if !seenTopic[name] {
				seenTopic[name] = true
				topics = append(topics, name)
			}
		case path != "":
			zone := strings.SplitN(path, "/", 2)[0]
			if zone != "" && !seenZone[zone] {
				seenZone[zone] = true
				zones = append(zones, zone)
			}
		}
	}

	sort.Strings(topics)
	switch {
	case len(topics) > 3:
		return fmt.Sprintf("docs(vault): конспекты — %s и ещё %d",
			strings.Join(topics[:3], ", "), len(topics)-3)
	case len(topics) > 0:
		return "docs(vault): конспекты — " + strings.Join(topics, ", ")
	case len(zones) > 0:
		sort.Strings(zones)
		return "chore(vault): изменения — " + strings.Join(zones, ", ")
	default:
		return defaultAutocommitMessage
	}
}

// VaultCommitMessage normalises a short human description into the platform
// convention. A bare "конспект по матрицам" becomes
// "docs(vault): конспект по матрицам"; an already-prefixed message is kept.
func VaultCommitMessage(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}
	if conventionalPrefixRe.MatchString(description) {
		return description
	}
	return "docs(vault): " + description
}

// Autocommit sweeps everything dirty in the vault into one commit with the
// given message (empty falls back to the generic one), then best-effort
// pull --rebase + push.
//
// The whole vault is committed on purpose: an agent that just wrote a
// conspectus should leave nothing behind, and the vault used to accumulate
// uncommitted edits for months.
func Autocommit(ifDue bool, out io.Writer) error {
	return AutocommitMessage(ifDue, "", out)
}

// AutocommitMessage is Autocommit with an explicit commit message.
func AutocommitMessage(ifDue bool, message string, out io.Writer) error {
	root, err := VaultRoot()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil
	}

	stamp := os.Getenv("WIKIPEDIK_AUTOCOMMIT_STAMP")
	if stamp == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		stamp = filepath.Join(home, ".cache", "wikipedik-autocommit.stamp")
	}

	if ifDue {
		if info, err := os.Stat(stamp); err == nil && time.Since(info.ModTime()) < 24*time.Hour {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(stamp, nil, 0o644); err != nil {
		return err
	}

	if len(Dirty(root)) > 0 {
		// Stage first so the scan also covers brand-new files (plain
		// `diff HEAD` misses untracked content).
		if _, err := gitOut(root, "add", "-A"); err != nil {
			return err
		}
		diff, err := gitOut(root, "diff", "--cached")
		if err != nil {
			return err
		}
		if secretRe.MatchString(diff) {
			_, _ = gitOut(root, "reset", "--quiet", "HEAD")
			return fmt.Errorf("похоже на секрет в diff — автокоммит отменён, проверь вручную (git -C %s diff)", root)
		}

		if message == "" {
			message = defaultAutocommitMessage
		}
		if _, err := gitOut(root, "commit", "--quiet", "-m", message); err != nil {
			return err
		}
		stat, _ := gitOut(root, "show", "--stat", "--format=", "HEAD")
		lines := strings.Split(stat, "\n")
		fmt.Fprintf(out, "wikipedik-autocommit: закоммичено %s\n", strings.TrimSpace(lines[len(lines)-1]))
	}

	// Cross-machine sync, best-effort and guarded: any network or conflict
	// issue warns and never breaks the caller.
	remotes, _ := gitOut(root, "remote")
	if remotes == "" {
		return nil
	}
	if _, err := gitOut(root, "pull", "--rebase", "--quiet"); err != nil {
		_, _ = gitOut(root, "rebase", "--abort")
		fmt.Fprintln(out, "⚠ pull --rebase не прошёл (конфликт/сеть?) — синк отложен, разреши вручную")
		return nil
	}
	if _, err := gitOut(root, "push", "--quiet"); err != nil {
		fmt.Fprintln(out, "⚠ push не прошёл (сеть/доступ?) — изменения сохранены локально")
		return nil
	}
	fmt.Fprintln(out, "wikipedik-autocommit: синхронизировано (pull+push)")
	return nil
}
