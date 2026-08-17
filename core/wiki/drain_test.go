package wiki

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The sweep exists because backlogs collect where sessions do not, so it must
// pick the fullest inbox rather than the nearest one.
func TestRepoScopesAndSweepPickTheFullestInbox(t *testing.T) {
	t.Setenv(RootEnv, t.TempDir())

	write := func(scope Scope, candidates int) {
		dir, err := scope.Path()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := ""
		for i := 0; i < candidates; i++ {
			body += "## [2026-08-17 10:00] capture | x\n\nStatus: candidate\n\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "_inbox.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	quiet := Scope{Company: "co", Product: "prod", Repo: "quiet"}
	loud := Scope{Company: "co", Product: "prod", Repo: "loud"}
	write(quiet, 6)
	write(loud, 40)

	scopes, err := RepoScopes()
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, s := range scopes {
		found[s.String()] = true
	}
	for _, want := range []string{quiet.String(), loud.String()} {
		if !found[want] {
			t.Errorf("RepoScopes missed %s: %v", want, scopes)
		}
	}

	guards := DrainGuards{MinCandidates: 5, Interval: time.Hour}
	var best Scope
	var bestCount int
	for _, scope := range scopes {
		decision, err := ShouldDrain(scope, guards)
		if err != nil || !decision.Run {
			continue
		}
		if decision.Candidates > bestCount {
			best, bestCount = scope, decision.Candidates
		}
	}
	if best.String() != loud.String() {
		t.Errorf("sweep picked %s (%d), want %s", best, bestCount, loud)
	}
}
