package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHot(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const hotBefore = `# repo hot context

## Fresh Lessons

- lessons.md#a: Первый урок

## Critical Gotchas

- gotchas.md#x: Старые грабли

<!-- last refreshed: 2026-08-17 10:00 -->
`

func TestHotDeltaSpeaksOnlyAboutWhatIsNew(t *testing.T) {
	home := t.TempDir()
	hot := filepath.Join(t.TempDir(), "hot.md")
	writeHot(t, hot, hotBefore)

	// First contact is silent: SessionStart has already delivered the file.
	if got := HotDelta(home, "s1", hot); got != "" {
		t.Fatalf("first call said %q, want silence", got)
	}
	// Nothing changed, nothing to say — the normal case, on every prompt.
	if got := HotDelta(home, "s1", hot); got != "" {
		t.Fatalf("unchanged file said %q, want silence", got)
	}

	// A rebuild that only moves the timestamp is not news.
	writeHot(t, hot, strings.Replace(hotBefore, "10:00", "14:00", 1))
	if got := HotDelta(home, "s1", hot); got != "" {
		t.Fatalf("timestamp-only rebuild said %q, want silence", got)
	}

	// A genuinely new entry is.
	writeHot(t, hot, strings.Replace(strings.Replace(hotBefore, "10:00", "16:00", 1),
		"- gotchas.md#x: Старые грабли",
		"- gotchas.md#x: Старые грабли\n- gotchas.md#y: Свежие грабли", 1))

	got := HotDelta(home, "s1", hot)
	if !strings.Contains(got, "Свежие грабли") {
		t.Fatalf("new entry missing from %q", got)
	}
	if strings.Contains(got, "Первый урок") || strings.Contains(got, "Старые грабли") {
		t.Errorf("delta repeated what the session already had:\n%s", got)
	}

	// Reported once, not on every prompt afterwards.
	if again := HotDelta(home, "s1", hot); again != "" {
		t.Errorf("delta repeated itself: %q", again)
	}
}

// Each session tracks its own baseline: a session opened later must not be
// told about entries it was already shown at start.
func TestHotDeltaIsPerSession(t *testing.T) {
	home := t.TempDir()
	hot := filepath.Join(t.TempDir(), "hot.md")
	writeHot(t, hot, hotBefore)

	HotDelta(home, "old", hot)
	writeHot(t, hot, hotBefore+"\n- lessons.md#b: Второй урок\n")

	if got := HotDelta(home, "fresh", hot); got != "" {
		t.Errorf("a session seeing the file for the first time got %q", got)
	}
	if got := HotDelta(home, "old", hot); !strings.Contains(got, "Второй урок") {
		t.Errorf("the older session was not told: %q", got)
	}
}

func TestSessionKeyFallsBackToDirectory(t *testing.T) {
	if SessionKey("abc", "claude", "/tmp/x") != "claude-abc" {
		t.Error("payload id must win")
	}
	a := SessionKey("", "codex", "/tmp/one")
	b := SessionKey("", "codex", "/tmp/two")
	if a == b {
		t.Error("different directories must not share a baseline")
	}
	if a != SessionKey("", "codex", "/tmp/one") {
		t.Error("the fallback key must be stable")
	}
}
