package wiki

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// hot.md is what SessionStart hooks inject into every agent session. The
// curator LLM is supposed to refresh it during inbox-drain, but that step has
// been skipped by the model before — this rebuild is the guaranteed fallback.

const (
	lessonsLimit   = 3
	gotchasLimit   = 5
	questionsLimit = 5
	rulesLimit     = 10
)

var slugDropRe = regexp.MustCompile(`[^\p{L}\p{N}\s_-]`)
var slugSpaceRe = regexp.MustCompile(`[\s_]+`)

// hotSlugify matches the anchor scheme the python fallback used.
func hotSlugify(title string) string {
	slug := slugDropRe.ReplaceAllString(strings.ToLower(title), "")
	return strings.Trim(slugSpaceRe.ReplaceAllString(slug, "-"), "-")
}

// entryTitles returns the `## ` entry titles of a curated page, oldest first.
func entryTitles(page string) []string {
	data, err := os.ReadFile(page)
	if err != nil {
		return nil
	}
	return entryTitlesContent(string(data))
}

func entryTitlesContent(content string) []string {
	var titles []string
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "## ["):
			// inbox-style heading: "## [date] capture | title"
			if _, after, found := strings.Cut(line, "|"); found {
				titles = append(titles, strings.TrimSpace(after))
			} else {
				titles = append(titles, strings.TrimSpace(line[3:]))
			}
		case strings.HasPrefix(line, "## "):
			titles = append(titles, strings.TrimSpace(line[3:]))
		}
	}
	return titles
}

func hotSection(label, pageName string, titles []string, limit int) []string {
	lines := []string{"## " + label, ""}
	if len(titles) == 0 {
		lines = append(lines, fmt.Sprintf("- No entries in %s yet.", pageName))
	} else {
		start := len(titles) - limit
		if start < 0 {
			start = 0
		}
		recent := titles[start:]
		for i := len(recent) - 1; i >= 0; i-- {
			lines = append(lines, fmt.Sprintf("- %s#%s: %s", pageName, hotSlugify(recent[i]), recent[i]))
		}
	}
	return append(lines, "")
}

var frontmatterRe = regexp.MustCompile(`(?s)^---\n(.*?)\n---`)
var pathsKeyRe = regexp.MustCompile(`(?m)^paths\s*:`)

// hasPathsFrontmatter is the validity bar a rule must clear: a rule that
// would load unconditionally is context bloat by definition. Shared between
// the hot.md digest and rules-sync so the two channels never disagree.
func hasPathsFrontmatter(text string) bool {
	front := frontmatterRe.FindStringSubmatch(text)
	return front != nil && pathsKeyRe.MatchString(front[1])
}

// ruleLines digests binding rules (repo + product level) into one line each.
// This is the codex-facing channel: codex cannot load path-scoped
// .claude/rules, so the digest tells the planner WHAT is binding; Claude
// additionally gets the full rule text lazily via rules-sync symlinks.
func ruleLines(repoMem string) []string {
	var out []string
	sources := []struct{ dir, label string }{
		{filepath.Join(repoMem, "rules"), "rules"},
		{filepath.Join(filepath.Dir(filepath.Dir(repoMem)), "shared", "rules"), "product rules"},
	}

	for _, src := range sources {
		rules, _ := filepath.Glob(filepath.Join(src.dir, "*.md"))
		sort.Strings(rules)
		for _, rule := range rules {
			data, err := os.ReadFile(rule)
			if err != nil {
				continue
			}
			text := string(data)
			if !hasPathsFrontmatter(text) {
				continue
			}

			title := ""
			for _, line := range strings.Split(text, "\n") {
				if strings.HasPrefix(line, "# ") {
					title = strings.TrimSpace(line[2:])
					break
				}
				if strings.HasPrefix(line, "description:") {
					title = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), `"`)
				}
			}
			if title == "" {
				title = strings.TrimSuffix(filepath.Base(rule), ".md")
			}
			out = append(out, fmt.Sprintf("- [%s] %s → %s/%s",
				src.label, title, strings.ReplaceAll(src.label, " ", "/"), filepath.Base(rule)))
		}
	}

	if len(out) > rulesLimit {
		out = out[:rulesLimit]
	}
	return out
}

// BuildHot renders the hot.md content for one repo memory directory.
func BuildHot(repoMem, repo string) string {
	return buildHot(repo, ruleLines(repoMem), entryTitles(filepath.Join(repoMem, "lessons.md")), entryTitles(filepath.Join(repoMem, "gotchas.md")), entryTitles(filepath.Join(repoMem, "open-questions.md")))
}

