// Package setup declares the desired state of a machine and can both check it
// and apply it.
//
// The point of declaring it once is that `hq setup` and `hq doctor` run the
// same list: the installer cannot claim to have set up something the checker
// does not know about, and the checker cannot drift from what the installer
// does. The previous bootstrap.sh had no checker at all, which is how broken
// skill symlinks and a profile without its baseline survived unnoticed.
//
// Steps are additive and idempotent. Nothing here deletes or disables anything
// the user set up by hand; unmanaged state is reported, never removed.
package setup

import (
	"fmt"
	"os"
	"path/filepath"
)

// Status is the outcome of checking one step.
type Status string

const (
	// StatusOK means reality already matches the declaration.
	StatusOK Status = "ok"
	// StatusMissing means the target does not exist yet.
	StatusMissing Status = "missing"
	// StatusDrifted means the target exists but points somewhere else.
	StatusDrifted Status = "drifted"
	// StatusSkipped means the step does not apply on this machine.
	StatusSkipped Status = "skipped"
)

// NeedsApply reports whether a status calls for Apply.
func (s Status) NeedsApply() bool {
	return s == StatusMissing || s == StatusDrifted
}

// Result is a checked status plus a human-readable detail line.
type Result struct {
	Status Status
	Detail string
}

func ok(format string, args ...any) Result {
	return Result{Status: StatusOK, Detail: fmt.Sprintf(format, args...)}
}

func missing(format string, args ...any) Result {
	return Result{Status: StatusMissing, Detail: fmt.Sprintf(format, args...)}
}

func drifted(format string, args ...any) Result {
	return Result{Status: StatusDrifted, Detail: fmt.Sprintf(format, args...)}
}

func skipped(format string, args ...any) Result {
	return Result{Status: StatusSkipped, Detail: fmt.Sprintf(format, args...)}
}

// Env is the machine a plan runs against. Keeping the paths in a value rather
// than reading the environment inside every step is what makes the steps
// testable against a temporary directory.
type Env struct {
	Home      string
	Dotfiles  string
	Workspace string
	Vault     string
}

// NewEnv builds an Env from the current machine.
func NewEnv(workspace string) (Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Env{}, fmt.Errorf("resolve home directory: %w", err)
	}

	vault := os.Getenv(VaultEnv)
	if vault == "" {
		vault = defaultVault(home)
	}

	return Env{
		Home:      home,
		Dotfiles:  filepath.Join(home, "dotfiles"),
		Workspace: workspace,
		Vault:     vault,
	}, nil
}

// Step is one declared piece of machine state.
type Step struct {
	// Name is the short identifier shown in output.
	Name string
	// About explains what the step guarantees, in one line.
	About string

	check func(Env) Result
	apply func(Env) error
}

// Check reports the current state without changing anything.
func (s Step) Check(env Env) Result {
	return s.check(env)
}

// Apply brings the machine to the declared state.
func (s Step) Apply(env Env) error {
	if s.apply == nil {
		return fmt.Errorf("step %q cannot be applied automatically", s.Name)
	}
	return s.apply(env)
}

// Applicable reports whether the step can fix itself.
func (s Step) Applicable() bool {
	return s.apply != nil
}

// Report pairs a step with the result of checking it.
type Report struct {
	Step   Step
	Result Result
}

// Check runs every step's check against env, in declaration order.
func Check(env Env, steps []Step) []Report {
	reports := make([]Report, 0, len(steps))
	for _, step := range steps {
		reports = append(reports, Report{Step: step, Result: step.Check(env)})
	}
	return reports
}

// Apply fixes every step that needs it and re-checks the result. Steps are
// independent: a failure is recorded and the run continues, because stopping
// at the first error is how a half-finished machine stays half-finished
// without anyone knowing which half.
func Apply(env Env, steps []Step) ([]Report, []error) {
	var (
		reports []Report
		errs    []error
	)

	for _, step := range steps {
		result := step.Check(env)

		if result.Status.NeedsApply() && step.Applicable() {
			if err := step.Apply(env); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", step.Name, err))
			} else {
				result = step.Check(env)
			}
		}

		reports = append(reports, Report{Step: step, Result: result})
	}

	return reports, errs
}

// CountByStatus tallies reports per status.
func CountByStatus(reports []Report) map[Status]int {
	counts := map[Status]int{}
	for _, report := range reports {
		counts[report.Result.Status]++
	}
	return counts
}
