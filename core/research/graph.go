package research

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Gerc0g/dotfiles/core/wiki"
)

// Obsidian keeps graph settings in .obsidian/graph.json. Writing them from
// here makes the learning view reproducible: the graph is part of how the
// zone is read, not a preference someone re-clicks after every reinstall.

// graphQuery lists what belongs on the learning graph, rather than excluding
// what does not: a whitelist keeps service files (the agent contract, the
// generated index, the journal) out by construction, including ones added
// later. The index and journal would also collapse the graph into a star,
// since they link to everything.
const graphQuery = "path:research/topics OR path:research/maps OR path:research/sources"

// domainColors gives each domain a stable colour, so a glance at the graph
// tells you which area a cluster belongs to.
var domainColors = []struct {
	query string
	hex   string
}{
	{"path:research/topics/math", "#4EA1F5"},  // синий
	{"path:research/topics/ml", "#F2A65A"},    // оранжевый
	{"path:research/topics/quant", "#6FCF97"}, // зелёный
	{"path:research/topics/cs", "#A78BFA"},    // фиолетовый
	{"path:research/maps", "#FFD166"},         // жёлтый: карты — узлы-хабы
	{"path:research/sources", "#9CA3AF"},      // серый: источники вторичны
}

type graphColor struct {
	A   int `json:"a"`
	RGB int `json:"rgb"`
}

type graphGroup struct {
	Query string     `json:"query"`
	Color graphColor `json:"color"`
}

// graphSettings mirrors Obsidian's graph.json. Unknown keys in the existing
// file are preserved by merging into a generic map, so an Obsidian update
// that adds a field does not lose it.
func graphSettings() map[string]any {
	groups := make([]graphGroup, 0, len(domainColors))
	for _, item := range domainColors {
		rgb, err := strconv.ParseInt(item.hex[1:], 16, 32)
		if err != nil {
			continue
		}
		groups = append(groups, graphGroup{
			Query: item.query,
			Color: graphColor{A: 1, RGB: int(rgb)},
		})
	}

	return map[string]any{
		"search":          graphQuery,
		"colorGroups":     groups,
		"showTags":        false,
		"showAttachments": false,
		// Unresolved links would add ghost nodes for every typo; the linter
		// reports those instead.
		"hideUnresolved": true,
		// Orphans stay visible on purpose: a topic nobody links to is a real
		// finding, not noise.
		"showOrphans": true,
		// Direction matters here — prerequisites point into what they enable.
		"showArrow": true,
		// Labels readable without zooming all the way in.
		"textFadeMultiplier": 1,
		"nodeSizeMultiplier": 1.3,
		"lineSizeMultiplier": 1,
		// Spread clusters apart instead of letting them clump.
		"centerStrength":        0.4,
		"repelStrength":         14,
		"linkStrength":          0.7,
		"linkDistance":          220,
		"scale":                 1,
		"collapse-filter":       false,
		"collapse-color-groups": false,
		"collapse-display":      false,
		"collapse-forces":       false,
		"close":                 false,
	}
}

// WriteGraph applies the learning-graph settings to the vault, backing up
// whatever was there. Returns the settings path and the backup path.
func WriteGraph() (path, backup string, err error) {
	vault, err := wiki.VaultRoot()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(vault, ".obsidian")
	if _, err := os.Stat(dir); err != nil {
		return "", "", fmt.Errorf("вольт не открыт в Obsidian: нет %s", dir)
	}
	path = filepath.Join(dir, "graph.json")

	merged := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &merged)
		backup = fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backup, data, 0o644); err != nil {
			return "", "", fmt.Errorf("бэкап %s: %w", backup, err)
		}
	}
	for key, value := range graphSettings() {
		merged[key] = value
	}

	encoded, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return "", "", fmt.Errorf("запись %s: %w", path, err)
	}
	return path, backup, nil
}
