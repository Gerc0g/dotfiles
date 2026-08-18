package routine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// The registry is the answer to "what runs on this machine without me". It
// holds what the platform ships and what the user wrote, in one list, because
// the failure this entity exists to prevent is a job nobody remembers.

// UserConfigPath is where the user declares their own routines.
func UserConfigPath(home string) string {
	return filepath.Join(home, ".config", "hq", "routines.yaml")
}

// builtins are the platform's own background work.
func builtins(dotfiles string) []Routine {
	return []Routine{
		{
			Name:     "transcript-scrub",
			About:    "вычистить секреты из транскриптов агентов",
			Kind:     KindInternal,
			Schedule: Schedule{At: "03:30"},
			Exec:     []string{"/usr/bin/python3", filepath.Join(dotfiles, "scripts", "transcript-scrub.py")},
		},
		{
			Name:  "memory-drain",
			About: "разобрать инбокс самого запущенного репозитория",
			Kind:  KindAgent,
			// Six hours between attempts; the drain's own guards then decide
			// whether any scope is actually due.
			Schedule:      Schedule{Every: 6 * time.Hour},
			Exec:          []string{filepath.Join(dotfiles, "bin", "hq"), "wiki", "cron", "--commit"},
			MaxRunsPerDay: 4,
		},
		{
			Name:  "vault-sync",
			About: "закоммитить и запушить вольт: результаты фоновых задач и накопившееся",
			Kind:  KindInternal,
			// Background work leaves the vault changed but unrecorded — the
			// scrubber rewrites session digests and walks away, and scoped
			// drains commit without ever pushing. Nothing else closes that gap.
			Schedule: Schedule{Every: 6 * time.Hour},
			Exec:     []string{filepath.Join(dotfiles, "bin", "hq"), "wiki", "autocommit"},
		},
	}
}

// userRoutine is the on-disk shape of a user-declared routine.
type userRoutine struct {
	Name          string   `yaml:"name"`
	About         string   `yaml:"about"`
	Every         string   `yaml:"every"`
	At            string   `yaml:"at"`
	Prompt        string   `yaml:"prompt"`
	Dir           string   `yaml:"dir"`
	Agent         string   `yaml:"agent"`
	Exec          []string `yaml:"exec"`
	MaxRunsPerDay int      `yaml:"max_runs_per_day"`
	MinInterval   string   `yaml:"min_interval"`
}

// Load returns every routine: the platform's, then the user's.
//
// A broken user file is reported, never silently ignored — a routine the user
// believes exists but which never runs is the exact failure this package is
// here to prevent.
func Load(home, dotfiles string) ([]Routine, error) {
	all := builtins(dotfiles)

	path := UserConfigPath(home)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return all, nil
	}
	if err != nil {
		return all, fmt.Errorf("чтение %s: %w", path, err)
	}

	var declared []userRoutine
	if err := yaml.Unmarshal(data, &declared); err != nil {
		return all, fmt.Errorf("%s: невалидный YAML: %w", path, err)
	}

	known := map[string]bool{}
	for _, r := range all {
		known[r.Name] = true
	}

	for _, u := range declared {
		r, err := u.toRoutine(path)
		if err != nil {
			return all, err
		}
		if known[r.Name] {
			return all, fmt.Errorf("%s: имя %q уже занято встроенной routine", path, r.Name)
		}
		known[r.Name] = true
		all = append(all, r)
	}

	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all, nil
}

func (u userRoutine) toRoutine(source string) (Routine, error) {
	r := Routine{
		Name:          strings.TrimSpace(u.Name),
		About:         u.About,
		Kind:          KindInternal,
		Exec:          u.Exec,
		MaxRunsPerDay: u.MaxRunsPerDay,
		Source:        source,
	}
	if u.Prompt != "" {
		r.Kind = KindAgent
		r.Agent = &AgentJob{Prompt: u.Prompt, Dir: u.Dir, Agent: u.Agent}
	}

	if u.Every != "" {
		every, err := time.ParseDuration(u.Every)
		if err != nil {
			return Routine{}, fmt.Errorf("%s: routine %q: интервал %q не разобран: %w", source, u.Name, u.Every, err)
		}
		r.Schedule.Every = every
	}
	r.Schedule.At = u.At

	if u.MinInterval != "" {
		min, err := time.ParseDuration(u.MinInterval)
		if err != nil {
			return Routine{}, fmt.Errorf("%s: routine %q: min_interval %q не разобран: %w", source, u.Name, u.MinInterval, err)
		}
		r.MinInterval = min
	}

	if err := r.Validate(); err != nil {
		return Routine{}, fmt.Errorf("%s: %w", source, err)
	}
	return r, nil
}

// Find returns one routine by name.
func Find(home, dotfiles, name string) (Routine, error) {
	all, err := Load(home, dotfiles)
	if err != nil {
		return Routine{}, err
	}
	for _, r := range all {
		if r.Name == name {
			return r, nil
		}
	}
	return Routine{}, fmt.Errorf("нет routine %q — список: hq routine", name)
}
