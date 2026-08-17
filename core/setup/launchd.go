package setup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	scrubLabel = "com.gerc0g.transcript-scrub"
	scrubPlist = scrubLabel + ".plist"

	drainLabel = "com.gerc0g.wikipedik-drain"
	drainPlist = drainLabel + ".plist"
)

// transcriptScrubStep keeps the daily secret-scrubber job installed. It is the
// one piece of scheduled work that has run without interruption, so the
// declaration matches reality rather than aspiration.
func transcriptScrubStep() Step {
	return launchdJobStep("transcript-scrub",
		"ежедневная launchd-задача, вычищающая секреты из транскриптов",
		scrubLabel, scrubPlist)
}

// wikipedikDrainStep schedules the curator. It is the only trigger that does
// not depend on how the user works: session hooks never fire for someone who
// stays in one editor session for weeks.
func wikipedikDrainStep() Step {
	return launchdJobStep("wikipedik-cron",
		"плановый разбор инбокса каждые 6 часов",
		drainLabel, drainPlist)
}

// launchdJobStep declares one scheduled job: the plist in LaunchAgents matches
// the repo copy, and launchd actually knows about it.
//
// The step skips itself off macOS: the same binary runs on the Ubuntu box,
// where launchd does not exist.
func launchdJobStep(name, about, label, plist string) Step {
	source := func(env Env) string {
		return filepath.Join(env.Dotfiles, "scripts", "launchd", plist)
	}
	target := func(env Env) string {
		return filepath.Join(env.Home, "Library", "LaunchAgents", plist)
	}

	return Step{
		Name:  name,
		About: about,
		check: func(env Env) Result {
			if runtime.GOOS != "darwin" {
				return skipped("launchd есть только в macOS")
			}

			want, err := os.ReadFile(source(env))
			if err != nil {
				return drifted("%s не читается: %v", short(source(env)), err)
			}

			got, err := os.ReadFile(target(env))
			if os.IsNotExist(err) {
				return missing("%s", short(target(env)))
			}
			if err != nil {
				return drifted("%s: %v", short(target(env)), err)
			}

			if !bytes.Equal(want, got) {
				return drifted("%s отличается от копии в репозитории", short(target(env)))
			}

			// The file matching is not enough: a plist can sit in
			// LaunchAgents byte-identical to the repo copy while the job is
			// not registered at all, and then nothing ever runs. Ask launchd.
			if !agentLoaded(label) {
				return drifted("%s на месте, но задача не загружена в launchd", short(target(env)))
			}

			return ok("%s", short(target(env)))
		},
		apply: func(env Env) error {
			if runtime.GOOS != "darwin" {
				return nil
			}

			body, err := os.ReadFile(source(env))
			if err != nil {
				return fmt.Errorf("read %s: %w", source(env), err)
			}

			if err := ensureDir(filepath.Dir(target(env))); err != nil {
				return err
			}
			if err := ensureDir(filepath.Join(env.Home, "Library", "Logs")); err != nil {
				return err
			}
			if err := os.WriteFile(target(env), body, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", target(env), err)
			}

			return reloadAgent(label, target(env))
		},
	}
}

// agentLoaded asks launchd whether the job is registered in the user domain.
// `launchctl print` exits non-zero for an unknown label, which is exactly the
// signal we want; its output is irrelevant.
func agentLoaded(label string) bool {
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
	return exec.Command("launchctl", "print", target).Run() == nil
}

// reloadAgent unloads any previous copy of the job and loads the current one.
// Unloading is best-effort: a job that was never loaded makes bootout fail, and
// that is not an error worth aborting on.
func reloadAgent(label, plist string) error {
	domain := fmt.Sprintf("gui/%d", os.Getuid())

	_ = exec.Command("launchctl", "bootout", domain+"/"+label).Run()

	out, err := exec.Command("launchctl", "bootstrap", domain, plist).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap %s: %w: %s", plist, err, bytes.TrimSpace(out))
	}

	return nil
}
