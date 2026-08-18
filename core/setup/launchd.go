package setup

// The bespoke launchd steps that used to live here — one per job, each with its
// own hand-written plist — are gone. Background work is declared in the routine
// registry now and materialised by routinesStep.
//
// What remains is the migration: the two jobs the platform ran before the
// registry existed. They keep running under their old labels until they are
// booted out, so setup removes them once.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// retiredJobs are launchd labels the platform used before routines.
var retiredJobs = []string{
	"com.gerc0g.transcript-scrub",
	"com.gerc0g.wikipedik-drain",
}

// retiredJobsStep unregisters and deletes the pre-registry jobs. Without it
// both the old and the new schedule would fire, and the drain would run twice
// as often as declared — at twice the cost.
func retiredJobsStep() Step {
	return Step{
		Name:  "routines-migrated",
		About: "старые самодельные launchd-задачи сняты",
		check: func(env Env) Result {
			if runtime.GOOS != "darwin" {
				return skipped("launchd есть только в macOS")
			}
			var left []string
			for _, label := range retiredJobs {
				if _, err := os.Stat(retiredPlist(env, label)); err == nil {
					left = append(left, label)
				}
			}
			if len(left) > 0 {
				return missing("остались: %v", left)
			}
			return ok("старых задач нет")
		},
		apply: func(env Env) error {
			if runtime.GOOS != "darwin" {
				return nil
			}
			domain := fmt.Sprintf("gui/%d", os.Getuid())
			for _, label := range retiredJobs {
				_ = exec.Command("launchctl", "bootout", domain+"/"+label).Run()
				path := retiredPlist(env, label)
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			return nil
		},
	}
}

func retiredPlist(env Env, label string) string {
	return filepath.Join(env.Home, "Library", "LaunchAgents", label+".plist")
}
