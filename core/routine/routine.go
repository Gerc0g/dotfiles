// Package routine is the platform's background work: what runs unattended,
// when, at whose expense, and whether it actually ran.
//
// Background work had spread across mechanisms nobody could see whole — a
// launchd job here, an --if-due guard there, a log file living by itself. Some
// of it starts an agent, so a forgotten job spends money quietly. "Set a cron
// and forget it" is more expensive here than usual, and twice in two days a
// silent failure went unnoticed for days.
//
// So a routine declares itself: its schedule, whether it spends tokens, the
// guards that bound it. The registry is the single place to look.
package routine

import (
	"fmt"
	"strings"
	"time"
)

// Kind separates work that costs only electricity from work that spends the
// subscription. Agent routines are guarded by default and marked in every
// listing, because their cost is invisible at the moment they run.
type Kind string

const (
	KindInternal Kind = "internal"
	KindAgent    Kind = "agent"
)

func (k Kind) SpendsTokens() bool { return k == KindAgent }

// Schedule is either a fixed interval or a time of day. Exactly one is set.
type Schedule struct {
	Every time.Duration
	At    string // "HH:MM", local time
}

func (s Schedule) String() string {
	switch {
	case s.Every > 0:
		return "каждые " + humanDuration(s.Every)
	case s.At != "":
		return "в " + s.At
	default:
		return "по требованию"
	}
}

// Period is how long between runs the schedule implies; it is what tells a
// health check when silence has gone on too long.
func (s Schedule) Period() time.Duration {
	if s.Every > 0 {
		return s.Every
	}
	if s.At != "" {
		return 24 * time.Hour
	}
	return 0
}

// AgentJob is work stated as a prompt rather than a command: the user
// describes what they want done, the platform supplies the run, the guards and
// the record of it.
type AgentJob struct {
	Prompt string `yaml:"prompt"`
	Dir    string `yaml:"dir"`
	Agent  string `yaml:"agent"` // codex (default) or claude
}

// Routine is one unit of background work.
type Routine struct {
	Name  string
	About string
	Kind  Kind

	Schedule Schedule

	// Exec is the command to run; Agent is a prompt to hand an agent. Exactly
	// one is set.
	Exec  []string
	Agent *AgentJob

	// MinInterval refuses a run that follows too closely on the last one, even
	// when something asks for it out of schedule.
	MinInterval time.Duration
	// MaxRunsPerDay is the hard ceiling on cost. Zero means the default for the
	// kind, which is never unlimited for agent work.
	MaxRunsPerDay int

	// Source says where the routine came from, so a listing can separate what
	// the platform ships from what the user wrote.
	Source string
}

// Label is the launchd job name for a routine.
func (r Routine) Label() string { return "com.gerc0g.routine." + r.Name }

// Ceiling is the effective runs-per-day limit.
//
// Agent routines never run unlimited: an unattended job that spends tokens
// needs a number even when nobody thought to set one.
func (r Routine) Ceiling() int {
	if r.MaxRunsPerDay > 0 {
		return r.MaxRunsPerDay
	}
	if r.Kind.SpendsTokens() {
		return defaultAgentRunsPerDay
	}
	return 0 // internal work is cheap; no ceiling needed
}

const defaultAgentRunsPerDay = 6

// Validate rejects a routine that cannot be scheduled or run.
func (r Routine) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("у routine нет имени")
	}
	if len(r.Exec) == 0 && r.Agent == nil {
		return fmt.Errorf("%s: нечего запускать — ни команды, ни промпта", r.Name)
	}
	if len(r.Exec) > 0 && r.Agent != nil {
		return fmt.Errorf("%s: и команда, и промпт — выбери одно", r.Name)
	}
	if r.Agent != nil && strings.TrimSpace(r.Agent.Prompt) == "" {
		return fmt.Errorf("%s: пустой промпт", r.Name)
	}
	if r.Schedule.Every > 0 && r.Schedule.At != "" {
		return fmt.Errorf("%s: и интервал, и время суток — выбери одно", r.Name)
	}
	if r.Schedule.At != "" {
		if _, _, err := parseDaily(r.Schedule.At); err != nil {
			return fmt.Errorf("%s: %w", r.Name, err)
		}
	}
	return nil
}

func parseDaily(at string) (hour, minute int, err error) {
	if _, err := fmt.Sscanf(at, "%d:%d", &hour, &minute); err != nil {
		return 0, 0, fmt.Errorf("время %q не в формате HH:MM", at)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("время %q вне суток", at)
	}
	return hour, minute, nil
}

func humanDuration(d time.Duration) string {
	switch {
	case d%(24*time.Hour) == 0:
		return fmt.Sprintf("%d дн.", int(d.Hours()/24))
	case d%time.Hour == 0:
		return fmt.Sprintf("%d ч.", int(d.Hours()))
	default:
		return d.String()
	}
}
