package wiki

import (
	"os"

	"github.com/Gerc0g/dotfiles/core/routine"
)

// Autosync runs from hooks, where output goes nowhere. Without a trace there
// is no way to tell "the hook never fired" from "the hook fired and found
// nothing" — and that distinction is exactly what breaks silently.
//
// The trace goes into the platform's one journal of background work rather
// than a file of its own. A log living by itself is a log nobody opens: this
// one sat unread while the hooks it was recording had been dead for days.

// autosyncSource tags these lines in the shared journal.
const autosyncSource = "wiki"

// AutosyncLogPath is where the trace lives.
func AutosyncLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return routine.JournalPath(home), nil
}

// LogAutosync records one line about wiki background work.
func LogAutosync(format string, args ...any) {
	routine.Log(autosyncSource, format, args...)
}

// LastAutosync returns the most recent wiki line, or empty when there is none.
func LastAutosync() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	lines := routine.Tail(home, autosyncSource, 1)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}