// BuildHotFromPages renders safely-read scoped memory using the same hot-context formatter.
func BuildHotFromPages(repo string, pages map[string]string) string {
	var rules []string
	names := make([]string, 0, len(pages))
	for name := range pages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !strings.HasPrefix(name, "rules/") && !strings.HasPrefix(name, "product-rules/") {
			continue
		}
		content := pages[name]
		if !hasPathsFrontmatter(content) {
			continue
		}
		label := "rules"
		target := name
		if strings.HasPrefix(name, "product-rules/") {
			label = "product rules"
			target = "product/rules/" + strings.TrimPrefix(name, "product-rules/")
		}
		title := strings.TrimSuffix(filepath.Base(name), ".md")
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(line[2:])
				break
			}
			if strings.HasPrefix(line, "description:") {
				title = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), "\"")
			}
		}
		rules = append(rules, fmt.Sprintf("- [%s] %s → %s", label, title, target))
	}
	if len(rules) > rulesLimit {
		rules = rules[:rulesLimit]
	}
	return buildHot(repo, rules, entryTitlesContent(pages["lessons.md"]), entryTitlesContent(pages["gotchas.md"]), entryTitlesContent(pages["open-questions.md"]))
}

func buildHot(repo string, rules, lessons, gotchas, questions []string) string {
	lines := []string{
		"# " + repo + " hot context",
		"",
		"Auto-refreshed by curator. Loaded by SessionStart hook on every codex/claude session in this repo.",
		"",
	}

	if len(rules) > 0 {
		lines = append(lines, "## Binding rules", "")
		lines = append(lines, rules...)
		lines = append(lines, "")
	}

	lines = append(lines, hotSection("Fresh Lessons", "lessons.md", lessons, lessonsLimit)...)
	lines = append(lines, hotSection("Critical Gotchas", "gotchas.md", gotchas, gotchasLimit)...)
	lines = append(lines, hotSection("Open Questions", "open-questions.md", questions, questionsLimit)...)

	lines = append(lines,
		"## Memory pages (read on demand)",
		"",
		"This file is an index. Full pages live next to it in `docs/knowledge/`:",
		"`lessons.md` · `gotchas.md` · `debugging-stories.md` · `decisions-not-adr.md` · `open-questions.md` · `links.md`.",
		"Product/company layers: `docs/product-knowledge/`, `docs/company-knowledge/`.",
		"",
		fmt.Sprintf("<!-- last refreshed: %s (wiki-hot-refresh) -->", time.Now().Format("2006-01-02 15:04")),
	)
	return strings.Join(lines, "\n") + "\n"
}

// repoMemDirs resolves the repo memory directories a scope covers.
func repoMemDirs(scope Scope) ([]string, error) {
	projects, err := ProjectsRoot()
	if err != nil {
		return nil, err
	}

	if scope.Repo != "" {
		return []string{filepath.Join(projects, scope.Company, scope.Product, "repos", scope.Repo)}, nil
	}

	root := filepath.Join(projects, scope.Company)
	if scope.Product != "" {
		root = filepath.Join(root, scope.Product)
	}
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("scope не найден: %s", root)
	}

	var dirs []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "index.md" {
			return nil
		}
		dir := filepath.Dir(path)
		if filepath.Base(filepath.Dir(dir)) == "repos" {
			dirs = append(dirs, dir)
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs, nil
}

// RefreshHot deterministically rebuilds hot.md for every repo in scope.
func RefreshHot(scope Scope, dryRun bool, out io.Writer) error {
	dirs, err := repoMemDirs(scope)
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		return fmt.Errorf("нет памяти репозиториев в скоупе %s", scope)
	}

	for _, repoMem := range dirs {
		if info, err := os.Stat(repoMem); err != nil || !info.IsDir() {
			fmt.Fprintf(out, "пропуск (нет каталога): %s\n", repoMem)
			continue
		}
		target := filepath.Join(repoMem, "hot.md")
		if dryRun {
			fmt.Fprintf(out, "обновил бы: %s\n", target)
			continue
		}
		content := BuildHot(repoMem, filepath.Base(repoMem))
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return fmt.Errorf("запись %s: %w", target, err)
		}
		fmt.Fprintf(out, "обновлён: %s\n", target)
	}
	return nil
}
