package routine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// An agent routine that nobody bounded is the failure this package exists to
// prevent: it spends the subscription unattended and nothing says stop.
func TestAgentRoutineAlwaysHasACeiling(t *testing.T) {
	agent := Routine{Name: "x", Kind: KindAgent}
	if agent.Ceiling() <= 0 {
		t.Error("агентская routine без потолка — так и жгут подписку молча")
	}
	internal := Routine{Name: "y", Kind: KindInternal}
	if internal.Ceiling() != 0 {
		t.Error("внутренней работе потолок не нужен")
	}
	if got := (Routine{Name: "z", Kind: KindAgent, MaxRunsPerDay: 2}).Ceiling(); got != 2 {
		t.Errorf("явный потолок = %d, want 2", got)
	}
}

func TestGuardsStopTooSoonAndTooMuch(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	r := Routine{Name: "drain", Kind: KindAgent, MinInterval: time.Hour, MaxRunsPerDay: 2}

	if d := Allowed(home, r, false, now); !d.Run {
		t.Fatalf("первый запуск отклонён: %s", d.Reason)
	}

	// Ran a minute ago: too soon.
	must(t, SaveStatus(home, r.Name, Status{
		Last: now.Add(-time.Minute), RunsDay: 1, DayStamp: now.Format("2006-01-02")}))
	if d := Allowed(home, r, false, now); d.Run {
		t.Error("интервальный гвард пропустил слишком ранний запуск")
	}
	if d := Allowed(home, r, true, now); !d.Run {
		t.Error("--force обязан обходить гварды")
	}

	// Yesterday's counter must not hold today back.
	must(t, SaveStatus(home, r.Name, Status{
		Last: now.Add(-48 * time.Hour), RunsDay: 9, DayStamp: "2000-01-01"}))
	if d := Allowed(home, r, false, now); !d.Run {
		t.Errorf("вчерашний счётчик заблокировал сегодня: %s", d.Reason)
	}

	// Today's budget spent.
	must(t, SaveStatus(home, r.Name, Status{
		Last: now.Add(-3 * time.Hour), RunsDay: 2, DayStamp: now.Format("2006-01-02")}))
	if d := Allowed(home, r, false, now); d.Run {
		t.Error("суточный потолок не сработал")
	}

	must(t, SetDisabled(home, r.Name, true))
	if d := Allowed(home, r, true, now); d.Run {
		t.Error("выключенная routine не должна запускаться даже принудительно")
	}
}

func TestRunRecordsOutcomeAndJournal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var out bytes.Buffer
	ok := Routine{Name: "ok-job", Kind: KindInternal, Exec: []string{"true"}}
	if err := Run(home, ok, false, &out); err != nil {
		t.Fatal(err)
	}
	status := LoadState(home)["ok-job"]
	if status.Outcome != OutcomeOK || status.Runs != 1 || status.Last.IsZero() {
		t.Fatalf("успешный прогон записан как %+v", status)
	}

	bad := Routine{Name: "bad-job", Kind: KindInternal, Exec: []string{"false"}}
	if err := Run(home, bad, false, &out); err == nil {
		t.Error("упавшая задача обязана вернуть ошибку")
	}
	if got := LoadState(home)["bad-job"].Outcome; got != OutcomeFailed {
		t.Errorf("исход = %q, want failed", got)
	}

	journal := strings.Join(Tail(home, "", 0), "\n")
	for _, want := range []string{"ok-job", "bad-job", "готово", "ошибка"} {
		if !strings.Contains(journal, want) {
			t.Errorf("в журнале нет %q:\n%s", want, journal)
		}
	}
	if only := Tail(home, "ok-job", 0); len(only) == 0 || strings.Contains(strings.Join(only, "\n"), "bad-job") {
		t.Error("фильтр журнала по имени не работает")
	}
}

// The user's own routines are the point of the entity: the platform supplies
// the schedule, the guards and the record, the user supplies the intent.
func TestUserRoutinesLoadAndAreGuarded(t *testing.T) {
	home := t.TempDir()
	dotfiles := filepath.Join(home, "dotfiles")
	config := UserConfigPath(home)
	must(t, os.MkdirAll(filepath.Dir(config), 0o755))
	must(t, os.WriteFile(config, []byte(`
- name: calendar-digest
  about: утренний разбор календаря
  at: "08:00"
  prompt: посмотри календарь на сегодня
  dir: ~/Desktop
`), 0o644))

	all, err := Load(home, dotfiles)
	if err != nil {
		t.Fatal(err)
	}
	var found *Routine
	for i := range all {
		if all[i].Name == "calendar-digest" {
			found = &all[i]
		}
	}
	if found == nil {
		t.Fatal("пользовательская routine не загрузилась")
	}
	if found.Kind != KindAgent {
		t.Error("routine с промптом обязана считаться агентской")
	}
	if found.Ceiling() <= 0 {
		t.Error("пользовательская агентская routine осталась без потолка")
	}
	if found.Source == "" {
		t.Error("не видно, откуда взялась routine")
	}
}

// A routine the user believes exists but which silently never runs is exactly
// the failure mode this package is against, so a broken file must be loud.
func TestBrokenUserConfigIsReported(t *testing.T) {
	home := t.TempDir()
	config := UserConfigPath(home)
	must(t, os.MkdirAll(filepath.Dir(config), 0o755))

	for _, body := range []string{
		"- name: x\n  every: не-время\n  prompt: y\n",
		"- name: memory-drain\n  every: 1h\n  prompt: y\n", // collides with a builtin
		"- about: без имени\n  every: 1h\n",
	} {
		must(t, os.WriteFile(config, []byte(body), 0o644))
		if _, err := Load(home, filepath.Join(home, "dotfiles")); err == nil {
			t.Errorf("сломанный конфиг принят молча:\n%s", body)
		}
	}
}

func TestPlistCarriesScheduleAndLauncher(t *testing.T) {
	home := t.TempDir()
	dotfiles := filepath.Join(home, "dotfiles")

	daily, err := Plist(home, dotfiles, Routine{Name: "scrub", Schedule: Schedule{At: "03:30"}, Exec: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<key>Hour</key>", "<integer>3</integer>", "<integer>30</integer>", "hq-cron", "routine", "run", "scrub"} {
		if !strings.Contains(daily, want) {
			t.Errorf("в плисте нет %q:\n%s", want, daily)
		}
	}

	interval, err := Plist(home, dotfiles, Routine{Name: "drain", Schedule: Schedule{Every: 6 * time.Hour}, Exec: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(interval, "<key>StartInterval</key>") || !strings.Contains(interval, "<integer>21600</integer>") {
		t.Errorf("интервал не проставлен:\n%s", interval)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
