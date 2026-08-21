package research

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Gerc0g/dotfiles/core/wiki"
)

// Register the zone pulse with the vault hook: a session started in research
// gets the state of the zone on top of the vault checkpoint.
func init() {
	wiki.ZoneContext["research"] = SessionContext
}

// SessionContext is the pulse a research session starts with. The previous
// research zone had none: nothing told the agent what was there, nothing
// nudged when it rotted, and the zone died unnoticed in two months.
func SessionContext() string {
	zone, err := Scan()
	if err != nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("# Research — состояние зоны\n\n")

	if len(zone.Topics) == 0 {
		b.WriteString("Конспектов пока нет. Первая тема заводится через " +
			"`hq research new <домен>/<slug> --title \"Название\"` — домены: " +
			strings.Join(Domains, ", ") + ".\n")
		b.WriteString("\nКонтракт зоны — `AGENTS.md` в корне research: роль писаря, " +
			"формат конспекта, правила связей и цитат.\n")
		return b.String()
	}

	byDomain := map[string]int{}
	solid := 0
	for _, topic := range zone.Topics {
		byDomain[topic.Domain]++
		if topic.Status == StatusSolid {
			solid++
		}
	}
	var parts []string
	for _, domain := range Domains {
		if byDomain[domain] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", domain, byDomain[domain]))
		}
	}
	fmt.Fprintf(&b, "Тем: %d (%s) · solid %d · карт %d · источников %d\n",
		len(zone.Topics), strings.Join(parts, ", "), solid, len(zone.Maps), len(zone.Sources))

	if loose := Unmapped(zone); len(loose) > 0 {
		var names []string
		for i, topic := range loose {
			if i == 5 {
				names = append(names, fmt.Sprintf("и ещё %d", len(loose)-5))
				break
			}
			names = append(names, topic.Slug)
		}
		fmt.Fprintf(&b, "\nВне карт: %s — спроси, вносить ли в маршрут.\n",
			strings.Join(names, ", "))
	}

	if recent := recentTopics(zone, 5); len(recent) > 0 {
		b.WriteString("\nПоследнее, что менялось: " + strings.Join(recent, ", ") + "\n")
	}

	if findings := Lint(zone); len(findings) > 0 {
		errors, warns := 0, 0
		for _, finding := range findings {
			switch finding.Severity {
			case SeverityError:
				errors++
			case SeverityWarn:
				warns++
			}
		}
		fmt.Fprintf(&b, "\nКачество: %d ошибок, %d предупреждений — детали `hq research lint --plain`.\n",
			errors, warns)
		for i, finding := range findings {
			if i >= 3 {
				break
			}
			fmt.Fprintf(&b, "- %s: %s\n", finding.Ref, finding.Message)
		}
	}

	if candidates := MapCandidates(zone); len(candidates) > 0 {
		var tags []string
		for _, candidate := range candidates {
			tags = append(tags, fmt.Sprintf("%s (%d)", candidate.Tag, len(candidate.Topics)))
		}
		b.WriteString("\nПора собрать карты: " + strings.Join(tags, ", ") + ".\n")
	}

	b.WriteString("\nКонтракт зоны — `AGENTS.md` в корне research. " +
		"Пользователь работает только через чат: команды `hq research …` и " +
		"`hq wikipedik sync \"<что сделал>\"` вызываешь ты, не он.\n")
	return b.String()
}

// recentTopics lists the most recently updated topics by their `updated` field.
func recentTopics(zone *Zone, limit int) []string {
	topics := append([]Topic(nil), zone.Topics...)
	sort.SliceStable(topics, func(i, j int) bool { return topics[i].Updated > topics[j].Updated })

	var out []string
	for i, topic := range topics {
		if i >= limit || topic.Updated == "" {
			break
		}
		out = append(out, fmt.Sprintf("%s (%s)", topic.DisplayName(), topic.Updated))
	}
	return out
}
