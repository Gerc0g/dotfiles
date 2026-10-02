package cli

import (
	"encoding/json"
	"github.com/Gerc0g/dotfiles/core/runner"
	"os"
	"path/filepath"
	"testing"
)

func TestLsJSONOrdinaryIsProvisionedCanonicalDirectory(t *testing.T) {
	lsFixture(t)
	data, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("HQ_DATA_ROOT", data)
	path := filepath.Join(data, "runner/workspaces/ordinary")
	for _, mode := range []string{"absent", "directory", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			os.Remove(path)
			if mode == "directory" {
				os.MkdirAll(path, 0700)
			}
			if mode == "symlink" {
				os.Symlink(t.TempDir(), path)
			}
			out, _, err := executeLs("--json")
			if err != nil {
				t.Fatal(err)
			}
			var catalog struct {
				Ordinary *struct {
					Path string `json:"path"`
				} `json:"ordinary"`
			}
			if err := json.Unmarshal([]byte(out), &catalog); err != nil {
				t.Fatal(err)
			}
			if mode != "directory" {
				if catalog.Ordinary != nil {
					t.Fatal("unavailable ordinary advertised")
				}
				if mode == "absent" {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("catalog provisioned workspace")
					}
				}
				return
			}
			if catalog.Ordinary == nil || catalog.Ordinary.Path != path {
				t.Fatalf("missing canonical ordinary: %s", out)
			}
			b, err := runner.Resolve(catalog.Ordinary.Path, "work")
			if err != nil || b.Preset != "ordinary" {
				t.Fatalf("catalog not launchable: %+v %v", b, err)
			}
		})
	}
}
