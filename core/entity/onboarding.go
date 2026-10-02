package entity

import (
	"regexp"
	"strings"
)

type definition struct {
	id, title, description string
	headings               []string
	automatic              bool
}

func definitions(kind string) []definition {
	switch kind {
	case "company":
		return []definition{
			{"coordinates", "Координаты", "Компания зарегистрирована в HQ.", nil, true},
			{"workflow", "Правила работы", "Stance, Git workflow и Stack overrides.", []string{"stance", "git workflow", "stack overrides"}, false},
			{"network", "Сеть", "Строка Network в Coordinates: VPN, DNS и ограничения.", []string{"coordinates"}, false},
			{"data", "Данные и секреты", "Secrets: чувствительные данные и правила обращения с ними.", []string{"secrets"}, false},
			{"references", "Ссылки", "External references: документация и рабочие ссылки либо явное отсутствие.", []string{"external references"}, false},
		}
	case "product":
		return []definition{
			{"coordinates", "Координаты", "Продукт зарегистрирован в HQ.", nil, true},
			{"purpose", "Назначение и статус", "What this is: назначение, пользователи и подтверждённый статус.", []string{"what this is"}, false},
			{"repositories", "Репозитории и роли", "Repos and roles: актуальный состав продукта.", []string{"repos and roles"}, false},
			{"contracts", "Связи и контракты", "Cross-repo conventions и Data flow, если поток данных требует описания.", []string{"cross-repo conventions", "data flow"}, false},
			{"boundaries", "Границы изменений", "Boundaries: ограничения для работы агентов.", []string{"boundaries"}, false},
		}
	default:
		return []definition{
			{"git", "Git-репозиторий", "Локальный checkout доступен Git.", nil, true},
			{"purpose", "Назначение", "What this repo does: роль репозитория.", []string{"what this repo does"}, false},
			{"commands", "Код и команды", "Commands, Stack и Key files: реальные команды и структура.", []string{"commands", "stack", "key files"}, false},
			{"environment", "Окружение", "Local dev и Env contract: требования без значений секретов.", []string{"local dev", "env contract"}, false},
			{"boundaries", "Правила и границы", "Boundaries и Non-obvious patterns.", []string{"boundaries", "non-obvious patterns"}, false},
		}
	}
}

func normalizeHeading(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.Index(s, " ("); i >= 0 {
		s = s[:i]
	}
	switch s {
	case "local development":
		return "local dev"
	case "stack defaults":
		return "stack overrides"
	case "secrets & sensitive data":
		return "secrets"
	case "conventions":
		return "non-obvious patterns"
	}
	return s
}

// Headings inside fenced examples are not document sections.
func sections(content string) map[string]string {
	out := map[string]string{}
	current := ""
	fence := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			if current != "" {
				out[current] += line + "\n"
			}
			continue
		}
		if fence == "" && strings.HasPrefix(line, "## ") {
			current = normalizeHeading(strings.TrimPrefix(line, "## "))
			if _, ok := out[current]; !ok {
				out[current] = ""
			}
			continue
		}
		if current != "" {
			out[current] += line + "\n"
		}
	}
	return out
}

// Unknown facts are placeholders when used as field values, not when those
// words occur in ordinary prose (for example, "rejects unknown fields").
var templatePlaceholder = regexp.MustCompile(`(?i)\b(TODO|TBD)\b|<(repo|product|ns|cmd|path|version|port|var_\d+|one-liner|top 3-5|if any|Python/Go/TS/\.\.\.|install|dev cmd|or "none"|rule|lint && test && typecheck)>`)
var unknownField = regexp.MustCompile(`(?im)^\s*(?:[-*]\s+)?(?:[^\n:]+:\s*)?(?:unknown|not found in repo|no data)[.!]?\s*$`)
var markdownFormatting = strings.NewReplacer("*", "", "_", "", "`", "", "~", "")
var comments = regexp.MustCompile(`(?s)<!--.*?-->`)

func hasPlaceholder(value string) bool {
	return templatePlaceholder.MatchString(value) || unknownField.MatchString(markdownFormatting.Replace(value))
}

func itemContent(s snapshot, d definition) string {
	all := sections(string(s.content))
	var values []string
	for _, heading := range d.headings {
		text, ok := all[heading]
		// A simple product may omit the optional data-flow section entirely.
		if !ok && s.kind == "product" && heading == "data flow" {
			continue
		}
		if !ok {
			return ""
		}
		if s.kind == "company" && d.id == "network" {
			text = ""
			for _, line := range strings.Split(all[heading], "\n") {
				plain := strings.ReplaceAll(line, "*", "")
				plain = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(plain), "-"))
				if strings.HasPrefix(strings.ToLower(plain), "network:") {
					text = strings.TrimSpace(plain[len("network:"):])
					break
				}
			}
		}
		if strings.TrimSpace(comments.ReplaceAllString(text, "")) == "" {
			return ""
		}
		values = append(values, strings.TrimSpace(text))
	}
	return strings.Join(values, "\n\n")
}

func progress(s snapshot) Onboarding {
	out := Onboarding{Status: "not_started", Items: []Item{}}
	manualPresent := 0
	for _, d := range definitions(s.kind) {
		item := Item{ID: d.id, Title: d.title, Description: d.description, Automatic: d.automatic, Status: "missing"}
		if d.automatic {
			if d.id != "git" || validGit(s.path) {
				item.Status = "ready"
			}
		} else {
			value := itemContent(s, d)
			if value != "" && !hasPlaceholder(value) {
				item.Status = "review"
				manualPresent++
				if s.state.Confirmed[d.id] == digest(value) {
					item.Status = "ready"
				}
			}
		}
		if item.Status == "ready" {
			out.Completed++
		}
		out.Items = append(out.Items, item)
	}
	out.Total = len(out.Items)
	if out.Completed == out.Total {
		out.Status = "complete"
	} else if manualPresent > 0 || len(s.state.History) > 0 {
		out.Status = "in_progress"
	}
	return out
}
