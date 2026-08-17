package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Drains now have three triggers — two session hooks and a scheduled job — and
// each one spawns a curator that runs for tens of minutes. Without a lock a
// quiet evening can end with three agents drafting the same vault at once:
// three times the spend, and concurrent rewrites of pages that are edited whole.

func lockPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "wikipedik", "drain.lock"), nil
}

// AcquireDrainLock takes the drain lock, reporting who holds it if it is busy.
// The returned release function is a no-op when the lock was not taken.
func AcquireDrainLock() (release func(), holder int, err error) {
	path, err := lockPath()
	if err != nil {
		return nil, 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, 0, err
	}

	// O_EXCL makes the check and the claim one step, so two drains starting in
	// the same second cannot both win.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		fmt.Fprintf(file, "%d", os.Getpid())
		_ = file.Close()
		return func() { _ = os.Remove(path) }, 0, nil
	}
	if !os.IsExist(err) {
		return nil, 0, err
	}

	// A lock file outlives a killed process, so it only counts while its owner
	// is alive. Otherwise one crash would stop draining forever.
	if pid := lockHolder(path); pid > 0 && processAlive(pid) {
		return nil, pid, nil
	}
	_ = os.Remove(path)
	return AcquireDrainLock()
}

func lockHolder(path string) int {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil {
		return 0
	}
	return pid
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks for existence without touching the process.
	return proc.Signal(syscall.Signal(0)) == nil
}
