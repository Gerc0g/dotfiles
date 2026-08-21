package research

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A map is the roadmap of an area: what to learn, in what order, and how far
// along it is. Folders stay flat — an area is a folder, and everything below it
// is structure inside one document, where a topic may sit in several places at
// once without lying about where it belongs.
//
// Coverage is computed, never written by hand. A hand-kept status is a status
// that rots: this vault already carried an index that had to be regenerated
// because the agent forgot it, and a backlog whose entries described problems
// that no longer existed. Here the only way to mark a topic closed is to
// actually finish its conspectus.

// glyphPartial marks a link to another map that is under way: a route whose
// own coverage is neither empty nor complete.
const glyphPartial = "◑"

// Coverage glyphs, in the order a topic travels.
const (
	glyphSolid   = "✓"
	glyphGrowing = "◐"
	glyphMissing = "·"
)

const (
	coverageStart = "<!-- coverage:start -->"
	coverageEnd   = "<!-- coverage:end -->"
)

// mapLinkRe finds the first wikilink on a route line.
var mapLinkRe = regexp.MustCompile(`\[\[([^\]|#]+)`)

// mapGlyphRe matches a glyph the previous refresh left after the bullet.
var mapGlyphRe = regexp.MustCompile(`^(\s*[-*]\s+)(` + glyphSolid + `|` + glyphGrowing + `|` + glyphPartial + `|` + glyphMissing + `)\s+`)

// mapCountRe matches the generated area count at the end of a line.
var mapCountRe = regexp.MustCompile(`\s*·\s*\d+/\d+\s*$`)

// MapsDir is where roadmaps live.
func MapsDir(root string) string { return filepath.Join(root, "maps") }

// MapTemplate is the frame of a roadmap. Like the topic template it lives in
// the core, so every map is scannable at a glance instead of being invented
// again each time.
func MapTemplate(title, domain string) string {
	return fmt.Sprintf(`---
kind: map
domain: %s
updated: %s
---

# %s

## Цель

<!-- Зачем эта область и к чему хочу прийти. Две-три строки. -->

%s
%s

## Маршрут

<!-- Разделы и подразделы — обычными заголовками и вложенными списками.
     Каждый пункт со ссылкой [[тема]]; статус проставляется сам по конспекту.
     Ссылка на ещё не изученное — это нормально, тут она не ошибка. -->

### Раздел

- [[тема]] — чем важна

## Пробелы

<!-- Честно: что не изучал и почему. Растёт по ходу — из «Открытых вопросов»
     конспектов. -->

## Источники

`, domain, time.Now().Format("2006-01-02"), title, coverageStart, coverageEnd)
}

// MapFile is one roadmap on disk.
type MapFile struct {
	Path string
	Slug string
}

// ScanMaps lists the roadmaps of the zone.
func ScanMaps(root string) []MapFile {
	entries, err := os.ReadDir(MapsDir(root))
	if err != nil {
		return nil
	}
	var maps []MapFile
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "_") {
			continue
		}
		maps = append(maps, MapFile{
			Path: filepath.Join(MapsDir(root), name),
			Slug: strings.TrimSuffix(name, ".md"),
		})
	}
	return maps
}

// Coverage is how far one route has come.
type Coverage struct {
	Section string
	Solid   int
	Growing int
	Missing int
}

// Total is how many topics the section plans.
func (c Coverage) Total() int { return c.Solid + c.Growing + c.Missing }

// Bar renders progress: full for finished, half for started.
func (c Coverage) Bar(width int) string {
	if c.Total() == 0 {
		return strings.Repeat("░", width)
	}
	done := c.Solid * width / c.Total()
	part := (c.Solid+c.Growing)*width/c.Total() - done
	return strings.Repeat("▓", done) + strings.Repeat("▒", part) +
		strings.Repeat("░", width-done-part)
}

// MapCoverage is one map's own totals, so a map above it can quote them
// instead of showing a whole area as untouched.
func MapCoverage(path string, zone *Zone) Coverage {
	data, err := os.ReadFile(path)
	if err != nil {
		return Coverage{}
	}
	total := Coverage{Section: mapSlug(path)}
	for _, line := range strings.Split(string(data), "\n") {
		match := mapLinkRe.FindStringSubmatch(line)
		if match == nil || !isListItem(line) {
			continue
		}
		// Only topic links count: a nested map would need its own read, and a
		// roadmap of roadmaps is two levels, not a recursion.
		if topic, ok := zone.FindTopic(strings.TrimSpace(match[1])); ok {
			if topic.Status == "solid" {
				total.Solid++
			} else {
				total.Growing++
			}
			continue
		}
		if !isMapLink(zone.Root, match[1]) {
			total.Missing++
		}
	}
	return total
}

