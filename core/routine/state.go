package routine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// The state store answers the questions a listing has to answer: did this run,
// when, how did it end, and how much of today's allowance is left. Guards read
// from it too — a ceiling nobody counts against is decoration.

// Outcome is how a run ended.
type Outcome string

const (
	OutcomeOK      Outcome = "ok"
	OutcomeFailed  Outcome = "failed"
	OutcomeSkipped Outcome = "skipped"
)

// Status is what the platform remembers about one routine.
type Status struct {
	Last       time.Time `json:"last,omitempty"`
	Outcome    Outcome   `json:"outcome,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Runs       int       `json:"runs,omitempty"`
	RunsDay    int       `json:"runs_day,omitempty"`
	DayStamp   string    `json:"day,omitempty"`
	Disabled   bool      `json:"disabled,omitempty"`
}

// RunsToday is the run count for today, ignoring a counter left from an
// earlier day.
func (s Status) RunsToday(now time.Time) int {
	if s.DayStamp != now.Format("2006-01-02") {
		return 0
	}
	return s.RunsDay
}

type stateFile struct {
	Routines map[string]Status `json:"routines"`
}

func statePath(home string) string {
	return filepath.Join(home, ".cache", "hq", "routines.json")
}

// LoadState reads every routine's status. A missing file is an empty state,
// not an error: nothing has run yet.
func LoadState(home string) map[string]Status {
	data, err := os.ReadFile(statePath(home))
	if err != nil {
		return map[string]Status{}
	}
	var file stateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return map[string]Status{}
	}
	if file.Routines == nil {
		return map[string]Status{}
	}
	return file.Routines
}

// SaveStatus records one routine's status, leaving the others alone.
func SaveStatus(home, name string, status Status) error {
	all := LoadState(home)
	all[name] = status

	path := statePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(stateFile{Routines: all}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

// SetDisabled turns a routine off or on without touching its history.
func SetDisabled(home, name string, disabled bool) error {
	status := LoadState(home)[name]
	status.Disabled = disabled
	return SaveStatus(home, name, status)
}
