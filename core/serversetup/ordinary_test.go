package serversetup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetupProvisionsPrivateOrdinaryWorkspaceAndPreservesNotes(t *testing.T) {
	o := opts(t, "ordinary-v1")
	if _, err := Run(o, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(o.RootDir, "srv/hq-data/runner/workspaces/ordinary")
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatalf("ordinary not private: %v %v", info, err)
	}
	note := filepath.Join(path, "note.md")
	if err := os.WriteFile(note, []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(o, false); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(note); string(data) != "keep me" {
		t.Fatal("setup erased ordinary workspace")
	}
}
