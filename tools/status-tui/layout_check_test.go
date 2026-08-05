package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func sampleSnapshot() snapshot {
	s := snapshot{
		Host:             "MacBook-Pro-Gerc0g.local",
		OS:               "darwin",
		Time:             time.Unix(1700000000, 0),
		LocalAgentLimit:  2,
		ActiveAgentCount: 18,
		ClaudeCount:      8,
		CodexCount:       10,
		TmuxSessionCount: 8,
		PressureSeverity: sevCritical,
		Memory: []metric{
			{Name: "ram", Value: "23.9G/24.0G", Detail: "99% not free", Severity: sevCritical},
			{Name: "swap", Value: "6.4G/7.0G", Detail: "92% used", Severity: sevCritical},
			{Name: "wired", Value: "12.5G", Detail: "system/GPU/kernel", Severity: sevCritical},
			{Name: "compressor", Value: "4.7G", Detail: "compressed pages", Severity: sevCritical},
		},
	}
	for i := 0; i < 8; i++ {
		s.Sessions = append(s.Sessions, sessionInfo{Name: "neurodesk-agents-cerebellum-agent-e64392df", State: "detached", Agents: "claude:12 codex:12 test:2"})
	}
	for i := 0; i < 8; i++ {
		s.Processes = append(s.Processes, processInfo{Name: "com.apple.WebKit.WebContent", Memory: "2207M", Kind: "term"})
	}
	return s
}

func TestNoLineOverflow(t *testing.T) {
	m := model{width: 220, height: 50, snap: sampleSnapshot()}
	for i := 0; i < 30; i++ {
		m.ramHist = pushHistory(m.ramHist, 99)
		m.swapHist = pushHistory(m.swapHist, 92)
	}
	out := m.View()
	for n, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > dashWidth {
			t.Fatalf("line %d width %d > %d: %q", n, w, dashWidth, ansi.Strip(line))
		}
	}
}
