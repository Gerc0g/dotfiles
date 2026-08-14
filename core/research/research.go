// Package research manages the learning zone of the vault.
//
// The unit is a topic the user studied and understood, not a source that
// arrived. One topic is one growing file: new material extends the existing
// sections instead of spawning a second note. The agent is a scribe and an
// editor — success is a markdown file the user can open half a year later and
// understand without asking anyone.
//
// Everything derived (the index, lint findings, map candidates) is computed
// from the files on demand. Nothing is kept in a side registry: the previous
// research zone died partly because its hand-maintained index rotted.
package research

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Gerc0g/dotfiles/core/wiki"
	"gopkg.in/yaml.v3"
)

// Domains are the fixed top-level areas. Sub-topics (RL, NLP) live in maps
// and tags, never in folders: a folder forces one home per note, while a
// topic legitimately belongs to several learning routes.
var Domains = []string{"ml", "math", "quant", "swe"}

// Frame sections every topic must carry. They serve the machinery — quick
// recall, the graph, verifiability — which is why they are mandatory while
// the body between them is free-form.
const (
	SectionSummary = "В двух словах"
	SectionLinks   = "Связи"
	SectionSources = "Источники"
)

// RequiredSections lists the frame in document order.
var RequiredSections = []string{SectionSummary, SectionLinks, SectionSources}

// Status values for topic maturity.
const (
	StatusGrowing = "growing"
	StatusSolid   = "solid"
)

// Meta is the frontmatter of a topic.
type Meta struct {
	Domain  string   `yaml:"domain"`
	Status  string   `yaml:"status"`
	Aliases []string `yaml:"aliases"`
	Topics  []string `yaml:"topics"`
	Prereqs []string `yaml:"prereqs"`
	Sources []string `yaml:"sources"`
	Updated string   `yaml:"updated"`
}

// Topic is one conspectus.
type Topic struct {
	Meta
	// Path is the file on disk, Ref is "<domain>/<slug>".
	Path string
	Ref  string
	Slug string
	// Title is the H1 heading.
	Title string
	// Summary is the text of the "В двух словах" section.
	Summary string
	// Sections are the H2 headings in document order.
	Sections []string
	// Links are the [[wikilink]] targets found in the body.
	Links []string
	// Body is the document without frontmatter.
	Body string
}

// Doc is a map or source page — lighter than a topic.
type Doc struct {
	Path  string
	Slug  string
	Title string
}

// Zone is a scanned research zone.
type Zone struct {
	Root    string
	Topics  []Topic
	Maps    []Doc
	Sources []Doc
	RawRefs []string
}

// Root reports the research zone directory.
func Root() (string, error) {
	return wiki.ZonePath("research")
}

// Scan reads the whole zone.
func Scan() (*Zone, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("зоны research нет: %s", root)
	}

	zone := &Zone{Root: root}

	for _, domain := range Domains {
		dir := filepath.Join(root, "topics", domain)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			topic, err := readTopic(filepath.Join(dir, entry.Name()), domain)
			if err != nil {
				return nil, err
			}
			zone.Topics = append(zone.Topics, topic)
		}
	}
	sort.Slice(zone.Topics, func(i, j int) bool { return zone.Topics[i].Ref < zone.Topics[j].Ref })

	zone.Maps = readDocs(filepath.Join(root, "maps"))
	zone.Sources = readDocs(filepath.Join(root, "sources"))
	zone.RawRefs = readRaw(filepath.Join(root, "raw"))

	return zone, nil
}

func readDocs(dir string) []Doc {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var docs []Doc
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		slug := strings.TrimSuffix(entry.Name(), ".md")
		docs = append(docs, Doc{Path: path, Slug: slug, Title: firstHeading(path, slug)})
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Slug < docs[j].Slug })
	return docs
}

// readRaw lists source files as vault-relative refs, so links into originals
// can be validated.
func readRaw(dir string) []string {
	var refs []string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() == ".gitkeep" {
			return nil
		}
		rel, relErr := filepath.Rel(filepath.Dir(dir), path)
		if relErr == nil {
			refs = append(refs, rel)
		}
		return nil
	})
	sort.Strings(refs)
	return refs
}

func firstHeading(path, fallback string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return fallback
}

// readTopic parses one conspectus file.
func readTopic(path, domain string) (Topic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Topic{}, fmt.Errorf("чтение %s: %w", path, err)
	}

	slug := strings.TrimSuffix(filepath.Base(path), ".md")
	topic := Topic{
		Path: path,
		Slug: slug,
		Ref:  domain + "/" + slug,
	}

	front, body := splitFrontmatter(string(data))
	topic.Body = body
	if front != "" {
		if err := yaml.Unmarshal([]byte(front), &topic.Meta); err != nil {
			return Topic{}, fmt.Errorf("frontmatter %s: %w", path, err)
		}
	}
	if topic.Domain == "" {
		topic.Domain = domain
	}

	topic.Title = slug
	sections := map[string][]string{}
	current := ""
	for _, line := range strings.Split(body, "\n") {
		switch {
		case strings.HasPrefix(line, "# "):
			topic.Title = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "## "):
			current = strings.TrimSpace(line[3:])
			topic.Sections = append(topic.Sections, current)
		default:
			if current != "" {
				sections[current] = append(sections[current], line)
			}
		}
	}
	topic.Summary = strings.TrimSpace(strings.Join(sections[SectionSummary], "\n"))
	topic.Links = wikiLinks(body)

	return topic, nil
}

// splitFrontmatter separates the YAML header from the document body.
func splitFrontmatter(text string) (front, body string) {
	if !strings.HasPrefix(text, "---\n") {
		return "", text
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return "", text
	}
	front = rest[:end]
	body = strings.TrimPrefix(rest[end+4:], "\n")
	return front, body
}

// SectionText returns the body of one section, or "" when it is absent.
func (t Topic) SectionText(name string) string {
	current := ""
	var lines []string
	for _, line := range strings.Split(t.Body, "\n") {
		if strings.HasPrefix(line, "## ") {
			current = strings.TrimSpace(line[3:])
			continue
		}
		if current == name {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// HasSection reports whether the topic carries a section.
func (t Topic) HasSection(name string) bool {
	for _, section := range t.Sections {
		if section == name {
			return true
		}
	}
	return false
}

// DisplayName is the human title with the slug as fallback.
func (t Topic) DisplayName() string {
	if t.Title != "" {
		return t.Title
	}
	return t.Slug
}

// FindTopic resolves a topic by "<domain>/<slug>", by slug alone, or by alias.
func (z *Zone) FindTopic(name string) (Topic, bool) {
	name = strings.TrimSpace(name)
	for _, topic := range z.Topics {
		if topic.Ref == name || topic.Slug == name {
			return topic, true
		}
	}
	lower := strings.ToLower(name)
	for _, topic := range z.Topics {
		if strings.EqualFold(topic.Title, name) {
			return topic, true
		}
		for _, alias := range topic.Aliases {
			if strings.ToLower(alias) == lower {
				return topic, true
			}
		}
	}
	return Topic{}, false
}
