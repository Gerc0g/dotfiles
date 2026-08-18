package routine

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Decision explains whether a run may proceed, so a skip is never mysterious.
type Decision struct {
	Run    bool
	Reason string
}

// Allowed applies the guards: disabled, too soon, or out of today's budget.
//
// Guards are checked here rather than inside each routine because the point of
// the entity is that no background job gets to decide on its own how much it
// may spend.
func Allowed(home string, r Routine, force bool, now time.Time) Decision {
	status := LoadState(home)[r.Name]

	if status.Disabled {
		return Decision{Reason: "выключена"}
	}
	if force {
		return Decision{Run: true, Reason: "принудительно"}
	}
	if r.MinInterval > 0 && !status.Last.IsZero() {
		if since := now.Sub(status.Last); since < r.MinInterval {
			return Decision{Reason: fmt.Sprintf("прошлый запуск %s назад, минимум %s",
				since.Round(time.Minute), humanDuration(r.MinInterval))}
		}
	}
	if ceiling := r.Ceiling(); ceiling > 0 {
		if done := status.RunsToday(now); done >= ceiling {
			return Decision{Reason: fmt.Sprintf("за сутки уже %d из %d", done, ceiling)}
		}
	}
	return Decision{Run: true, Reason: "по расписанию"}
}

// Run executes a routine and records what happened.
//
// It records the attempt before the work starts: a run that dies halfway still
// happened, and a record written only on success reports a broken job as one
// that was never scheduled.
func Run(home string, r Routine, force bool, out io.Writer) error {
	now := time.Now()
	decision := Allowed(home, r, force, now)
	if !decision.Run {
		Log(r.Name, "пропуск: %s", decision.Reason)
		fmt.Fprintf(out, "%s: пропуск (%s)\n", r.Name, decision.Reason)
		return nil
	}

	status := LoadState(home)[r.Name]
	status.Last = now
	status.Runs++
	status.RunsDay = status.RunsToday(now) + 1
	status.DayStamp = now.Format("2006-01-02")
	status.Outcome = OutcomeSkipped
	status.Detail = "выполняется"
	_ = SaveStatus(home, r.Name, status)

	spends := ""
	if r.Kind.SpendsTokens() {
		spends = fmt.Sprintf(", тратит токены (%d/%d за сутки)", status.RunsDay, r.Ceiling())
	}
	Log(r.Name, "старт: %s%s", decision.Reason, spends)

	err := execute(r, out)

	status.DurationMS = time.Since(now).Milliseconds()
	if err != nil {
		status.Outcome = OutcomeFailed
		status.Detail = truncate(err.Error(), 200)
		Log(r.Name, "ошибка за %s — %v", time.Since(now).Round(time.Second), err)
	} else {
		status.Outcome = OutcomeOK
		status.Detail = ""
		Log(r.Name, "готово за %s", time.Since(now).Round(time.Second))
	}
	_ = SaveStatus(home, r.Name, status)
	return err
}

// execute runs the routine's work: a command, or a prompt handed to an agent.
func execute(r Routine, out io.Writer) error {
	argv := r.Exec
	if r.Agent != nil {
		agent := r.Agent.Agent
		if agent == "" {
			agent = "codex"
		}
		switch agent {
		case "codex":
			argv = []string{"codex", "exec", r.Agent.Prompt}
		case "claude":
			argv = []string{"claude", "-p", r.Agent.Prompt}
		default:
			return fmt.Errorf("неизвестный агент %q", agent)
		}
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	if r.Agent != nil && r.Agent.Dir != "" {
		cmd.Dir = expandHome(r.Agent.Dir)
	}
	// Nothing is attached to a background run's stdin; inheriting it would let
	// an agent drain whatever the caller was reading.
	cmd.Stdin = nil
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + strings.TrimPrefix(path, "~")
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
