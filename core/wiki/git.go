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
// company's git_email when the company config declares one.
//
// Staging is explicit and scoped, so changes elsewhere in the vault cannot be
// swept in — they are reported and left alone. Refusing over them instead used
// to cascade: one drain that failed to commit left its files dirty, and every
// later drain of every other scope then refused too.
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
		fmt.Fprintf(out, "вне скоупа %q грязно, не трогаю:\n  %s\n",
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

// AutoMessage builds a commit message from what actually changed. Commits
// land after every edit, so the message has to say what happened in that step
// — "автокоммит" tells nobody anything a year later.
func AutoMessage(root string) string {
	var created, updated, deleted, other []string
	seen := map[string]bool{}
	add := func(list *[]string, name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			*list = append(*list, name)
		}
	}

	for _, line := range Dirty(root) {
		path := porcelainPath(line)
		if path == "" {
			continue
		}

		switch {
		case isDocPath(path, "topics"), isDocPath(path, "maps"), isDocPath(path, "sources"):
			name := strings.TrimSuffix(filepath.Base(path), ".md")
			// Porcelain status lives in the first two columns: "??"/"A" is a
			// new file, "D" in either column is a removal.
			switch {
			case strings.HasPrefix(line, "??"), strings.HasPrefix(line, "A"):
				add(&created, name)
			case strings.ContainsRune(line[:2], 'D'):
				add(&deleted, name)
			default:
				add(&updated, name)
			}
		default:
			add(&other, strings.SplitN(path, "/", 2)[0])
		}
	}

	sort.Strings(created)
	sort.Strings(updated)
	sort.Strings(deleted)
	sort.Strings(other)

	var parts []string
	if len(created) > 0 {
		parts = append(parts, "создан "+joinNames(created))
	}
	if len(updated) > 0 {
		parts = append(parts, "дополнен "+joinNames(updated))
	}
	if len(deleted) > 0 {
		parts = append(parts, "удалён "+joinNames(deleted))
	}
	if len(parts) > 0 {
		return "docs(vault): " + strings.Join(parts, ", ")
	}
	if len(other) > 0 {
		return "chore(vault): изменения — " + strings.Join(other, ", ")
	}
	return defaultAutocommitMessage
}

// isDocPath reports whether a vault path is a markdown page of one of the
// research sub-directories.
func isDocPath(path, dir string) bool {
	return strings.HasPrefix(path, "research/"+dir+"/") && strings.HasSuffix(path, ".md")
}

// joinNames keeps the subject readable when a step touched many files.
func joinNames(names []string) string {
	if len(names) > 3 {
		return fmt.Sprintf("%s и ещё %d", strings.Join(names[:3], ", "), len(names)-3)
	}
	return strings.Join(names, ", ")
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
// given message (empty falls back to the generic one), then pushes an archive
// copy. Obsidian Sync owns synchronization between devices.
//
// The whole vault is committed on purpose: an agent that just wrote a
// conspectus should leave nothing behind, and the vault used to accumulate
// uncommitted edits for months.
func Autocommit(ifDue bool, out io.Writer) error {
	return AutocommitMessage(ifDue, "", out)
}

// SyncOption tunes what Autocommit does after committing.
type SyncOption func(*syncConfig)

type syncConfig struct{ push bool }

// WithPush controls the archive upload. Commits happen after every
// edit and must stay fast; pushing is left to the end of a turn or session.
func WithPush(push bool) SyncOption {
	return func(c *syncConfig) { c.push = push }
}

// AutocommitMessage is Autocommit with an explicit commit message. Pushing is
// on unless WithPush(false) says otherwise.
func AutocommitMessage(ifDue bool, message string, out io.Writer, opts ...SyncOption) error {
	conf := syncConfig{push: true}
	for _, opt := range opts {
		opt(&conf)
	}
	return autocommit(ifDue, message, conf, out)
}

func autocommit(ifDue bool, message string, conf syncConfig, out io.Writer) error {
	root, err := VaultRoot()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil
	}

	// This commits whatever is dirty, so it must not run while a curator is
	// halfway through rewriting a page: the commit would capture a file in the
	// middle of being written. One lock, one rule — only one process mutates
	// the vault at a time.
	release, holder, err := AcquireDrainLock()
	if err != nil {
		return err
	}
	if release == nil {
		fmt.Fprintf(out, "wikipedik-autocommit: пропуск, вольт занят разбором (pid %d)\n", holder)
		return nil
	}
	defer release()

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

	// Skipped for per-step commits, where the network round-trip would be
	// felt on every edit.
	if !conf.push {
		return nil
	}
	return PushVault(root, out)
}

// PushVault pushes the vault to its archive remote without importing changes.
// Obsidian Sync owns synchronization; Git must never change vault contents.
// It is separate from committing because per-step commits stay local, and the push
// at the end of a turn has to happen even when there is nothing new to commit
// — otherwise those local commits never leave the machine.
//
// Failures are returned so scheduled archive jobs report a failed backup.
// Local files and commits are preserved, including when the remote diverges.
func PushVault(root string, out io.Writer) error {
	remotes, err := gitOut(root, "remote")
	if err != nil {
		return err
	}
	if remotes == "" {
		return nil
	}
	if _, err := gitOut(root, "push", "--quiet"); err != nil {
		return fmt.Errorf("архив WikiPedik не отправлен; файлы и коммиты сохранены локально. Проверь сеть, доступ и upstream; при расхождении истории проверь архив в отдельном клоне, не выполняй pull/rebase или force-push в рабочем вольте. Синхронизацией устройств управляет Obsidian Sync: %w", err)
	}
	fmt.Fprintln(out, "wikipedik-autocommit: архив отправлен (push-only)")
	return nil
}

// Unpushed counts local commits the remote does not have yet.
func Unpushed(root string) int {
	out, err := gitOut(root, "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return 0
	}
	count := 0
	_, _ = fmt.Sscanf(out, "%d", &count)
	return count
}
