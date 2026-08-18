package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/routine"
)

// Background work is declared in the routine registry and materialised here.
// Hand-written plists used to be the source of truth; they are generated now,
// because the copy that drifts is always the one nobody reads.

// routinesStep keeps every declared routine registered with launchd, and
// removes the jobs of routines that no longer exist.
func routinesStep() Step {
	return Step{
		Name:  "routines",
		About: "расписания фоновых задач соответствуют реестру",
		check: func(env Env) Result {
			if runtime.GOOS != "darwin" {
				return skipped("launchd есть только в macOS")
			}
			all, err := routine.Load(env.Home, env.Dotfiles)
			if err != nil {
				return drifted("%v", err)
			}

			var problems []string
			for _, r := range all {
				want, err := routine.Plist(env.Home, env.Dotfiles, r)
				if err != nil {
					problems = append(problems, fmt.Sprintf("%s: %v", r.Name, err))
					continue
				}
				got, err := os.ReadFile(routine.PlistPath(env.Home, r))
				if os.IsNotExist(err) {
					problems = append(problems, r.Name+": нет расписания")
					continue
				}
				if err != nil || string(got) != want {
					problems = append(problems, r.Name+": расписание разошлось с реестром")
					continue
				}
				// A plist can sit in LaunchAgents byte-perfect while launchd
				// knows nothing about it, and then nothing ever runs.
				if !routine.Loaded(r) {
					problems = append(problems, r.Name+": не загружена в launchd")
				}
			}
			for _, orphan := range routine.Orphans(env.Home, all) {
				problems = append(problems, "лишняя задача "+filepath.Base(orphan))
			}

			if len(problems) > 0 {
				sort.Strings(problems)
				return missing("%s", strings.Join(problems, "; "))
			}
			return ok("%d задач по расписанию", len(all))
		},
		apply: func(env Env) error {
			if runtime.GOOS != "darwin" {
				return nil
			}
			all, err := routine.Load(env.Home, env.Dotfiles)
			if err != nil {
				return err
			}
			for _, r := range all {
				if err := routine.Install(env.Home, env.Dotfiles, r); err != nil {
					return err
				}
			}
			for _, orphan := range routine.Orphans(env.Home, all) {
				label := strings.TrimSuffix(filepath.Base(orphan), ".plist")
				routine.Unload(routine.Routine{Name: strings.TrimPrefix(label, "com.gerc0g.routine.")})
				if err := os.Remove(orphan); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// routinePulseStep reports whether background work is actually happening.
//
// A schedule is a claim, not evidence. Both times this platform lost a
// background job — hooks silently untrusted, a scheduled run refused by
// macOS — the declaration stayed perfectly healthy while nothing ran.
func routinePulseStep() Step {
	return Step{
		Name:  "routine-pulse",
		About: "фоновые задачи действительно отрабатывают",
		check: func(env Env) Result {
			all, err := routine.Load(env.Home, env.Dotfiles)
			if err != nil {
				return drifted("%v", err)
			}
			state := routine.LoadState(env.Home)

			var cold []string
			for _, r := range all {
				status := state[r.Name]
				if status.Disabled {
					continue
				}
				period := r.Schedule.Period()
				if period == 0 {
					continue
				}
				switch {
				case status.Last.IsZero():
					cold = append(cold, r.Name+" ни разу")
				case status.Outcome == routine.OutcomeFailed:
					cold = append(cold, fmt.Sprintf("%s упала: %s", r.Name, status.Detail))
				case time.Since(status.Last) > 2*period:
					cold = append(cold, fmt.Sprintf("%s молчит %s",
						r.Name, time.Since(status.Last).Round(time.Hour)))
				}
			}
			if len(cold) > 0 {
				sort.Strings(cold)
				return drifted("%s", strings.Join(cold, "; "))
			}
			return ok("все задачи отрабатывают")
		},
	}
}
