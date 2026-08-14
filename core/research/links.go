package research

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Obsidian wikilinks: [[target]], [[target|подпись]], [[target#раздел]],
// [[raw/ml/paper.pdf#page=4|Автор 2017, §3.2]].
var wikiLinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// wikiLinks extracts link targets from a document body, stripping the display
// alias and the anchor.
func wikiLinks(body string) []string {
	var out []string
	seen := map[string]bool{}
	for _, match := range wikiLinkRe.FindAllStringSubmatch(body, -1) {
		target := LinkTarget(match[1])
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

// LinkTarget normalises one wikilink payload to its target path.
func LinkTarget(raw string) string {
	target := raw
	if pipe := strings.Index(target, "|"); pipe != -1 {
		target = target[:pipe]
	}
	if hash := strings.Index(target, "#"); hash != -1 {
		target = target[:hash]
	}
	return strings.TrimSpace(target)
}

// Resolve reports whether a link target points at something real in the zone:
// a topic (by ref, slug, title or alias), a map, a source note, or a file
// under raw/.
func (z *Zone) Resolve(target string) bool {
	if target == "" {
		return false
	}
	if _, ok := z.FindTopic(target); ok {
		return true
	}
	// Vault-style paths: topics/ml/x, maps/ml, sources/x, raw/ml/x.pdf.
	trimmed := strings.TrimSuffix(target, ".md")
	base := filepath.Base(trimmed)

	for _, doc := range z.Maps {
		if doc.Slug == base {
			return true
		}
	}
	for _, doc := range z.Sources {
		if doc.Slug == base {
			return true
		}
	}
	for _, ref := range z.RawRefs {
		if ref == target || filepath.Base(ref) == filepath.Base(target) {
			return true
		}
	}
	// A topic referenced as topics/<domain>/<slug>.
	if strings.HasPrefix(trimmed, "topics/") {
		if _, ok := z.FindTopic(strings.TrimPrefix(trimmed, "topics/")); ok {
			return true
		}
	}
	return false
}
