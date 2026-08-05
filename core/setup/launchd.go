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
)

// transcriptScrubStep keeps the daily secret-scrubber job installed. It is the
// one piece of scheduled work that has run without interruption, so the
// declaration matches reality rather than aspiration.
//
// The step skips itself off macOS: the same binary runs on the Ubuntu box,
// where launchd does not exist.
func transcriptScrubStep() Step {
	return Step{
		Name:  "transcript-scrub",
		About: "ежедневная launchd-задача, вычищающая секреты из транскриптов",
		check: func(env Env) Result {
			if runtime.GOOS != "darwin" {
				return skipped("launchd есть только в macOS")
			}

			src := scrubSource(env)
			dst := scrubTarget(env)

			want, err := os.ReadFile(src)
			if err != nil {
				return drifted("%s не читается: %v", short(src), err)
			}

			got, err := os.ReadFile(dst)
			if os.IsNotExist(err) {
				return missing("%s", short(dst))
			}
			if err != nil {
				return drifted("%s: %v", short(dst), err)
			}

			if !bytes.Equal(want, got) {
				return drifted("%s отличается от копии в репозитории", short(dst))
			}

			// The file matching is not enough: a plist can sit in
			// LaunchAgents byte-identical to the repo copy while the job is
			// not registered at all, and then nothing ever runs. Ask launchd.
			if !agentLoaded() {
				return drifted("%s на месте, но задача не загружена в launchd", short(dst))
			}

			return ok("%s", short(dst))
		},
		apply: func(env Env) error {
			if runtime.GOOS != "darwin" {
				return nil
			}

			src := scrubSource(env)
			dst := scrubTarget(env)

			body, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("read %s: %w", src, err)
			}

			if err := ensureDir(filepath.Dir(dst)); err != nil {
				return err
			}
			if err := ensureDir(filepath.Join(env.Home, "Library", "Logs")); err != nil {
				return err
			}
			if err := os.WriteFile(dst, body, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", dst, err)
			}

			return reloadAgent(dst)
		},
	}
}

func scrubSource(env Env) string {
	return filepath.Join(env.Dotfiles, "scripts", "launchd", scrubPlist)
}

func scrubTarget(env Env) string {
	return filepath.Join(env.Home, "Library", "LaunchAgents", scrubPlist)
}

// agentLoaded asks launchd whether the job is registered in the user domain.
// `launchctl print` exits non-zero for an unknown label, which is exactly the
// signal we want; its output is irrelevant.
func agentLoaded() bool {
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), scrubLabel)
	return exec.Command("launchctl", "print", target).Run() == nil
}

// reloadAgent unloads any previous copy of the job and loads the current one.
// Unloading is best-effort: a job that was never loaded makes bootout fail, and
// that is not an error worth aborting on.
func reloadAgent(plist string) error {
	domain := fmt.Sprintf("gui/%d", os.Getuid())

	_ = exec.Command("launchctl", "bootout", domain+"/"+scrubLabel).Run()

	out, err := exec.Command("launchctl", "bootstrap", domain, plist).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap %s: %w: %s", plist, err, bytes.TrimSpace(out))
	}

	return nil
}
