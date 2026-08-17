package wiki

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SessionStart injects memory once and the session keeps that copy for as long
// as it lives. That is fine for a session measured in hours and useless for one
// measured in weeks: the curator runs four times a day, and none of it reaches
// a session opened on Monday.
//
// The fix is not to re-inject memory on every prompt — over a week that would
// paste the same index dozens of times and crowd out the actual work. Instead
// each session remembers the hot.md it was last shown, and only the lines that
// appeared since are sent. Nothing changed, nothing is sent, which is the
// normal case.

// hotSnapshotTTL is how long a session's snapshot is kept. Sessions do not
// announce their end, so old snapshots are swept by age.
const hotSnapshotTTL = 14 * 24 * time.Hour

func snapshotPath(home, session string) string {
	return filepath.Join(home, ".cache", "wikipedik", "sessions",
		sanitizeStamp(session)+".hot")
}

// HotDelta reports what appeared in hot.md since this session last saw it, and
// records the current state as the new baseline.
//
// An empty result means there is nothing to say. A session that has never been
// shown anything gets nothing either: SessionStart already delivered the full
// file, and repeating it would be noise.
func HotDelta(home, session, hotFile string) string {
	current, err := os.ReadFile(hotFile)
	if err != nil {
		return ""
	}

	path := snapshotPath(home, session)
	previous, err := os.ReadFile(path)
	if err != nil {
		// First sight of this session: record the baseline, say nothing.
		writeSnapshot(path, current)
		return ""
	}
	if string(previous) == string(current) {
		return ""
	}

	added := addedLines(string(previous), string(current))
	writeSnapshot(path, current)
	if len(added) == 0 {
		return ""
	}

	return "# Память проекта обновилась\n\n" +
		"Куратор разобрал новые записи. Появилось в docs/knowledge/hot.md:\n\n" +
		strings.Join(added, "\n") +
		"\n\nПолные страницы читай по требованию — hot.md это индекс, не весь текст."
}

// addedLines returns the meaningful lines present in next but not in previous.
//
// hot.md is regenerated whole on every refresh, so a line-level comparison is
// what separates new knowledge from a moved timestamp. Headings and the refresh
// stamp are dropped: they change on every rebuild and carry nothing.
func addedLines(previous, next string) []string {
	before := map[string]bool{}
	for _, line := range strings.Split(previous, "\n") {
		before[strings.TrimSpace(line)] = true
	}

	var added []string
	for _, line := range strings.Split(next, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || before[trimmed] {
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			continue // headings, prose, the refresh stamp
		}
		if strings.Contains(trimmed, "No entries in") {
			continue
		}
		added = append(added, trimmed)
	}
	return added
}

func writeSnapshot(path string, body []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, body, 0o644)
	pruneSnapshots(filepath.Dir(path))
}

// pruneSnapshots drops snapshots of sessions that are long gone.
func pruneSnapshots(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > hotSnapshotTTL {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

// SessionKey identifies the session a prompt belongs to.
//
// Agents pass their own id in the hook payload; when one does not, the working
// directory is the next best thing — two sessions in the same repo then share a
// baseline, which costs one missed notice, never a wrong one.
func SessionKey(payloadID, agent, cwd string) string {
	if id := strings.TrimSpace(payloadID); id != "" {
		return agent + "-" + id
	}
	sum := 0
	for _, r := range cwd {
		sum = sum*31 + int(r)
	}
	if sum < 0 {
		sum = -sum
	}
	return agent + "-cwd-" + filepath.Base(cwd) + "-" + strconv.Itoa(sum%100000)
}

// RepoHotFile is the hot.md of the repository a directory belongs to.
func RepoHotFile(cwd string) string {
	repoRoot := gitToplevel(cwd)
	if repoRoot == "" {
		return ""
	}
	return filepath.Join(repoRoot, "docs", "knowledge", "hot.md")
}
