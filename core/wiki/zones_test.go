package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZonePath(t *testing.T) {
	vault := vaultEnv(t)

	cases := map[string]string{
		"":         vault,
		"root":     vault,
		"dev":      filepath.Join(vault, "dev"),
		"research": filepath.Join(vault, "research"),
		"brand":    filepath.Join(vault, "Personal Brand"),
	}
	for name, want := range cases {
		got, err := ZonePath(name)
		if err != nil {
			t.Fatalf("ZonePath(%q): %v", name, err)
		}
		if got != want {
			t.Errorf("ZonePath(%q) = %q, want %q", name, got, want)
		}
	}

	if _, err := ZonePath("ghost"); err == nil || !strings.Contains(err.Error(), "root, dev") {
		t.Errorf("unknown zone must fail with the valid list, got: %v", err)
	}
}

func TestZoneOverview(t *testing.T) {
	vault := vaultEnv(t)

	// dev exists with content (.obsidian must not count), research/brand absent.
	if err := os.MkdirAll(filepath.Join(vault, "dev", ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		filepath.Join(vault, "dev", "note.md"),
		filepath.Join(vault, "dev", ".obsidian", "state.json"),
	} {
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	zones, err := ZoneOverview()
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 3 {
		t.Fatalf("want 3 zones, got %d", len(zones))
	}
	if !zones[0].Exists || zones[0].Files != 1 {
		t.Errorf("dev: exists=%v files=%d (want 1, .obsidian excluded)", zones[0].Exists, zones[0].Files)
	}
	if zones[1].Exists || zones[2].Exists {
		t.Error("research/brand must not exist in a fresh vault")
	}
}
