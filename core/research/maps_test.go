package research

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mapZone builds a zone with two conspectuses at different stages.
func mapZone(t *testing.T) *Zone {
	t.Helper()
	root := zoneEnv(t)

	write := func(domain, slug, status string) {
		dir := filepath.Join(root, "topics", domain)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\ndomain: " + domain + "\nstatus: " + status +
			"\naliases: []\n---\n\n# " + slug + "\n\n## В двух словах\n\nтекст\n"
		if err := os.WriteFile(filepath.Join(dir, slug+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ml", "linear-regression", "solid")
	write("ml", "regularization", "growing")

	zone, err := Scan()
	if err != nil {
		t.Fatal(err)
	}
	return zone
}

// Coverage must come from the conspectuses, never from what someone typed into
// the map: a hand-kept status is a status that rots.
func TestMapCoverageIsComputedFromTopics(t *testing.T) {
	zone := mapZone(t)

	path, err := NewMap(zone.Root, "classic-ml", "Классическое ML", "ml")
	if err != nil {
		t.Fatal(err)
	}

	route := `
## Маршрут

### Регрессия

- [[linear-regression]] — базовая постановка
- [[regularization]]
- [[logistic-regression]] — ещё не трогал

### Оптимизация

- [[gradient-descent]]
`
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Replace the template's placeholder route with a real one.
	trimmed := string(body)[:strings.Index(string(body), "## Маршрут")] + strings.TrimPrefix(route, "\n")
	if err := os.WriteFile(path, []byte(trimmed), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RefreshMap(path, zone); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)

	for _, want := range []string{
		"- " + glyphSolid + " [[linear-regression]] — базовая постановка",
		"- " + glyphGrowing + " [[regularization]]",
		"- " + glyphMissing + " [[logistic-regression]] — ещё не трогал",
		"- " + glyphMissing + " [[gradient-descent]]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("нет строки %q:\n%s", want, got)
		}
	}

	if !strings.Contains(got, "закрыто 1 · в работе 1 · не начато 2") {
		t.Errorf("сводка покрытия неверна:\n%s", got)
	}
	if !strings.Contains(got, "Регрессия 1/3") || !strings.Contains(got, "Оптимизация 0/1") {
		t.Errorf("разделы посчитаны неверно:\n%s", got)
	}

	// A second refresh must be a no-op, not a growing pile of glyphs.
	changed, err := RefreshMap(path, zone)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("повторное обновление изменило файл — глифы дублируются")
	}
}

// The route lines are the user's own words about why a topic matters; refresh
// touches the status and nothing else.
func TestMapRefreshKeepsHandWrittenText(t *testing.T) {
	zone := mapZone(t)
	path, err := NewMap(zone.Root, "keep", "Сохранность", "ml")
	if err != nil {
		t.Fatal(err)
	}

	prose := "## Цель\n\nМоя формулировка цели.\n\n## Маршрут\n\n### Раздел\n\n" +
		"- [[linear-regression]] — **важно** потому что база, см. [заметку](x)\n"
	if err := os.WriteFile(path, []byte(prose), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshMap(path, zone); err != nil {
		t.Fatal(err)
	}

	got := read(t, path)
	if !strings.Contains(got, "Моя формулировка цели.") {
		t.Error("обновление съело авторский текст")
	}
	if !strings.Contains(got, "**важно** потому что база, см. [заметку](x)") {
		t.Errorf("обновление испортило строку маршрута:\n%s", got)
	}
}

// Finishing a conspectus is the only way to close a line on the roadmap.
func TestClosingATopicMovesTheRoadmap(t *testing.T) {
	zone := mapZone(t)
	path, err := NewMap(zone.Root, "progress", "Прогресс", "ml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("## Маршрут\n\n### Р\n\n- [[regularization]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshMap(path, zone); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); !strings.Contains(got, glyphGrowing+" [[regularization]]") {
		t.Fatalf("ожидался статус «в работе»:\n%s", got)
	}

	// Promote the conspectus to solid, rescan, refresh.
	topic := filepath.Join(zone.Root, "topics", "ml", "regularization.md")
	body := strings.Replace(read(t, topic), "status: growing", "status: solid", 1)
	if err := os.WriteFile(topic, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rescanned, err := Scan()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshMap(path, rescanned); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); !strings.Contains(got, glyphSolid+" [[regularization]]") {
		t.Errorf("карта не отразила закрытие темы:\n%s", got)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The top-level roadmap lists areas, not topics. A link to another map has to
// carry that area's numbers, or a half-finished area would read as untouched.
func TestTopMapAggregatesAreaMaps(t *testing.T) {
	zone := mapZone(t)

	area, err := NewMap(zone.Root, "dl", "Глубокое обучение", "ml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(area, []byte("## Маршрут\n\n### Основы\n\n"+
		"- [[linear-regression]]\n- [[regularization]]\n- [[transformers]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	empty, err := NewMap(zone.Root, "rl", "Обучение с подкреплением", "ml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(empty, []byte("## Маршрут\n\n### Основы\n\n- [[q-learning]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	top, err := NewMap(zone.Root, "roadmap", "Роадмап", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(top, []byte(coverageStart+"\n"+coverageEnd+
		"\n\n## Маршрут\n\n### Области\n\n- [[dl]] — трансформеры и обучение\n- [[rl]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RefreshMap(top, zone); err != nil {
		t.Fatal(err)
	}
	got := read(t, top)

	// dl: one solid, one growing, one missing → under way, scored 1/3.
	if !strings.Contains(got, glyphPartial+" [[dl]] — трансформеры и обучение · 1/3") {
		t.Errorf("область не подтянула свои числа:\n%s", got)
	}
	// rl: nothing started at all.
	if !strings.Contains(got, glyphMissing+" [[rl]] · 0/1") {
		t.Errorf("пустая область показана неверно:\n%s", got)
	}
	// The top summary sums the areas, not the two lines.
	if !strings.Contains(got, "закрыто 1 · в работе 1 · не начато 2 (всего 4)") {
		t.Errorf("верхняя сводка не сложила области:\n%s", got)
	}

	// Refreshing again must not stack counts like "· 1/3 · 1/3".
	if _, err := RefreshMap(top, zone); err != nil {
		t.Fatal(err)
	}
	if strings.Count(read(t, top), "· 1/3") != 1 {
		t.Errorf("счётчик области продублировался:\n%s", read(t, top))
	}
}
