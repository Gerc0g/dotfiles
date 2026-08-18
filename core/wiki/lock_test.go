package wiki

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

// Autocommit stages everything dirty, so running it while a curator rewrites a
// page would capture that page half-written. The lock is what prevents it.
func TestAutocommitYieldsToARunningDrain(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	vault := vaultEnv(t)

	if err := os.MkdirAll(filepath.Join(vault, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A file that a commit would sweep up if the guard failed.
	if err := os.WriteFile(filepath.Join(vault, "page.md"), []byte("наполовину написано\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	release, _, err := AcquireDrainLock()
	if err != nil || release == nil {
		t.Fatalf("не удалось занять замок: %v", err)
	}
	defer release()

	var out bytes.Buffer
	if err := Autocommit(false, &out); err != nil {
		t.Fatalf("автокоммит обязан уступить, а не падать: %v", err)
	}
	if !strings.Contains(out.String(), "занят разбором") {
		t.Errorf("автокоммит не объяснил пропуск:\n%s", out.String())
	}
}
