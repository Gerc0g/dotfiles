package wiki

import (
	"os"
	"path/filepath"
	"time"
)

// Agent hooks fail silently by design: a hook that breaks a session is worse
// than a hook that does nothing. That safety turns into a trap — codex revokes
// hook trust on every edit of the script, and the memory hooks stayed dead for
// three days and 53 sessions before anyone noticed.
//
// Checking the config cannot catch it: the trust entry survives, only its hash
// goes stale. So the pulse is the firing itself. Each run stamps a file, and
// `hq doctor` reads how long ago that was.

// HookStale is how long a silent hook goes before doctor calls it out. Long
// enough to survive a weekend away from the machine.
const HookStale = 3 * 24 * time.Hour

func hookStampPath(home, agent, event string) string {
	name := "hook-" + sanitizeStamp(agent) + "-" + sanitizeStamp(event) + ".stamp"
	return filepath.Join(home, ".cache", "wikipedik", name)
}

// TouchHook records that a hook just fired.
func TouchHook(home, agent, event string) {
	path := hookStampPath(home, agent, event)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	now := time.Now()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		return
	}
	_ = os.Chtimes(path, now, now)
}

// LastHook reports when a hook last fired. A zero time means never.
func LastHook(home, agent, event string) time.Time {
	info, err := os.Stat(hookStampPath(home, agent, event))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