func mapSlug(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

// isMapLink reports whether a wikilink points at another map.
func isMapLink(root, link string) bool {
	name := strings.TrimSpace(link)
	name = strings.TrimPrefix(name, "maps/")
	_, err := os.Stat(filepath.Join(MapsDir(root), name+".md"))
	return err == nil
}

// RefreshMap rewrites the status glyphs and the coverage block of one map, and
// reports whether anything changed.
//
// Only the glyph is touched: the rest of a line is the user's own words about
// why a topic matters, and regeneration must never eat them.
func RefreshMap(path string, zone *Zone) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	original := string(data)

	lines := strings.Split(original, "\n")
	var section string
	var sections []Coverage
	index := map[string]int{}

	for i, line := range lines {
		if heading := strings.TrimSpace(line); strings.HasPrefix(heading, "###") {
			section = strings.TrimSpace(strings.TrimLeft(heading, "# "))
			continue
		}
		match := mapLinkRe.FindStringSubmatch(line)
		if match == nil || !isListItem(line) {
			continue
		}

		link := strings.TrimSpace(match[1])
		glyph := glyphMissing
		var nested *Coverage

		if topic, ok := zone.FindTopic(link); ok {
			if topic.Status == "solid" {
				glyph = glyphSolid
			} else {
				glyph = glyphGrowing
			}
		} else if isMapLink(zone.Root, link) && !samePath(path, zone.Root, link) {
			// A link to another map stands for a whole area, so it carries that
			// area's numbers rather than counting as one unstarted item.
			area := MapCoverage(filepath.Join(MapsDir(zone.Root),
				strings.TrimPrefix(link, "maps/")+".md"), zone)
			nested = &area
			switch {
			case area.Total() == 0:
				glyph = glyphMissing
			case area.Solid == area.Total():
				glyph = glyphSolid
			case area.Solid+area.Growing == 0:
				glyph = glyphMissing
			default:
				glyph = glyphPartial
			}
		}

		lines[i] = setGlyph(line, glyph)
		if nested != nil {
			lines[i] = appendAreaCount(lines[i], *nested)
		} else {
			// A line that used to point at a map keeps its old score otherwise,
			// and a deleted area would go on reporting numbers it no longer has.
			lines[i] = mapCountRe.ReplaceAllString(lines[i], "")
		}

		name := section
		if name == "" {
			name = "Без раздела"
		}
		at, ok := index[name]
		if !ok {
			at = len(sections)
			index[name] = at
			sections = append(sections, Coverage{Section: name})
		}
		if nested != nil {
			sections[at].Solid += nested.Solid
			sections[at].Growing += nested.Growing
			sections[at].Missing += nested.Missing
			continue
		}
		switch glyph {
		case glyphSolid:
			sections[at].Solid++
		case glyphGrowing:
			sections[at].Growing++
		default:
			sections[at].Missing++
		}
	}

	updated := replaceCoverage(strings.Join(lines, "\n"), sections)
	if updated == original {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(updated), 0o644)
}

// RefreshMaps refreshes every map and returns how many changed.
func RefreshMaps(zone *Zone) (int, error) {
	changed := 0
	for _, m := range ScanMaps(zone.Root) {
		did, err := RefreshMap(m.Path, zone)
		if err != nil {
			return changed, err
		}
		if did {
			changed++
		}
	}
	return changed, nil
}

// appendAreaCount puts an area's score at the end of the line, replacing the
// one a previous refresh left there.
func appendAreaCount(line string, area Coverage) string {
	line = mapCountRe.ReplaceAllString(line, "")
	return fmt.Sprintf("%s · %d/%d", strings.TrimRight(line, " "), area.Solid, area.Total())
}

// samePath guards against a map that links to itself.
func samePath(path, root, link string) bool {
	return mapSlug(path) == strings.TrimPrefix(strings.TrimSpace(link), "maps/")
}

func isListItem(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")
}

// setGlyph replaces the status marker of a list line, leaving its text alone.
func setGlyph(line, glyph string) string {
	if match := mapGlyphRe.FindStringSubmatch(line); match != nil {
		return match[1] + glyph + " " + line[len(match[0]):]
	}
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	bullet := trimmed[:2] // "- " or "* "
	return indent + bullet + glyph + " " + trimmed[2:]
}

// replaceCoverage fills the generated block, or leaves the file alone when it
// carries no markers — a map the user has not opted into keeps its own shape.
func replaceCoverage(body string, sections []Coverage) string {
	from := strings.Index(body, coverageStart)
	to := strings.Index(body, coverageEnd)
	if from < 0 || to < from {
		return body
	}
	return body[:from+len(coverageStart)] + "\n" + renderCoverage(sections) + body[to:]
}

