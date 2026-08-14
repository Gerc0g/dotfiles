package research

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gerc0g/dotfiles/core/wiki"
)

// zoneEnv points the package at a temp vault and returns the research root.
func zoneEnv(t *testing.T) string {
	t.Helper()
	vault := t.TempDir()
	t.Setenv(wiki.RootEnv, vault)

	root := filepath.Join(vault, "research")
	for _, dir := range []string{"topics/ml", "topics/math", "maps", "sources", "raw/ml"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeTopic(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, "topics", rel+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const goodTopic = `---
domain: ml
status: solid
aliases: [Линейная регрессия]
topics: [linear-models]
prereqs: [math/matrices]
sources: [cs229]
updated: 2026-08-14
---

# Линейная регрессия

## В двух словах

Предсказываем число как взвешенную сумму признаков.

## Формально

Модель $\hat{y} = X\beta$, потери $$L(\beta) = \|y - X\beta\|^2$$

## Связи

- Предпосылки: [[matrices]]

## Источники

- [[cs229]], лекция 2, с. 11
`

const matricesTopic = `---
domain: math
status: growing
updated: 2026-08-14
---

# Матрицы

## В двух словах

Таблица чисел, задающая линейное отображение.

## Связи

- Дальше: [[linear-regression]]

## Источники

- собственный вывод
`

func TestScanAndParse(t *testing.T) {
	root := zoneEnv(t)
	writeTopic(t, root, "ml/linear-regression", goodTopic)
	writeTopic(t, root, "math/matrices", matricesTopic)
	if err := os.WriteFile(filepath.Join(root, "sources", "cs229.md"), []byte("# CS229\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	zone, err := Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(zone.Topics) != 2 {
		t.Fatalf("want 2 topics, got %d", len(zone.Topics))
	}

	topic, ok := zone.FindTopic("ml/linear-regression")
	if !ok {
		t.Fatal("topic not found by ref")
	}
	if topic.Title != "Линейная регрессия" || topic.Status != StatusSolid {
		t.Errorf("parsed: title=%q status=%q", topic.Title, topic.Status)
	}
	if len(topic.Prereqs) != 1 || topic.Prereqs[0] != "math/matrices" {
		t.Errorf("prereqs: %v", topic.Prereqs)
	}
	if !strings.HasPrefix(topic.Summary, "Предсказываем") {
		t.Errorf("summary: %q", topic.Summary)
	}

	// Lookup by alias and by bare slug must work — the agent uses both.
	if _, ok := zone.FindTopic("Линейная регрессия"); !ok {
		t.Error("alias lookup failed")
	}
	if _, ok := zone.FindTopic("matrices"); !ok {
		t.Error("slug lookup failed")
	}
}

func TestLintCleanZone(t *testing.T) {
	root := zoneEnv(t)
	writeTopic(t, root, "ml/linear-regression", goodTopic)
	writeTopic(t, root, "math/matrices", matricesTopic)
	if err := os.WriteFile(filepath.Join(root, "sources", "cs229.md"), []byte("# CS229\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	zone, err := Scan()
	if err != nil {
		t.Fatal(err)
	}
	if findings := Lint(zone); len(findings) != 0 {
		t.Errorf("clean zone must lint clean, got: %v", findings)
	}
}

func TestLintCatchesProblems(t *testing.T) {
	root := zoneEnv(t)

	writeTopic(t, root, "ml/broken", `---
domain: ml
status: unknown
updated: 2026-08-14
---

# Сломанная тема

## В двух словах

<!-- ничего -->

## Формально

Формула \frac{a}{b} написана без долларов, и ещё висит одинокий $.

## Связи

- Предпосылки: [[nonexistent-topic]]

## Источники

- какая-то книга
`)

	zone, err := Scan()
	if err != nil {
		t.Fatal(err)
	}

	rules := map[string]bool{}
	for _, finding := range Lint(zone) {
		rules[finding.Rule] = true
	}

	for _, want := range []string{
		"bad-status",         // status: unknown
		"empty-summary",      // только комментарий
		"latex-outside-math", // \frac вне $
		"unbalanced-math",    // одинокий $
		"broken-link",        // [[nonexistent-topic]]
		"vague-citation",     // источник без места
	} {
		if !rules[want] {
			t.Errorf("правило %q не сработало; получено: %v", want, rules)
		}
	}
}

func TestLintDuplicateTitles(t *testing.T) {
	root := zoneEnv(t)
	writeTopic(t, root, "ml/attention", strings.Replace(goodTopic, "# Линейная регрессия", "# Внимание", 1))
	writeTopic(t, root, "math/attention-2", strings.Replace(goodTopic, "# Линейная регрессия", "# Внимание", 1))

	zone, err := Scan()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range Lint(zone) {
		if finding.Rule == "duplicate-topic" {
			found = true
		}
	}
	if !found {
		t.Error("одинаковые заголовки должны ловиться как дубль")
	}
}

func TestNewFromTemplate(t *testing.T) {
	zoneEnv(t)

	path, err := New("ml", "gradient-descent", "Градиентный спуск")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"# Градиентный спуск", "## " + SectionSummary, "## " + SectionLinks, "## " + SectionSources} {
		if !strings.Contains(text, want) {
			t.Errorf("шаблон без %q:\n%s", want, text)
		}
	}

	if _, err := New("ml", "gradient-descent", "Дубль"); err == nil {
		t.Error("повторное создание должно падать")
	}
	if _, err := New("ml", "Градиентный Спуск", "x"); err == nil {
		t.Error("нелатинский slug должен отклоняться")
	}
	if _, err := New("physics", "quarks", "Кварки"); err == nil {
		t.Error("неизвестный домен должен отклоняться")
	}

	// A fresh topic must lint with only the expected "not filled yet" noise.
	zone, err := Scan()
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range Lint(zone) {
		if finding.Severity == SeverityError && finding.Rule != "empty-summary" {
			t.Errorf("свежий шаблон не должен давать ошибку %s: %s", finding.Rule, finding.Message)
		}
	}
}

func TestIndexAndMapCandidates(t *testing.T) {
	root := zoneEnv(t)
	writeTopic(t, root, "ml/linear-regression", goodTopic)
	writeTopic(t, root, "math/matrices", matricesTopic)
	// Three topics sharing a tag → map candidate.
	for _, slug := range []string{"a", "b", "c"} {
		writeTopic(t, root, "ml/"+slug, strings.Replace(goodTopic,
			"topics: [linear-models]", "topics: [rl]", 1))
	}

	zone, err := Scan()
	if err != nil {
		t.Fatal(err)
	}

	index := BuildIndex(zone)
	if !strings.Contains(index, "## ml") || !strings.Contains(index, "Линейная регрессия") {
		t.Errorf("index:\n%s", index)
	}

	candidates := MapCandidates(zone)
	if len(candidates) != 1 || candidates[0].Tag != "rl" {
		t.Errorf("map candidates: %+v", candidates)
	}

	path, err := WriteIndex(zone)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("index.md не записан")
	}
}
