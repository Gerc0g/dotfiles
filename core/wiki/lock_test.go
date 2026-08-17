package wiki

import (
	"os"
	"path/filepath"
	"testing"
)

// Three triggers can fire at once; only one curator may run.
func TestDrainLockIsExclusiveAndReleasable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	release, holder, err := AcquireDrainLock()
	if err != nil {
		t.Fatal(err)
	}
	if release == nil {
		t.Fatalf("first acquire failed, holder %d", holder)
	}

	second, holder, err := AcquireDrainLock()
	if err != nil {
		t.Fatal(err)
	}
	if second != nil {
		t.Error("lock handed out twice")
	}
	if holder <= 0 {
		t.Error("a busy lock must name its holder")
	}

	release()
	third, _, err := AcquireDrainLock()
	if err != nil || third == nil {
		t.Fatalf("lock not reusable after release: %v", err)
	}
	third()
}

// A killed drain must not block every future one.
func TestDrainLockIgnoresDeadHolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := lockPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeStaleLock(path); err != nil {
		t.Fatal(err)
	}

	release, holder, err := AcquireDrainLock()
	if err != nil {
		t.Fatal(err)
	}
	if release == nil {
		t.Fatalf("stale lock blocked a new drain (holder %d)", holder)
	}
	release()
}

// writeStaleLock leaves a lock owned by a pid that cannot be running.
func writeStaleLock(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Above the pid ceiling, so it can never match a live process.
	return os.WriteFile(path, []byte("4194304"), 0o644)
}
