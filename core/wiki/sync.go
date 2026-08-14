package wiki

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Curator instructions, ported verbatim: the LLM half of the memory cycle.
const (
	drainInstruction = "Drain worker captures into curated WikiPedik pages. Preserve company boundaries. " +
		"For each candidate, propose the target and ask before applying. " +
		"Update repo index, product log, and hot.md when accepted."
	statusInstruction = "Report inbox counts, curated pages, synthesis candidates, stale health files, " +
		"recent log entries, and recommended next actions. Do not modify files."
	synthesizeInstruction = "Review synthesis candidates and repeated repo lessons/gotchas. " +
		"Propose cross-repo product patterns before writing product shared pages."
)

// Preflight prints the sync context: scope, inbox counters, hot stamps and
// vault git state.
func Preflight(scope Scope, out io.Writer) error {
	dir, err := scope.Path()
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("wiki scope не найден: %s\n  ожидался: %s\n  запусти: hq wiki bootstrap %s",
			scope, dir, strings.ReplaceAll(scope.String(), "/", " "))
	}

	root, err := VaultRoot()
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "─── Wiki sync preflight ───")
	fmt.Fprintf(out, "Scope: %s\n", scope)
	fmt.Fprintf(out, "Path:  %s\n", shortHome(dir))
	fmt.Fprintf(out, "Pending inbox: %d candidate\n", CountStatus(dir, "candidate"))
	fmt.Fprintf(out, "Drained inbox:  %d drained\n", CountStatus(dir, "drained"))

	if stamps := HotStamps(dir); len(stamps) > 0 {
		fmt.Fprintln(out, "Hot cache:")
		for _, stamp := range stamps {
			fmt.Fprintf(out, "  %s\n", stamp)
		}
	} else {
		fmt.Fprintln(out, "Hot cache: none")
	}

	if dirty := Dirty(root); len(dirty) > 0 {
		fmt.Fprintln(out, "Git status before: dirty")
		for i, line := range dirty {
			if i >= 20 {
				break
			}
			fmt.Fprintf(out, "  %s\n", line)
		}
	} else {
		fmt.Fprintln(out, "Git status before: clean")
	}
	fmt.Fprintln(out)
	return nil
}

// Postflight prints the counters and vault changes after a curator run.
func Postflight(scope Scope, out io.Writer) {
	dir, err := scope.Path()
	if err != nil {
		return
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "─── Wiki sync summary ───")
	fmt.Fprintf(out, "Scope: %s\n", scope)
	fmt.Fprintf(out, "Pending inbox: %d candidate\n", CountStatus(dir, "candidate"))
	fmt.Fprintf(out, "Drained inbox:  %d drained\n", CountStatus(dir, "drained"))
	fmt.Fprintf(out, "Rejected inbox: %d rejected\n", CountStatus(dir, "rejected"))

	root, err := VaultRoot()
	if err != nil {
		return
	}
	if changed := Dirty(root); len(changed) > 0 {
		fmt.Fprintln(out, "\nChanged files:")
		for i, line := range changed {
			if i >= 80 {
				break
			}
			fmt.Fprintf(out, "  %s\n", line)
		}
	} else {
		fmt.Fprintln(out, "\nChanged files: none")
	}
}

// runCurator launches the codex curator with a skill over the memory root.
// Interactive terminals get the full TUI; non-interactive callers may only
// run read-only skills through `codex exec`.
func runCurator(skill string, scope Scope, instruction string, readonlyOK bool) error {
	memoryRoot, err := MemoryRoot()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf("Use skill %s. Scope: %s. %s", skill, scope, instruction)
	interactive := stdinIsTerminal()

	var cmd *exec.Cmd
	switch {
	case interactive:
		cmd = exec.Command("codex", "--no-alt-screen", prompt)
	case readonlyOK:
		cmd = exec.Command("codex", "exec", prompt)
	default:
		return fmt.Errorf("wiki %s требует интерактивный терминал для подтверждений — запусти из своей оболочки", skill)
	}

	cmd.Dir = memoryRoot
	cmd.Env = append(os.Environ(), "CODEX_HOME="+filepath.Join(home, ".codex"))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// SyncOptions controls one wiki sync run.
type SyncOptions struct {
	Commit bool
	Push   bool
}

// Sync drives the drain cycle: preflight → curator → postflight, then the
// optional scope-atomic commit. Auto-commit refuses when the vault already
// had dirty files outside the scope before the curator ran.
func Sync(scope Scope, opts SyncOptions, out io.Writer) error {
	root, err := VaultRoot()
	if err != nil {
		return err
	}

	dirtyBefore := Dirty(root)
	outsideBefore, err := DirtyOutsideScope(root, scope)
	if err != nil {
		return err
	}

	if err := Preflight(scope, out); err != nil {
		return err
	}

	curatorErr := runCurator("inbox-drain", scope, drainInstruction, false)
	Postflight(scope, out)
	if curatorErr != nil {
		return fmt.Errorf("wiki sync: куратор завершился с ошибкой: %w", curatorErr)
	}

	if !opts.Commit {
		return nil
	}
	if len(outsideBefore) > 0 {
		return fmt.Errorf("отказ от автокоммита: до синка в вольте были грязные файлы вне скоупа.\nРазбери вручную: git -C %s status", root)
	}
	if len(dirtyBefore) > 0 {
		fmt.Fprintln(out, "\nЗаметка автокоммита: файлы, грязные до синка, были внутри скоупа и считаются входом inbox.")
	}
	return Commit(scope, opts.Push, out)
}

// Status runs the read-only curator report around the same pre/postflight.
func Status(scope Scope, out io.Writer) error {
	if err := Preflight(scope, out); err != nil {
		return err
	}
	curatorErr := runCurator("wiki-status", scope, statusInstruction, true)
	Postflight(scope, out)
	return curatorErr
}

// Synthesize launches the cross-repo pattern curator.
func Synthesize(scope Scope) error {
	return runCurator("wiki-synthesize", scope, synthesizeInstruction, false)
}

func shortHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
