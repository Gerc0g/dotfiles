package routine

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// launchd is the scheduler. The registry generates its plists rather than the
// other way round: a hand-written plist is a second source of truth, and the
// one that drifts is always the one nobody reads.
//
// Every routine runs through hq-cron. That launcher holds the Full Disk Access
// grant — macOS keys it to a path, and bin/hq already carries an explicit deny
// from a prompt a background job could not answer.

// PlistName is the file a routine's schedule lives in.
func PlistName(r Routine) string { return r.Label() + ".plist" }

// PlistPath is where launchd looks for it.
func PlistPath(home string, r Routine) string {
	return filepath.Join(home, "Library", "LaunchAgents", PlistName(r))
}

// Plist renders the launchd job for a routine.
func Plist(home, dotfiles string, r Routine) (string, error) {
	var schedule string
	switch {
	case r.Schedule.Every > 0:
		schedule = fmt.Sprintf("    <key>StartInterval</key>\n    <integer>%d</integer>\n",
			int(r.Schedule.Every.Seconds()))
	case r.Schedule.At != "":
		hour, minute, err := parseDaily(r.Schedule.At)
		if err != nil {
			return "", err
		}
		schedule = fmt.Sprintf("    <key>StartCalendarInterval</key>\n    <dict>\n"+
			"        <key>Hour</key>\n        <integer>%d</integer>\n"+
			"        <key>Minute</key>\n        <integer>%d</integer>\n    </dict>\n", hour, minute)
	default:
		return "", fmt.Errorf("%s: нечего ставить в расписание", r.Name)
	}

	logPath := filepath.Join(home, "Library", "Logs", "hq-routines.log")

	var out bytes.Buffer
	fmt.Fprint(&out, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+
		"<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n"+
		"<plist version=\"1.0\">\n<dict>\n")
	fmt.Fprintf(&out, "    <!-- Сгенерировано hq из реестра routine. Правки руками потеряются. -->\n")
	fmt.Fprintf(&out, "    <key>Label</key>\n    <string>%s</string>\n", r.Label())
	fmt.Fprintf(&out, "    <key>ProgramArguments</key>\n    <array>\n")
	for _, arg := range []string{filepath.Join(dotfiles, "bin", "hq-cron"), "routine", "run", r.Name} {
		fmt.Fprintf(&out, "        <string>%s</string>\n", arg)
	}
	fmt.Fprintf(&out, "    </array>\n")
	// launchd hands a job a bare PATH, so anything the work shells out to —
	// codex above all — would not be found.
	fmt.Fprintf(&out, "    <key>EnvironmentVariables</key>\n    <dict>\n"+
		"        <key>PATH</key>\n        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>\n    </dict>\n")
	fmt.Fprint(&out, schedule)
	fmt.Fprintf(&out, "    <key>StandardOutPath</key>\n    <string>%s</string>\n", logPath)
	fmt.Fprintf(&out, "    <key>StandardErrorPath</key>\n    <string>%s</string>\n", logPath)
	fmt.Fprint(&out, "</dict>\n</plist>\n")
	return out.String(), nil
}

// Loaded reports whether launchd knows the job.
func Loaded(r Routine) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), r.Label())
	return exec.Command("launchctl", "print", target).Run() == nil
}

// Install writes the plist and registers it with launchd.
func Install(home, dotfiles string, r Routine) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	body, err := Plist(home, dotfiles, r)
	if err != nil {
		return err
	}
	path := PlistPath(home, r)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(home, "Library", "Logs"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	return Reload(r, path)
}

// Reload replaces any registered copy of the job with the current one.
// Unloading is best effort: a job that was never loaded makes bootout fail,
// and that is not worth aborting on.
func Reload(r Routine, path string) error {
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+r.Label()).Run()

	out, err := exec.Command("launchctl", "bootstrap", domain, path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap %s: %w: %s", path, err, bytes.TrimSpace(out))
	}
	return nil
}

// Unload removes a job from launchd without deleting its plist.
func Unload(r Routine) {
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+r.Label()).Run()
}

// Orphans are generated plists whose routine no longer exists — a routine the
// user deleted from the config must stop running, not linger in launchd.
func Orphans(home string, known []Routine) []string {
	dir := filepath.Join(home, "Library", "LaunchAgents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	live := map[string]bool{}
	for _, r := range known {
		live[PlistName(r)] = true
	}

	var orphans []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "com.gerc0g.routine.") || live[name] {
			continue
		}
		orphans = append(orphans, filepath.Join(dir, name))
	}
	return orphans
}
