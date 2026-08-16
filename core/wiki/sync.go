package wiki

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// autonomousRules turn the interactive drain into one that can run in the
// background. The judgement stays with the curator — dedup, retirement,
// derivability — only the per-entry confirmation goes away, because a human
// answering 138 prompts in a row is exactly why the inbox stopped being
// drained at all.
const autonomousRules = `АВТОНОМНЫЙ РЕЖИМ. Подтверждений не жди — их некому дать.

- Применяй только те решения, в которых уверен: сигнал однозначно ложится в
  целевой файл, дубля нет, запись не выводится из кода репозитория.
- Всё спорное — противоречие с существующей записью, неясный сигнал, выбор
  между двумя файлами, подозрение на устаревание — НЕ применяй: оставь
  Status: candidate и назови причину в отчёте.
- Ничего не удаляй и не переписывай в curated-страницах: только добавляй.
  Устаревшее помечай, но не стирай.
- Обязательно доведи до конца фазу hot.md для каждого затронутого репо.
- Если кандидатов много — иди по ним подряд, а не выборочно: пропущенная
  запись вернётся следующим прогоном и будет стоить ещё один запуск.
- В конце: сколько разобрано, сколько оставлено и почему, что требует
  человека.`

// Curator instructions, ported verbatim: the LLM half of the memory cycle.
const (
	drainInstruction = "Drain worker captures into curated WikiPedik pages. Preserve company boundaries. " +
		"For each candidate, propose the target and ask before applying. " +
		"Update repo index, product log, and hot.md when accepted."
	autonomousDrainInstruction = "Drain worker captures into curated WikiPedik pages. Preserve company boundaries. " +
		"Update repo index, product log, and hot.md for every repo you touch."
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

// curatorMode is how a curator run interacts with the user.
type curatorMode int

const (
	// modeInteractive requires a terminal: the curator asks about every entry.
	modeInteractive curatorMode = iota
	// modeReadonly may run headless because it changes nothing.
	modeReadonly
	// modeAutonomous runs headless and writes: the curator applies what it is
	// sure about and leaves anything doubtful as a candidate. Review happens
	// afterwards over the diff instead of per-entry confirmation.
	modeAutonomous
)

// runCurator launches the codex curator with a skill over the memory root.
func runCurator(skill string, scope Scope, instruction string, mode curatorMode) error {
	memoryRoot, err := MemoryRoot()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf("Use skill %s. Scope: %s. %s", skill, scope, instruction)
	if mode == modeAutonomous {
		prompt += "\n\n" + autonomousRules
	}

	var cmd *exec.Cmd
	switch {
	case mode != modeAutonomous && stdinIsTerminal():
		cmd = exec.Command("codex", "--no-alt-screen", prompt)
	case mode == modeInteractive:
		return fmt.Errorf("wiki %s требует интерактивный терминал для подтверждений — запусти из своей оболочки", skill)
	default:
		cmd = exec.Command("codex", "exec", prompt)
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
	// Autonomous runs the curator headless: it applies what it is sure about
	// and leaves the rest as candidates, instead of asking per entry. This is
	// what makes a background drain possible at all.
	Autonomous bool
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

	mode := modeInteractive
	instruction := drainInstruction
	if opts.Autonomous {
		mode = modeAutonomous
		instruction = autonomousDrainInstruction
	}

	curatorErr := runCurator("inbox-drain", scope, instruction, mode)
	Postflight(scope, out)
	if curatorErr != nil {
		return fmt.Errorf("wiki sync: куратор завершился с ошибкой: %w", curatorErr)
	}

	// The curator has skipped its hot.md phase before, which is how a stale
	// index got served as memory for weeks. Rebuild deterministically after
	// every drain instead of trusting the phase.
	if err := RefreshHot(scope, false, out); err != nil {
		fmt.Fprintf(out, "⚠ hot.md не пересобран: %v\n", err)
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
	curatorErr := runCurator("wiki-status", scope, statusInstruction, modeReadonly)
	Postflight(scope, out)
	return curatorErr
}

// Synthesize launches the cross-repo pattern curator.
func Synthesize(scope Scope) error {
	return runCurator("wiki-synthesize", scope, synthesizeInstruction, modeInteractive)
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
