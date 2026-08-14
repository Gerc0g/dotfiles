package research

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The index is generated, never hand-written: the previous research zone kept
// it by hand, the agent forgot to update it, and it rotted into a lie.

const indexHeader = `# Research — индекс

<!-- Этот файл генерируется: hq research index. Руками не править. -->
`

// BuildIndex renders index.md from the scanned zone.
func BuildIndex(zone *Zone) string {
	var b strings.Builder
	b.WriteString(indexHeader)

	if len(zone.Topics) == 0 {
		b.WriteString("\nПока пусто: конспектов нет.\n")
		return b.String()
	}

	byDomain := map[string][]Topic{}
	for _, topic := range zone.Topics {
		byDomain[topic.Domain] = append(byDomain[topic.Domain], topic)
	}

	for _, domain := range Domains {
		topics := byDomain[domain]
		if len(topics) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n\n", domain)
		for _, topic := range topics {
			line := fmt.Sprintf("- [[%s|%s]] · %s", topic.Slug, topic.DisplayName(), topic.Status)
			if hint := firstLine(stripComments(topic.Summary)); hint != "" {
				line += " — " + hint
			}
			b.WriteString(line + "\n")
		}
	}

	if len(zone.Maps) > 0 {
		b.WriteString("\n## Карты\n\n")
		for _, doc := range zone.Maps {
			fmt.Fprintf(&b, "- [[%s|%s]]\n", doc.Slug, doc.Title)
		}
	}
	if len(zone.Sources) > 0 {
		b.WriteString("\n## Источники\n\n")
		for _, doc := range zone.Sources {
			fmt.Fprintf(&b, "- [[%s|%s]]\n", doc.Slug, doc.Title)
		}
	}

	fmt.Fprintf(&b, "\n<!-- обновлено: %s -->\n", time.Now().Format("2006-01-02 15:04"))
	return b.String()
}

// WriteIndex regenerates index.md in the zone and returns its path.
func WriteIndex(zone *Zone) (string, error) {
	path := filepath.Join(zone.Root, "index.md")
	if err := os.WriteFile(path, []byte(BuildIndex(zone)), 0o644); err != nil {
		return "", fmt.Errorf("запись %s: %w", path, err)
	}
	return path, nil
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// MapCandidate is a tag that grew enough topics to deserve a map.
type MapCandidate struct {
	Tag    string
	Topics []string
}

// mapThreshold is how many topics make a learning route worth drawing.
const mapThreshold = 3

// MapCandidates reports tags with enough topics and no map yet.
func MapCandidates(zone *Zone) []MapCandidate {
	byTag := map[string][]string{}
	for _, topic := range zone.Topics {
		for _, tag := range topic.Topics {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				byTag[tag] = append(byTag[tag], topic.Ref)
			}
		}
	}

	existing := map[string]bool{}
	for _, doc := range zone.Maps {
		existing[doc.Slug] = true
	}

	var out []MapCandidate
	for tag, refs := range byTag {
		if len(refs) >= mapThreshold && !existing[tag] {
			out = append(out, MapCandidate{Tag: tag, Topics: refs})
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i].Topics) > len(out[j].Topics) })
	return out
}
