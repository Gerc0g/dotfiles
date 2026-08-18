// Command hq-cron runs the scheduled wiki drain on behalf of launchd.
//
// It exists for one reason: macOS keeps the vault under ~/Desktop behind TCC,
// and a launchd job reaches it only if the binary holds Full Disk Access. That
// grant is keyed to the binary's contents, and `make build` rewrites bin/hq
// several times a day — so a grant given to bin/hq is revoked by the next
// build, silently.
//
// This launcher is frozen instead. It is installed once, never overwritten,
// and granted Full Disk Access once. It spawns the real binary and stays its
// parent, so the child inherits the grant — the same reason every command run
// from a granted terminal can read the Desktop.
//
// Keep this file tiny and unchanging. Rebuilding it revokes the grant.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		os.Exit(1)
	}

	hq := os.Getenv("HQ_BIN")
	if hq == "" {
		hq = filepath.Join(home, "dotfiles", "bin", "hq")
	}

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"wiki", "cron", "--commit"}
	}

	cmd := exec.Command(hq, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		os.Exit(1)
	}
}
