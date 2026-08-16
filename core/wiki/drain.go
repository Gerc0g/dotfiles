package wiki

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// A background drain spends money on every run, so it is guarded twice:
// nothing happens below a candidate threshold, and nothing happens more often
// than an interval. Both guards are visible in the log — a job whose cost is
// invisible is a job nobody notices burning tokens.

// DrainGuards decide whether a background drain should run at all.
type DrainGuards struct {
	// MinCandidates skips scopes that have barely accumulated anything.
	MinCandidates int
	// Interval is the minimum time between runs for the same scope.
	Interval time.Duration
}

// DefaultDrainGuards are deliberately conservative.
func DefaultDrainGuards() DrainGuards {
	return DrainGuards{MinCandidates: inboxNudgeThreshold, Interval: 24 * time.Hour}
}

// DrainDecision explains what a guarded run did, so the log can say why
// nothing happened.
type DrainDecision struct {
	Run        bool
	Reason     string
	Candidates int
}

// stampPath is where the last run time of a scope is recorded.
func stampPath(scope Scope) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	name := "drain-" + sanitizeStamp(scope.String()) + ".stamp"
	return filepath.Join(home, ".cache", "wikipedik", name), nil
}

func sanitizeStamp(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r == '/' || r == ' ' {
			out[i] = '-'
		}
	}
	return string(out)
}

// ShouldDrain applies both guards and reports the decision.
func ShouldDrain(scope Scope, guards DrainGuards) (DrainDecision, error) {
	dir, err := scope.Path()
	if err != nil {
		return DrainDecision{}, err
	}
	if _, err := os.Stat(dir); err != nil {
		return DrainDecision{Reason: "скоупа нет в вольте"}, nil
	}

	candidates := CountStatus(dir, "candidate")
	if candidates < guards.MinCandidates {
		return DrainDecision{
			Candidates: candidates,
			Reason:     fmt.Sprintf("кандидатов %d, порог %d", candidates, guards.MinCandidates),
		}, nil
	}

	stamp, err := stampPath(scope)
	if err != nil {
		return DrainDecision{}, err
	}
	if info, err := os.Stat(stamp); err == nil {
		if since := time.Since(info.ModTime()); since < guards.Interval {
			return DrainDecision{
				Candidates: candidates,
				Reason: fmt.Sprintf("прошлый разбор %s назад, интервал %s",
					since.Round(time.Minute), guards.Interval),
			}, nil
		}
	}

	return DrainDecision{Run: true, Candidates: candidates,
		Reason: fmt.Sprintf("кандидатов %d", candidates)}, nil
}

// MarkDrained records the run time, so the interval guard has something to
// measure from. It is written before the curator starts: a crashed run must
// not turn into a retry loop that spends tokens on every session.
func MarkDrained(scope Scope) error {
	stamp, err := stampPath(scope)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
		return err
	}
	return os.WriteFile(stamp, nil, 0o644)
}

// AutoDrain is the background entry point: guards, then an autonomous
// curator run, then a scope-atomic commit of what it produced.
func AutoDrain(scope Scope, guards DrainGuards, commit bool, out io.Writer) error {
	decision, err := ShouldDrain(scope, guards)
	if err != nil {
		return err
	}
	if !decision.Run {
		LogAutosync("drain %s: пропуск (%s)", scope, decision.Reason)
		fmt.Fprintf(out, "разбор не нужен: %s\n", decision.Reason)
		return nil
	}

	if err := MarkDrained(scope); err != nil {
		return err
	}
	LogAutosync("drain %s: старт, %s", scope, decision.Reason)

	if err := Sync(scope, SyncOptions{Autonomous: true, Commit: commit}, out); err != nil {
		LogAutosync("drain %s: ошибка — %v", scope, err)
		return err
	}

	dir, _ := scope.Path()
	left := CountStatus(dir, "candidate")
	LogAutosync("drain %s: готово, было %d, осталось %d", scope, decision.Candidates, left)
	fmt.Fprintf(out, "разбор завершён: было %d кандидатов, осталось %d\n", decision.Candidates, left)
	return nil
}
