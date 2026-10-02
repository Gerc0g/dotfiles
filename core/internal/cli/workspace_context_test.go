package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gerc0g/dotfiles/core/workspace"
)

func TestWorkspaceContextCommandsCheckAndRepair(t *testing.T) {
	root := lsFixture(t)
	m, err := workspace.New(root, workspace.WithProjectSync(func() {}), workspace.WithWarnings(io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	w, err := m.Start("acme", "product", "repo", "existing")
	if err != nil {
		t.Fatal(err)
	}
	lsWrite(t, filepath.Join(root, "acme", "AGENTS.md"), "company context\n")
	lsWrite(t, filepath.Join(root, "acme", "product", "AGENTS.md"), "product context\n")
	lsWrite(t, filepath.Join(root, "acme", "product", "repo", "AGENTS.md"), "repo context\n")

	for _, repair := range []bool{false, true} {
		verb := "context-check"
		if repair {
			verb = "context-repair"
		}
		cmd := NewRoot()
		var out, stderr bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"workspace", verb, "acme", "product", "repo", w.ID, "--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s: %v", verb, err)
		}
		var got struct {
			Worktree string                   `json:"worktree"`
			Issues   []workspace.ContextIssue `json:"issues"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.Worktree != w.Path {
			t.Fatalf("invalid JSON output %q: %v", out.String(), err)
		}
		if stderr.Len() != 0 {
			t.Fatalf("unexpected command stderr: %s", stderr.String())
		}
		_, statErr := os.Stat(filepath.Join(w.Path, "AGENTS.md"))
		if !repair && !os.IsNotExist(statErr) {
			t.Fatal("context-check created files")
		}
		if repair && statErr != nil {
			t.Fatalf("context-repair did not seed missing instructions: %v", statErr)
		}
		if repair && (len(got.Issues) != 1 || got.Issues[0].Severity != "warning") {
			t.Fatalf("optional Wiki should be the only remaining warning: %+v", got.Issues)
		}
	}
}

func TestWorkspaceStartRequestUsesPreEffectRefusalMarker(t *testing.T) {
	lsFixture(t)
	cmd := NewRoot()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"workspace", "start", "acme", "product", "missing", "Fix bug", "--request-id", "creation-123"})
	err := cmd.Execute()
	if err == nil || !strings.HasPrefix(err.Error(), "HQ_WORKTREE_NOT_CREATED:") {
		t.Fatalf("missing refusal marker: %v", err)
	}
}
