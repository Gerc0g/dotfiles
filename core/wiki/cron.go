package wiki

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// The scheduled run is the only trigger that does not depend on how the user
// works. Session hooks never fire for someone who lives in one editor session
// for weeks, and that is most of the point of automating this at all.
//
// It has one fragile dependency: the vault sits under ~/Desktop, which macOS
// puts behind TCC, so launchd is refused unless the binary holds Full Disk
// Access. That grant is keyed to the binary and `make build` rewrites it —
// which means it can be revoked silently. Everything here exists so that it
// cannot be revoked quietly: the run records its own outcome, and doctor reads
// it back.

// CronStale is how long a scheduled sweep may be missing before doctor says so.
const CronStale = 36 * time.Hour

// Cron outcomes, recorded as the stamp file's contents.
const (
	CronOK     = "ok"
	CronDenied = "denied"
)

func cronStampPath(home string) string {
	return filepath.Join(home, ".cache", "wikipedik", "cron-sweep.stamp")
}

// RecordCron stores the outcome of a scheduled run.
func RecordCron(home, outcome string) {
	path := cronStampPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(outcome), 0o644)
}

// LastCron reports when the last scheduled run happened and how it ended. A
// zero time means the job has never run.
func LastCron(home string) (time.Time, string) {
	path := cronStampPath(home)
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return info.ModTime(), ""
	}
	return info.ModTime(), string(body)
}

// VaultReachable reports whether this process may read the vault, separating a
// permission refusal from a vault that is simply not there.
func VaultReachable() error {
	root, err := VaultRoot()
	if err != nil {
		return err
	}
	if _, err := os.ReadDir(root); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return fmt.Errorf("нет доступа к %s: macOS не пускает — выдай Full Disk Access для bin/hq", root)
		}
		return err
	}
	return nil
}

// RunCron is the scheduled entry point: prove access, then sweep.
func RunCron(home string, guards DrainGuards, commit bool, out io.Writer) error {
	if err := VaultReachable(); err != nil {
		RecordCron(home, CronDenied)
		LogAutosync("cron: %v", err)
		return err
	}

	// Recorded before the sweep, not after: a run that dies mid-drain still
	// happened, and a stamp that only appears on success would report the job
	// as missing rather than as broken.
	RecordCron(home, CronOK)
	LogAutosync("cron: старт")
	return SweepDrainInline(guards, commit, out)
}
