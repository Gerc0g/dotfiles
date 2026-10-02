package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func ordinaryFixture(t *testing.T) string {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("HQ_DATA_ROOT", root)
	path := filepath.Join(root, "runner/workspaces/ordinary")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOrdinaryDirectoryBindsOnlyItsIsolatedPreset(t *testing.T) {
	path := ordinaryFixture(t)
	for _, preset := range []string{"work", "ordinary"} {
		b, err := Resolve(path, preset)
		if err != nil || b.Preset != "ordinary" || b.Kind != "ordinary" || b.WorktreePath != path || b.HomeKey != "ordinary" || b.CompanyID != "" || b.RepoID != "" || b.EntityRef != "" {
			t.Fatalf("binding: %+v %v", b, err)
		}
		if _, err := contextOperation(b, "context.get", json.RawMessage(`{}`)); err == nil {
			t.Fatal("ordinary read company context")
		}
		if _, err := captureMemory(b, json.RawMessage(`{"title":"test","body":"test"}`)); err == nil {
			t.Fatal("ordinary captured company memory")
		}
	}
	for _, tc := range []struct{ path, preset string }{{path, "research"}, {filepath.Dir(path), "ordinary"}, {path + "/nested", "ordinary"}, {t.TempDir(), "ordinary"}} {
		if _, err := Resolve(tc.path, tc.preset); err == nil {
			t.Fatalf("accepted wrong route: %+v", tc)
		}
	}
}

func TestOrdinaryRejectsMissingAndSymlinkWorkspace(t *testing.T) {
	path := ordinaryFixture(t)
	os.Remove(path)
	if _, err := Resolve(path, "work"); err == nil {
		t.Fatal("accepted absent workspace")
	}
	if err := os.Symlink(t.TempDir(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(path, "ordinary"); err == nil {
		t.Fatal("accepted symlink workspace")
	}
}

func TestOrdinaryAdmissionIsSeparateFromResearchAndNamedCompany(t *testing.T) {
	for _, other := range []Binding{{Preset: "research", Kind: "research"}, {Preset: "work", CompanyID: "ordinary"}} {
		t.Run(other.Preset, func(t *testing.T) {
			ordinaryFixture(t)
			ordinary := Binding{Preset: "ordinary", Kind: "ordinary"}
			a, err := acquire(ordinary)
			if err != nil {
				t.Fatal(err)
			}
			defer a.release("completed", "")
			second, err := acquire(other)
			if err != nil {
				t.Fatalf("ordinary blocked unrelated scope: %v", err)
			}
			defer second.release("completed", "")
			if _, err := acquire(ordinary); err == nil {
				t.Fatal("concurrent ordinary admitted")
			}
		})
	}
}
