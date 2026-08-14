package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The vault is wider than project memory. `hq wiki` owns the dev zone (the
// memory loop around projects); `hq wikipedik` fronts the vault itself and
// its personal zones — research (learning) and brand (articles, content).
// Zone structure is deliberately not prescribed here yet: the skeleton came
// first, the pipelines are a separate conversation.

// Zone is one vault space.
type Zone struct {
	// Name is the CLI handle.
	Name string
	// Dir is the directory name inside the vault.
	Dir string
	// About is the zone's role, one line.
	About string
}

// VaultZones lists the known zones in display order.
var VaultZones = []Zone{
	{Name: "dev", Dir: "dev", About: "память проектов (hq wiki)"},
	{Name: "research", Dir: "research", About: "обучение и исследования"},
	{Name: "brand", Dir: "Personal Brand", About: "личный бренд: статьи и контент"},
}

// ZonePath resolves a zone handle to its directory. "root" (or empty) is the
// vault itself.
func ZonePath(name string) (string, error) {
	root, err := VaultRoot()
	if err != nil {
		return "", err
	}
	if name == "" || name == "root" {
		return root, nil
	}
	for _, zone := range VaultZones {
		if zone.Name == name {
			return filepath.Join(root, zone.Dir), nil
		}
	}
	var names []string
	for _, zone := range VaultZones {
		names = append(names, zone.Name)
	}
	return "", fmt.Errorf("нет зоны %q (есть: root, %s)", name, strings.Join(names, ", "))
}

// ZoneInfo is one zone's on-disk state.
type ZoneInfo struct {
	Zone
	Path   string
	Exists bool
	Files  int
}

// ZoneOverview reports every zone's existence and size.
func ZoneOverview() ([]ZoneInfo, error) {
	var out []ZoneInfo
	for _, zone := range VaultZones {
		path, err := ZonePath(zone.Name)
		if err != nil {
			return nil, err
		}
		info := ZoneInfo{Zone: zone, Path: path}
		if _, err := os.Stat(path); err == nil {
			info.Exists = true
			info.Files = countFiles(path)
		}
		out = append(out, info)
	}
	return out, nil
}

func countFiles(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			// Obsidian's own state is not content.
			if entry.Name() == ".obsidian" || entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		count++
		return nil
	})
	return count
}

// VaultState is the vault's git pulse.
type VaultState struct {
	Root       string
	Dirty      int
	Unpushed   int
	LastCommit string
}

// VaultStatus reports the vault repository state.
func VaultStatus() (VaultState, error) {
	root, err := VaultRoot()
	if err != nil {
		return VaultState{}, err
	}
	state := VaultState{Root: root}

	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return state, nil
	}
	state.Dirty = len(Dirty(root))
	if last, err := gitOut(root, "log", "-1", "--format=%cr · %s"); err == nil {
		state.LastCommit = last
	}
	if out, err := gitOut(root, "rev-list", "--count", "@{u}..HEAD"); err == nil {
		fmt.Sscanf(out, "%d", &state.Unpushed)
	}
	return state, nil
}