func renderCoverage(sections []Coverage) string {
	var total Coverage
	for _, s := range sections {
		total.Solid += s.Solid
		total.Growing += s.Growing
		total.Missing += s.Missing
	}
	if total.Total() == 0 {
		return "\nМаршрут пуст: добавь пункты со ссылками на темы.\n\n"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n**Покрытие:** %s  закрыто %d · в работе %d · не начато %d (всего %d)\n\n",
		total.Bar(12), total.Solid, total.Growing, total.Missing, total.Total())
	for _, s := range sections {
		fmt.Fprintf(&b, "- %s %s %d/%d\n", s.Bar(10), s.Section, s.Solid, s.Total())
	}
	b.WriteString("\n")
	return b.String()
}

// NewMap creates a roadmap from the template. It refuses to overwrite: a map
// carries hand-written intent, and regenerating one would erase the thinking
// that is the whole point of it.
func NewMap(root, slug, title, domain string) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", fmt.Errorf("нужен slug карты")
	}
	if title == "" {
		title = slug
	}

	if err := os.MkdirAll(MapsDir(root), 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(MapsDir(root), slug+".md")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("карта уже есть: %s", path)
	}
	if err := os.WriteFile(path, []byte(MapTemplate(title, domain)), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// MappedNames collects every topic name the maps plan for.
func MappedNames(root string) map[string]bool {
	planned := map[string]bool{}
	for _, m := range ScanMaps(root) {
		data, err := os.ReadFile(m.Path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !isListItem(line) {
				continue
			}
			if match := mapLinkRe.FindStringSubmatch(line); match != nil {
				planned[strings.TrimSpace(match[1])] = true
			}
		}
	}
	return planned
}

// Unmapped lists conspectuses no map plans for.
//
// The roadmap deliberately does not grow by itself: a plan that swallows every
// note becomes the index, and coverage stops meaning anything when its
// denominator grows with every file. So a topic written outside the plan stays
// outside it — and is reported, so the choice to adopt it is made rather than
// missed.
func Unmapped(zone *Zone) []Topic {
	planned := MappedNames(zone.Root)
	if len(planned) == 0 {
		return nil
	}

	var loose []Topic
	for _, topic := range zone.Topics {
		if planned[topic.Ref] || planned[topic.Slug] || planned[topic.Title] {
			continue
		}
		claimed := false
		for _, alias := range topic.Aliases {
			if planned[alias] {
				claimed = true
				break
			}
		}
		if !claimed {
			loose = append(loose, topic)
		}
	}
	return loose
}

// NextItem is one thing to study, with the area it belongs to.
type NextItem struct {
	Area  string
	Link  string
	Note  string
	Order int
}

// Next returns what to study next, in the order the routes are written.
//
// A map is a reference to a whole field: four of them here plan 260 topics, and
// a wall of 260 unstarted items paralyses rather than guides. The route is
// already ordered by prerequisite, so "next" is simply the first unstarted item
// — the queue the roadmap could not be.
func Next(zone *Zone, area string, limit int) []NextItem {
	var found []NextItem
	for _, m := range ScanMaps(zone.Root) {
		if m.Slug == "roadmap" || (area != "" && m.Slug != area) {
			continue
		}
		data, err := os.ReadFile(m.Path)
		if err != nil {
			continue
		}

		order := 0
		perArea := 0
		for _, line := range strings.Split(string(data), "\n") {
			if !isListItem(line) {
				continue
			}
			match := mapLinkRe.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			order++
			link := strings.TrimSpace(match[1])
			if _, done := zone.FindTopic(link); done {
				continue
			}
			if isMapLink(zone.Root, link) {
				continue
			}

			found = append(found, NextItem{
				Area: m.Slug, Link: displayLink(link), Note: itemNote(line), Order: order,
			})
			perArea++
			// Without an area, one line each is enough to choose between them.
			if area == "" || perArea >= limit {
				break
			}
		}
	}
	return found
}

// displayLink strips the alias part of a wikilink.
func displayLink(link string) string {
	if at := strings.Index(link, "|"); at >= 0 {
		return strings.TrimSpace(link[:at])
	}
	return link
}

// itemNote is the user's words after the link, which say why the item matters.
func itemNote(line string) string {
	if at := strings.Index(line, "]]"); at >= 0 {
		note := strings.TrimSpace(line[at+2:])
		note = strings.TrimPrefix(note, "—")
		return strings.TrimSpace(note)
	}
	return ""
}
