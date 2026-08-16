package wiki

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// SpawnDrain starts a drain in its own process and returns immediately.
//
// Hooks fire at the end of a session and must not hold it: a drain takes
// minutes. The child re-checks the guards itself, so spawning is cheap even
// when nothing needs draining — and every decision still lands in the log.
func SpawnDrain(scope Scope, guards DrainGuards, commit bool) error {
	// Guard first: spawning a process per session end, only to have it exit,
	// is noise in the process table and in the log.
	decision, err := ShouldDrain(scope, guards)
	if err != nil {
		return err
	}
	if !decision.Run {
		LogAutosync("drain %s: пропуск (%s)", scope, decision.Reason)
		return nil
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}

	args := []string{"wiki", "drain", scope.String(),
		"--min-candidates", fmt.Sprint(guards.MinCandidates),
		"--interval-hours", fmt.Sprint(int(guards.Interval / time.Hour)),
	}
	if commit {
		args = append(args, "--commit")
	}

	cmd := exec.Command(self, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	// Detach: the session that triggered this is about to exit.
	cmd.SysProcAttr = detachAttr()

	if err := cmd.Start(); err != nil {
		LogAutosync("drain %s: не запустился — %v", scope, err)
		return err
	}
	LogAutosync("drain %s: запущен фоном (pid %d), кандидатов %d",
		scope, cmd.Process.Pid, decision.Candidates)
	return cmd.Process.Release()
}
