package routine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// One journal for all background work. Scattered logs are how a job dies
// unnoticed: nobody opens a file they have to remember exists.

const journalLines = 2000

// JournalPath is the single log of background work.
func JournalPath(home string) string {
	return filepath.Join(home, ".cache", "hq", "routines.log")
}

// Log appends one line, attributed to its source.
func Log(source, format string, args ...any) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	line := fmt.Sprintf("%s [%s] %s\n",
		time.Now().Format("2006-01-02 15:04:05"), source, fmt.Sprintf(format, args...))

	path := JournalPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = file.WriteString(line)
	_ = file.Close()

	trimJournal(path)
}

// Tail returns the last n lines, newest last, optionally filtered by source.
func Tail(home, source string, n int) []string {
	data, err := os.ReadFile(JournalPath(home))
	if err != nil {
		return nil
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		if source != "" && !strings.Contains(line, "["+source+"]") {
			continue
		}
		kept = append(kept, line)
	}
	if n > 0 && len(kept) > n {
		kept = kept[len(kept)-n:]
	}
	return kept
}

// trimJournal keeps the log bounded. An unattended writer with no ceiling
// eventually becomes the problem it was meant to reveal.
func trimJournal(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) <= journalLines {
		return
	}
	kept := strings.Join(lines[len(lines)-journalLines:], "\n") + "\n"
	_ = os.WriteFile(path, []byte(kept), 0o644)
}
