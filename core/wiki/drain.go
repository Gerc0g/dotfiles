package wiki

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/world"
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

// platformCompany is where the dotfiles repo keeps its memory. It does not fit
// the company/product/repo model, so it is a company-level scope of its own.
const platformCompany = "_platform"

// Unstamp forgets the last run of a scope, so the interval guard stops holding
// it back.
func Unstamp(scope Scope) {
	if stamp, err := stampPath(scope); err == nil {
		_ = os.Remove(stamp)
	}
}

// ScopeForDir resolves which memory scope a working directory belongs to, so
// a hook can drain "the repo I just worked in" without being told.
//
// The platform repo is special: its memory lives at _platform/repos/dotfiles,
// which the company/product/repo model does not describe — the scope is the
// company level instead.
func ScopeForDir(dir string) (Scope, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Scope{}, err
	}
	dotfiles := filepath.Join(home, "dotfiles")
	if dir == dotfiles || strings.HasPrefix(dir, dotfiles+string(filepath.Separator)) {
		return Scope{Company: platformCompany}, nil
	}

	workRoot, err := world.Root()
	if err != nil {
		return Scope{}, err
	}
	rel := strings.TrimPrefix(dir, workRoot+string(filepath.Separator))
	if rel == dir {
		return Scope{}, fmt.Errorf("каталог вне рабочего корня: %s", dir)
	}

	// The worktree pool repeats the triple one level deeper:
	// .worktrees/<co>/<prod>/<repo>/<id>/…
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) > 0 && parts[0] == ".worktrees" {
		parts = parts[1:]
	}
	if len(parts) < 3 {
		return Scope{}, fmt.Errorf("каталог не внутри репозитория: %s", dir)
	}
	return Scope{Company: parts[0], Product: parts[1], Repo: parts[2]}, nil
}

// maxDrainPasses bounds one background run.
//
// A curator asked to drain a large backlog stops well before the inbox is
// empty — the first live run curated 9 of 138 and quit. One pass per day would
// take that repo two weeks, so a run repeats itself while it is still making
// progress. The cap is what keeps an unattended job from spending the whole
// subscription on one inbox.
const maxDrainPasses = 3

// AutoDrain is the background entry point: guards, then autonomous curator
// passes, each committing what it produced.
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

	release, holder, err := AcquireDrainLock()
	if err != nil {
		return err
	}
	if release == nil {
		LogAutosync("drain %s: пропуск, уже идёт разбор (pid %d)", scope, holder)
		fmt.Fprintf(out, "уже идёт другой разбор (pid %d)\n", holder)
		return nil
	}
	defer release()

	// Stamp before the first pass: a crashed run must not become a retry loop
	// that spends tokens on every session end.
	if err := MarkDrained(scope); err != nil {
		return err
	}
	LogAutosync("drain %s: старт, %s", scope, decision.Reason)

	dir, _ := scope.Path()
	left := decision.Candidates
	for pass := 1; pass <= maxDrainPasses; pass++ {
		if err := Sync(scope, SyncOptions{Autonomous: true, Commit: commit}, out); err != nil {
			LogAutosync("drain %s: ошибка на проходе %d — %v", scope, pass, err)
			if pass == 1 {
				// The curator never ran, so the stamp records an attempt that
				// did not happen — and would block this scope for a day over a
				// broken PATH or a missing login. Let the next trigger retry.
				Unstamp(scope)
			}
			return err
		}

		before := left
		left = CountStatus(dir, "candidate")
		LogAutosync("drain %s: проход %d, было %d, осталось %d", scope, pass, before, left)

		// Nothing left to do, or the pass moved nothing: another one would
		// only repeat the same refusal at full price.
		if left < guards.MinCandidates {
			break
		}
		if left >= before {
			LogAutosync("drain %s: прогресса нет, останавливаюсь", scope)
			break
		}
	}

	LogAutosync("drain %s: готово, было %d, осталось %d", scope, decision.Candidates, left)
	fmt.Fprintf(out, "разбор завершён: было %d кандидатов, осталось %d\n", decision.Candidates, left)
	return nil
}

// RepoScopes lists every repo-level scope in the vault, plus the platform
// scope, which lives one level up.
func RepoScopes() ([]Scope, error) {
	projects, err := ProjectsRoot()
	if err != nil {
		return nil, err
	}

	dirs, err := filepath.Glob(filepath.Join(projects, "*", "*", "repos", "*"))
	if err != nil {
		return nil, err
	}

	scopes := []Scope{{Company: platformCompany}}
	for _, dir := range dirs {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		rel, err := filepath.Rel(projects, dir)
		if err != nil {
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 4 {
			continue
		}
		scopes = append(scopes, Scope{Company: parts[0], Product: parts[1], Repo: parts[3]})
	}
	return scopes, nil
}

// SweepDrain drains the single most backed-up scope that passes the guards.
//
// The session trigger only ever reaches the repo just worked in, and the
// backlog does not sit there: it collects in repos nobody has opened for
// weeks — 55 captures in one, 20 in another. Sweeping at the end of a session
// lends that moment to whichever repo needs it most.
//
// One scope per sweep, on purpose. Draining four repos at once quadruples the
// spend of a single session ending, and the guards would let that happen every
// day.
func SweepDrain(guards DrainGuards, commit bool) error {
	best, count, err := PickSweepScope(guards)
	if err != nil || count == 0 {
		return err
	}
	LogAutosync("sweep: выбран %s (кандидатов %d)", best, count)
	return SpawnDrain(best, guards, commit)
}

// SweepDrainInline is the sweep for a scheduled run, which has nothing to
// return to and should stay alive for as long as the work takes — launchd
// tracks a job by its process.
func SweepDrainInline(guards DrainGuards, commit bool, out io.Writer) error {
	best, count, err := PickSweepScope(guards)
	if err != nil {
		return err
	}
	if count == 0 {
		fmt.Fprintln(out, "разбирать нечего")
		return nil
	}
	LogAutosync("sweep: выбран %s (кандидатов %d)", best, count)
	return AutoDrain(best, guards, commit, out)
}

// PickSweepScope returns the scope with the largest inbox that passes the
// guards, or a zero count when none does.
func PickSweepScope(guards DrainGuards) (Scope, int, error) {
	scopes, err := RepoScopes()
	if err != nil {
		return Scope{}, 0, err
	}

	var best Scope
	var bestCount int
	for _, scope := range scopes {
		decision, err := ShouldDrain(scope, guards)
		if err != nil || !decision.Run {
			continue
		}
		if decision.Candidates > bestCount {
			best, bestCount = scope, decision.Candidates
		}
	}
	if bestCount == 0 {
		LogAutosync("sweep: разбирать нечего")
	}
	return best, bestCount, nil
}
