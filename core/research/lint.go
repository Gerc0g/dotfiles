package research

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// The linter guards one thing: a conspectus must stay readable on its own.
// It checks the frame, the links, the citations and the maths rendering —
// never the free-form body, which is the author's business.

// Severity of a finding.
type Severity string

const (
	SeverityError = Severity("error")
	SeverityWarn  = Severity("warn")
	SeverityInfo  = Severity("info")
)

// Finding is one lint result.
type Finding struct {
	Severity Severity
	// Rule is the machine-readable slug, e.g. "broken-link".
	Rule string
	// Ref is the topic (or file) the finding belongs to.
	Ref     string
	Message string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s\t%s\t%s\t%s", f.Severity, f.Rule, f.Ref, f.Message)
}

// summaryMaxLines keeps the recall entry an entry, not a second article.
const summaryMaxLines = 12

// latexCommandRe spots LaTeX written outside math delimiters — the formula is
// there but Obsidian will render it as plain text.
var latexCommandRe = regexp.MustCompile(`\\(frac|sum|int|prod|sqrt|hat|bar|vec|alpha|beta|gamma|delta|theta|lambda|sigma|mu|partial|nabla|cdot|times|approx|leq|geq|neq|in|forall|exists|mathbb|mathcal|text)\b`)

// citationRe matches a reference with a concrete location: page, section,
// chapter, lecture, timestamp, equation.
var citationRe = regexp.MustCompile(`(?i)(#page=|с\.\s*\d|стр\.?\s*\d|§|раздел|глава|лекци|мин\.|\d+:\d{2}|ур\.\s*\(|eq\.?\s*\(|p\.\s*\d|ch\.\s*\d)`)

// Lint checks every topic in the zone.
func Lint(zone *Zone) []Finding {
	var findings []Finding

	titles := map[string][]string{}

	for _, topic := range zone.Topics {
		findings = append(findings, lintTopic(zone, topic)...)

		key := strings.ToLower(topic.DisplayName())
		titles[key] = append(titles[key], topic.Ref)
		for _, alias := range topic.Aliases {
			alias = strings.ToLower(strings.TrimSpace(alias))
			if alias != "" && alias != key {
				titles[alias] = append(titles[alias], topic.Ref)
			}
		}
	}

	// Duplicates: the same human name reachable from two files.
	var names []string
	for name := range titles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		refs := titles[name]
		if len(refs) > 1 {
			findings = append(findings, Finding{
				Severity: SeverityWarn,
				Rule:     "duplicate-topic",
				Ref:      strings.Join(refs, ", "),
				Message:  fmt.Sprintf("одно название %q ведёт в разные файлы — свести в одну тему", name),
			})
		}
	}

	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
	})
	return findings
}

func severityRank(s Severity) int {
	switch s {
	case SeverityError:
		return 0
	case SeverityWarn:
		return 1
	default:
		return 2
	}
}

func lintTopic(zone *Zone, topic Topic) []Finding {
	var findings []Finding
	add := func(severity Severity, rule, format string, args ...any) {
		findings = append(findings, Finding{
			Severity: severity, Rule: rule, Ref: topic.Ref,
			Message: fmt.Sprintf(format, args...),
		})
	}

	// Frame.
	for _, section := range RequiredSections {
		if !topic.HasSection(section) {
			add(SeverityError, "missing-section", "нет обязательной секции «%s»", section)
		}
	}
	if topic.Status != StatusGrowing && topic.Status != StatusSolid {
		add(SeverityError, "bad-status", "status должен быть growing или solid, сейчас %q", topic.Status)
	}
	if topic.Title == topic.Slug {
		add(SeverityWarn, "no-title", "нет русского заголовка H1 — файл читается как slug")
	}

	// Recall entry.
	switch {
	case topic.HasSection(SectionSummary) && stripComments(topic.Summary) == "":
		add(SeverityError, "empty-summary", "«%s» пустая — без неё нечего перечитывать", SectionSummary)
	case len(strings.Split(stripComments(topic.Summary), "\n")) > summaryMaxLines:
		add(SeverityWarn, "bloated-summary", "«%s» разрослась (>%d строк) — перенеси детали ниже",
			SectionSummary, summaryMaxLines)
	}

	// Maths rendering.
	body := stripCodeBlocks(topic.Body)
	if outside := latexOutsideMath(body); outside != "" {
		add(SeverityError, "latex-outside-math",
			"LaTeX вне формульных долларов (%s) — Obsidian покажет это текстом", outside)
	}
	if unbalancedMath(body) {
		add(SeverityError, "unbalanced-math", "нечётное число $ — формула не отрендерится")
	}

	// Verifiability.
	sourcesText := stripComments(topic.SectionText(SectionSources))
	if sourcesText == "" && len(topic.Sources) == 0 {
		add(SeverityWarn, "no-sources", "нет источников — укажи материал или пометь как собственный вывод")
	}
	if sourcesText != "" && !citationRe.MatchString(sourcesText) &&
		!strings.Contains(strings.ToLower(sourcesText), "собственн") {
		add(SeverityInfo, "vague-citation",
			"в источниках нет точного места (страница/раздел/лекция/таймкод)")
	}

	if topic.DeclaredDomain != "" && topic.DeclaredDomain != topic.Domain {
		add(SeverityError, "domain-mismatch",
			"в шапке domain: %s, а файл лежит в %s — правит путь, шапку поправь или убери",
			topic.DeclaredDomain, topic.Domain)
	}

	// Graph.
	for _, link := range topic.Links {
		if !zone.Resolve(link) {
			add(SeverityError, "broken-link", "ссылка в никуда: [[%s]]", link)
		}
	}
	for _, prereq := range topic.Prereqs {
		if _, ok := zone.FindTopic(prereq); !ok {
			add(SeverityError, "broken-prereq", "prereqs указывает на несуществующую тему: %s", prereq)
		}
	}

	return findings
}

// stripComments removes HTML comments — template hints must not count as
// content.
var commentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

func stripComments(text string) string {
	return strings.TrimSpace(commentRe.ReplaceAllString(text, ""))
}

// stripCodeBlocks removes fenced code, where LaTeX-looking text is legitimate.
func stripCodeBlocks(body string) string {
	var out []string
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// latexOutsideMath returns the first LaTeX command found outside $…$/$$…$$,
// or "" when the maths is properly delimited.
func latexOutsideMath(body string) string {
	stripped := mathRe.ReplaceAllString(body, " ")
	if match := latexCommandRe.FindString(stripped); match != "" {
		return match
	}
	return ""
}

// mathRe matches block and inline maths.
var mathRe = regexp.MustCompile(`(?s)\$\$.*?\$\$|\$[^$\n]+\$`)

// unbalancedMath reports an odd number of dollar signs left after removing
// well-formed maths — a broken formula in Obsidian.
func unbalancedMath(body string) bool {
	stripped := mathRe.ReplaceAllString(body, "")
	return strings.Count(stripped, "$")%2 == 1
}
