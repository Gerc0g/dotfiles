package research

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The template is the single source of the topic format. It lives in the core
// rather than as a file in the vault so nobody edits it by accident and the
// frame stays predictable for the linter.
const topicTemplate = `---
domain: %s
status: growing
aliases: [%s]
topics: []
prereqs: []
sources: []
updated: %s
---

# %s

## %s

<!-- 3-5 строк: что это и зачем. Точка входа при перечитывании. -->

<!-- Ниже — свободная часть: интуиция, формально, вывод, примеры, ошибки.
     Порядок от короткого к глубокому. Формулы обязательно в LaTeX:
     $inline$ и $$block$$. -->

## %s

- Предпосылки:
- Дальше:
- Рядом:

## %s

`

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidSlug reports whether a topic file name is acceptable: lowercase
// kebab-case English, so links survive renames of the human title.
func ValidSlug(slug string) bool { return slugRe.MatchString(slug) }

// New creates a topic file from the template and returns its path. The title
// is the Russian heading; the slug stays English.
func New(domain, slug, title string) (string, error) {
	if !validDomain(domain) {
		return "", fmt.Errorf("нет домена %q (есть: %s)", domain, strings.Join(Domains, ", "))
	}
	if !ValidSlug(slug) {
		return "", fmt.Errorf("имя файла — lowercase kebab-case латиницей: %q", slug)
	}
	if title == "" {
		title = slug
	}

	root, err := Root()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, "topics", domain, slug+".md")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("тема уже есть: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("создание %s: %w", filepath.Dir(path), err)
	}

	content := fmt.Sprintf(topicTemplate,
		domain, title, time.Now().Format("2006-01-02"), title,
		SectionSummary, SectionLinks, SectionSources)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("запись %s: %w", path, err)
	}
	return path, nil
}

func validDomain(domain string) bool {
	for _, known := range Domains {
		if known == domain {
			return true
		}
	}
	return false
}
