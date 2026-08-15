package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Autosync runs from hooks, where output goes nowhere. Without a trace there
// is no way to tell "the hook never fired" from "the hook fired and found
// nothing" — and that distinction is exactly what breaks silently.
//
// One line per invocation, capped, so it stays a diagnostic and not a log to
// maintain.

const autosyncLogLimit = 400

// AutosyncLogPath is where the trace lives.
func AutosyncLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "wikipedik-autosync.log"), nil
}

// LogAutosync appends one line; failures are ignored on purpose — a hook must
// never break because its diagnostics could not be written.
func LogAutosync(format string, args ...any) {
	path, err := AutosyncLogPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	line := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(format, args...))

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = file.WriteString(line)
	_ = file.Close()

	trimAutosyncLog(path)
}

// trimAutosyncLog keeps the tail, so the file cannot grow without bound.
func trimAutosyncLog(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := splitLines(string(data))
	if len(lines) <= autosyncLogLimit {
		return
	}
	tail := lines[len(lines)-autosyncLogLimit:]
	_ = os.WriteFile(path, []byte(joinLines(tail)), 0o644)
}

func splitLines(text string) []string {
	var out []string
	start := 0
	for i, r := range text {
		if r == '\n' {
			out = append(out, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

func joinLines(lines []string) string {
	out := ""
	for _, line := range lines {
		out += line + "\n"
	}
	return out
}
