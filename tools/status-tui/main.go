package main

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"
)

type severity int

const (
	sevOK severity = iota
	sevWarn
	sevCritical
	sevUnknown
)

type metric struct {
	Name     string
	Value    string
	Detail   string
	Severity severity
}

type processInfo struct {
	PID     string
	Name    string
	Memory  string
	CPU     string
	Threads string
	Kind    string
}

type processArgs struct {
	PID  string
	Comm string
	Args string
}

type sessionInfo struct {
	Name   string
	State  string
	Agents string
	Age    string
}

type snapshot struct {
	Host              string
	OS                string
	Time              time.Time
	Memory            []metric
	Agents            []metric
	Sessions          []sessionInfo
	Processes         []processInfo
	Notes             []string
	LocalAgentLimit   int
	ActiveAgentCount  int
	ClaudeCount       int
	CodexCount        int
	TmuxSessionCount  int
	TestOracleCount   int
	PressureSeverity  severity
	CollectionWarning string
}

type model struct {
	width     int
	height    int
	snap      snapshot
	loading   bool
	err       string
	refreshes int
	ramHist   []float64
	swapHist  []float64
}

type snapshotMsg struct {
	Snap snapshot
	Err  error
}

type tickMsg struct{}

const refreshEvery = 5 * time.Second

// dashWidth keeps the whole panel a fixed, compact block anchored to the
// top-left, leaving the rest of the terminal free for future board panels.
const dashWidth = 84

var (
	bg        = lipgloss.Color("#151821")
	panel     = lipgloss.Color("#202534")
	panel2    = lipgloss.Color("#252B3A")
	accent    = lipgloss.Color("#9ADBC5")
	accent2   = lipgloss.Color("#F3C969")
	text      = lipgloss.Color("#E8EAF2")
	muted     = lipgloss.Color("#8F96AA")
	dim       = lipgloss.Color("#5D6374")
	okColor   = lipgloss.Color("#9ADBC5")
	warnColor = lipgloss.Color("#F0C674")
	badColor  = lipgloss.Color("#FF8FA3")
	hotBg     = lipgloss.Color("#3A2430")
	okBg      = lipgloss.Color("#1D332F")
	warnBg    = lipgloss.Color("#3A3320")

	// No background fills anywhere: Lip Gloss does not fill a parent Background
	// uniformly behind pre-styled content (bars/chips/clamp insert ANSI resets),
	// which leaks as uneven rectangles. Cards are defined by coloured borders
	// only; everything sits flat on the terminal background.
	screenStyle = lipgloss.NewStyle().Foreground(text)
	titleStyle  = lipgloss.NewStyle().Foreground(accent).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(muted)
	dimStyle    = lipgloss.NewStyle().Foreground(dim)
	helpStyle   = lipgloss.NewStyle().Foreground(muted)
	heroStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 2)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dim).
			Padding(0, 1)
	cardHotStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Background(panel).
			Padding(0, 1)
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--once" {
		s, err := collectSnapshot()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(renderPlain(s))
		return
	}

	m := model{loading: true}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "status tui failed: %s\n", err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(loadSnapshot(), tick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, loadSnapshot()
		}
	case tickMsg:
		m.loading = true
		return m, tea.Batch(loadSnapshot(), tick())
	case snapshotMsg:
		m.loading = false
		m.refreshes++
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.err = ""
		m.snap = msg.Snap
		m.ramHist = pushHistory(m.ramHist, percentFromMetric(metricByName(msg.Snap.Memory, "ram")))
		m.swapHist = pushHistory(m.swapHist, percentFromMetric(metricByName(msg.Snap.Memory, "swap")))
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	w := m.width
	if w <= 0 || w > dashWidth {
		w = dashWidth
	}
	if w < 60 {
		w = 60
	}

	body := []string{renderHeader(m, w)}
	if m.err != "" {
		body = append(body, cardStyle.Width(w-2).Render(clamp(styleSeverity(sevCritical).Render("collection error")+"  "+m.err, w-4)))
	} else if m.snap.Time.IsZero() {
		body = append(body, cardStyle.Width(w-2).Render("loading..."))
	} else {
		body = append(body, renderVerdict(m.snap, w))
		body = append(body, renderCommandCenter(m, w))
		body = append(body, renderMiddle(m.snap, w))
	}

	body = append(body, renderHelp(w))
	out := lipgloss.JoinVertical(lipgloss.Left, body...)
	if m.height > lipgloss.Height(out) {
		out += strings.Repeat("\n", m.height-lipgloss.Height(out))
	}
	return screenStyle.Width(w).Render(out)
}

func renderHeader(m model, width int) string {
	s := m.snap
	status := "COLLECTING"
	sev := sevUnknown
	if !s.Time.IsZero() {
		status = strings.ToUpper(severityLabel(s.PressureSeverity))
		sev = s.PressureSeverity
	}
	if m.loading {
		status = status + " · refresh"
	}
	left := titleStyle.Render("Состояние агентов")
	right := styleSeverity(sev).Render(status)
	meta := mutedStyle.Render(time.Now().Format("15:04:05"))
	lineWidth := max(1, width-lipgloss.Width(left)-lipgloss.Width(right)-lipgloss.Width(meta)-8)
	line := dimStyle.Render(strings.Repeat("─", lineWidth))
	return clamp(lipgloss.JoinHorizontal(lipgloss.Center, "  ", left, " ", line, " ", meta, " ", right), width)
}

func renderVerdict(s snapshot, width int) string {
	sev := s.PressureSeverity
	headline := "Система в норме"
	switch sev {
	case sevCritical:
		headline = "Mac перегружен"
	case sevWarn:
		headline = "Mac на границе"
	}

	over := s.ActiveAgentCount - s.LocalAgentLimit
	agentLine := fmt.Sprintf("агентов %d/%d", s.ActiveAgentCount, s.LocalAgentLimit)
	if over > 0 {
		agentLine += fmt.Sprintf(" (+%d)", over)
	}
	details := fmt.Sprintf("%s · swap %s · comp %s · wired %s",
		agentLine,
		emptyDash(metricValue(s.Memory, "swap")),
		emptyDash(metricValue(s.Memory, "compressor")),
		emptyDash(metricValue(s.Memory, "wired")))

	status := pill(strings.ToUpper(severityLabel(sev)), sev)
	title := lipgloss.NewStyle().Bold(true).Foreground(severityColor(sev)).Render(headline)
	inner := width - 6
	content := lipgloss.JoinVertical(lipgloss.Left,
		clamp(lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", status), inner),
		clamp(mutedStyle.Render(details), inner),
		clamp(mutedStyle.Render("решение: держи 1-2 активных, остальное усыпляй/закрывай"), inner),
	)
	return heroStyle.BorderForeground(severityColor(sev)).Width(width - 2).Render(content)
}

func renderCommandCenter(m model, width int) string {
	s := m.snap
	gap := 1
	avail := width - gap
	left := avail / 2
	right := avail - left
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderMemoryCard(s, m.ramHist, m.swapHist, left),
		strings.Repeat(" ", gap),
		renderAgentCard(s, right),
	)
}

func renderMemoryCard(s snapshot, ramHist, swapHist []float64, outer int) string {
	inner := outer - 4
	ram := metricByName(s.Memory, "ram")
	swap := metricByName(s.Memory, "swap")
	comp := metricByName(s.Memory, "compressor")
	wired := metricByName(s.Memory, "wired")
	lines := []string{sectionTitle("Память", maxMetricSeverity(s.Memory))}
	lines = append(lines, metricBarLine("RAM", ram, inner))
	lines = append(lines, sparkRow(ramHist, ram.Severity, inner))
	lines = append(lines, metricBarLine("Swap", swap, inner))
	lines = append(lines, sparkRow(swapHist, swap.Severity, inner))
	lines = append(lines, compactMetric("Comp", comp, inner))
	lines = append(lines, compactMetric("Wired", wired, inner))
	return hotCard(maxMetricSeverity(s.Memory)).Width(outer - 2).Height(7).Render(strings.Join(clampAll(lines, inner), "\n"))
}

func renderAgentCard(s snapshot, outer int) string {
	inner := outer - 4
	sev := limitSeverity(s.ActiveAgentCount, s.LocalAgentLimit)
	over := max(0, s.ActiveAgentCount-s.LocalAgentLimit)
	lines := []string{sectionTitle("Агенты", sev)}
	lines = append(lines, bigNumber(fmt.Sprintf("%d/%d", s.ActiveAgentCount, s.LocalAgentLimit), sev)+"  "+mutedStyle.Render("живые Claude+Codex"))
	if over > 0 {
		lines = append(lines, styleSeverity(sevCritical).Render(fmt.Sprintf("перебор: +%d процессов", over)))
	} else {
		lines = append(lines, styleSeverity(sevOK).Render("в пределах лимита"))
	}
	lines = append(lines, "")
	lines = append(lines, chip("Claude", strconv.Itoa(s.ClaudeCount), sevForCount(s.ClaudeCount, 2, 5))+" "+chip("Codex", strconv.Itoa(s.CodexCount), sevForCount(s.CodexCount, 2, 5)))
	lines = append(lines, chip("tmux", strconv.Itoa(s.TmuxSessionCount), sevForCount(s.TmuxSessionCount, 3, 7))+" "+chip("test/oracle", strconv.Itoa(s.TestOracleCount), sevForCount(s.TestOracleCount, 2, 6)))
	return hotCard(sev).Width(outer - 2).Height(7).Render(strings.Join(clampAll(lines, inner), "\n"))
}

func sectionTitle(title string, sev severity) string {
	return lipgloss.JoinHorizontal(lipgloss.Center,
		styleSeverity(sev).Render(symbol(sev)),
		" ",
		titleStyle.Render(title),
	)
}

func metricBarLine(label string, m metric, width int) string {
	pct := percentFromMetric(m)
	barWidth := max(6, width-17)
	left := mutedStyle.Width(5).Render(label)
	value := styleSeverity(m.Severity).Bold(true).Width(11).Render(m.Value)
	return clamp(lipgloss.JoinHorizontal(lipgloss.Center, left, value, " ", gradientBar(pct, barWidth)), width)
}

// gradientBar renders a smooth green→red progress bar via bubbles/progress.
// The scaled gradient maps fill level to colour, so a near-full bar reads red.
func gradientBar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	p := progress.New(
		progress.WithScaledGradient("#9ADBC5", "#FF8FA3"),
		progress.WithoutPercentage(),
		progress.WithWidth(width),
	)
	return p.ViewAs(math.Max(0, math.Min(1, pct/100)))
}

func compactMetric(label string, m metric, width int) string {
	left := mutedStyle.Width(6).Render(label)
	value := styleSeverity(m.Severity).Bold(true).Width(9).Render(emptyDash(m.Value))
	detail := dimStyle.Render(truncate(m.Detail, max(0, width-15)))
	return clamp(lipgloss.JoinHorizontal(lipgloss.Top, left, value, detail), width)
}

func hotCard(sev severity) lipgloss.Style {
	// Severity is carried by the border colour; no background fill (see styles).
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(severityColor(sev)).
		Padding(0, 1)
}

func bigNumber(value string, sev severity) string {
	return lipgloss.NewStyle().
		Foreground(severityColor(sev)).
		Bold(true).
		Render(value)
}

func chip(label, value string, sev severity) string {
	return lipgloss.NewStyle().
		Foreground(bg).
		Background(severityColor(sev)).
		Bold(true).
		Padding(0, 1).
		Render(label + " " + value)
}

func actionLine(num, text string) string {
	badge := lipgloss.NewStyle().
		Foreground(bg).
		Background(accent).
		Bold(true).
		Padding(0, 1).
		Render(num)
	return lipgloss.JoinHorizontal(lipgloss.Top, badge, " ", text)
}

func pill(label string, sev severity) string {
	return lipgloss.NewStyle().
		Foreground(bg).
		Background(severityColor(sev)).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

const maxHistory = 120

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

func pushHistory(h []float64, v float64) []float64 {
	h = append(h, v)
	if len(h) > maxHistory {
		h = h[len(h)-maxHistory:]
	}
	return h
}

// sparkline renders the trailing values as a single row of block glyphs,
// padding the left with dots until enough history has accumulated.
func sparkline(values []float64, width int, sev severity) string {
	if width <= 0 || len(values) == 0 {
		return ""
	}
	vals := values
	if len(vals) > width {
		vals = vals[len(vals)-width:]
	}
	var b strings.Builder
	for _, v := range vals {
		v = math.Max(0, math.Min(100, v))
		idx := int(math.Round(v / 100 * float64(len(sparkRunes)-1)))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkRunes) {
			idx = len(sparkRunes) - 1
		}
		b.WriteRune(sparkRunes[idx])
	}
	out := ""
	if pad := width - len(vals); pad > 0 {
		out += dimStyle.Render(strings.Repeat("·", pad))
	}
	return out + styleSeverity(sev).Render(b.String())
}

// sparkRow draws the history sparkline starting at the exact column where the
// metric bar starts (label 5 + value 11 + 1 space = 17) and at the bar's width,
// so the bar and its history line up in one column.
func sparkRow(values []float64, sev severity, width int) string {
	return clamp(lipgloss.JoinHorizontal(lipgloss.Top, strings.Repeat(" ", 17), sparkline(values, max(6, width-17), sev)), width)
}

// clamp truncates a (possibly styled) string to w visible cells, ANSI-safe.
func clamp(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "")
}

func clampAll(lines []string, w int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = clamp(l, w)
	}
	return out
}

func metricByName(metrics []metric, name string) metric {
	for _, m := range metrics {
		if m.Name == name {
			return m
		}
	}
	return metric{Name: name, Value: "—", Detail: "нет данных", Severity: sevUnknown}
}

func metricValue(metrics []metric, name string) string {
	return metricByName(metrics, name).Value
}

func percentFromMetric(m metric) float64 {
	fields := strings.Fields(m.Detail)
	for _, f := range fields {
		if strings.HasSuffix(f, "%") {
			v, err := strconv.ParseFloat(strings.TrimSuffix(f, "%"), 64)
			if err == nil {
				return v
			}
		}
	}
	switch m.Severity {
	case sevCritical:
		return 95
	case sevWarn:
		return 70
	case sevOK:
		return 35
	default:
		return 0
	}
}

func renderMiddle(s snapshot, width int) string {
	gap := 1
	avail := width - gap
	leftW := avail / 2
	rightW := avail - leftW
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderSessions(s, leftW),
		strings.Repeat(" ", gap),
		renderProcesses(s, rightW),
	)
}

// listTable builds a borderless lipgloss/table sized by the per-column widths
// returned from styleFn. The card supplies the surrounding border.
//
// Quirk: in lipgloss v1.1.0 disabling all borders drops the last row
// (off-by-one). Instead we use an empty Border{} — which renders correctly but
// emits a blank top and bottom line — and trim those two lines ourselves.
func listTable(rows [][]string, styleFn table.StyleFunc) string {
	out := table.New().
		Border(lipgloss.Border{}).
		BorderColumn(false).BorderRow(false).
		Wrap(false).
		StyleFunc(styleFn).
		Rows(rows...).
		Render()
	lines := strings.Split(out, "\n")
	if len(lines) >= 2 {
		lines = lines[1 : len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func sessionSev(s sessionInfo) severity {
	if strings.Contains(s.State, "attached") {
		return sevWarn
	}
	return sevOK
}

func renderSessions(s snapshot, outer int) string {
	inner := outer - 4
	title := sectionTitle("Сессии tmux", sevForCount(s.TmuxSessionCount, 3, 7))
	body := dimStyle.Render("tmux-сессий нет")
	if len(s.Sessions) > 0 {
		sess := firstSessions(s.Sessions, 8)
		agW := 12
		nameW := max(8, inner-agW-3)
		rows := make([][]string, len(sess))
		for i, se := range sess {
			rows[i] = []string{symbol(sessionSev(se)), truncate(se.Name, nameW), truncate(se.Agents, agW)}
		}
		body = listTable(rows, func(r, c int) lipgloss.Style {
			switch c {
			case 0:
				sev := sevOK
				if r >= 0 && r < len(sess) {
					sev = sessionSev(sess[r])
				}
				return lipgloss.NewStyle().Foreground(severityColor(sev)).PaddingRight(1)
			case 1:
				return lipgloss.NewStyle().Foreground(text).Width(nameW).PaddingRight(1)
			default:
				return mutedStyle.Width(agW)
			}
		})
	}
	content := lipgloss.JoinVertical(lipgloss.Left, title, body)
	return cardStyle.Width(outer-2).Height(9).Render(strings.Join(clampAll(strings.Split(content, "\n"), inner), "\n"))
}

func renderProcesses(s snapshot, outer int) string {
	inner := outer - 4
	title := sectionTitle("Топ памяти", sevWarn)
	body := dimStyle.Render("нет данных по процессам")
	if len(s.Processes) > 0 {
		procs := firstProcesses(s.Processes, 8)
		kindW, memW := 9, 7
		nameW := max(6, inner-kindW-memW-2)
		rows := make([][]string, len(procs))
		for i, p := range procs {
			rows[i] = []string{processKindRu(p.Kind), p.Memory, truncate(p.Name, nameW)}
		}
		body = listTable(rows, func(r, c int) lipgloss.Style {
			switch c {
			case 0:
				return dimStyle.Width(kindW).PaddingRight(1)
			case 1:
				return lipgloss.NewStyle().Foreground(accent2).Width(memW).PaddingRight(1)
			default:
				return lipgloss.NewStyle().Foreground(text).Width(nameW)
			}
		})
	}
	content := lipgloss.JoinVertical(lipgloss.Left, title, body)
	return cardStyle.Width(outer-2).Height(9).Render(strings.Join(clampAll(strings.Split(content, "\n"), inner), "\n"))
}

func renderNotes(s snapshot, width int) string {
	var lines []string
	lines = append(lines, sectionTitle("Пояснение", s.PressureSeverity))
	for _, n := range s.Notes {
		lines = append(lines, "  "+translateNote(n))
	}
	return cardStyle.Width(width - 4).Render(strings.Join(lines, "\n"))
}

func renderHelp(width int) string {
	help := "q выход · r обновить · авто 5с · лимит STATUS_LOCAL_AGENT_LIMIT"
	return clamp(helpStyle.Render("  "+help), width)
}

func renderPlain(s snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Состояние агентов  %s  %s\n", s.Host, s.Time.Format(time.RFC3339))
	fmt.Fprintf(&b, "Вердикт: %s\n\n", severityLabel(s.PressureSeverity))
	for _, m := range append(s.Memory, s.Agents...) {
		fmt.Fprintf(&b, "%-14s %-12s %s [%s]\n", m.Name, m.Value, m.Detail, severityLabel(m.Severity))
	}
	return b.String()
}

func loadSnapshot() tea.Cmd {
	return func() tea.Msg {
		s, err := collectSnapshot()
		return snapshotMsg{Snap: s, Err: err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

func collectSnapshot() (snapshot, error) {
	host, _ := os.Hostname()
	s := snapshot{
		Host:            host,
		OS:              runtime.GOOS,
		Time:            time.Now(),
		LocalAgentLimit: envInt("STATUS_LOCAL_AGENT_LIMIT", 2),
	}

	var warnings []string
	if runtime.GOOS == "darwin" {
		mem, warn := collectDarwinMemory()
		s.Memory = mem
		warnings = append(warnings, warn...)
	} else if runtime.GOOS == "linux" {
		mem, warn := collectLinuxMemory()
		s.Memory = mem
		warnings = append(warnings, warn...)
	} else {
		s.Memory = []metric{{Name: "memory", Value: "unknown", Detail: runtime.GOOS, Severity: sevUnknown}}
	}

	procs, warn := collectProcesses()
	warnings = append(warnings, warn...)
	s.Processes = procs
	fullProcs, warn := collectProcessArgs()
	warnings = append(warnings, warn...)

	sessions, warn := collectTmuxSessions(fullProcs)
	warnings = append(warnings, warn...)
	s.Sessions = sessions
	s.TmuxSessionCount = len(sessions)

	s.ClaudeCount = countMatchingArgs(fullProcs, isClaudeProcess)
	s.CodexCount = countMatchingArgs(fullProcs, isCodexProcess)
	s.TestOracleCount = countMatchingArgs(fullProcs, func(p processArgs) bool {
		comm := baseName(p.Comm)
		return comm == "test-tui" || comm == "oracle-tui" || strings.Contains(p.Args, "/tools/test-tui/test-tui") || strings.Contains(p.Args, "/tools/oracle-tui/oracle-tui")
	})
	s.ActiveAgentCount = s.ClaudeCount + s.CodexCount
	s.Agents = []metric{
		{Name: "active", Value: fmt.Sprintf("%d/%d", s.ActiveAgentCount, s.LocalAgentLimit), Detail: "claude + codex", Severity: limitSeverity(s.ActiveAgentCount, s.LocalAgentLimit)},
		{Name: "claude", Value: strconv.Itoa(s.ClaudeCount), Detail: "live processes", Severity: sevForCount(s.ClaudeCount, 2, 5)},
		{Name: "codex", Value: strconv.Itoa(s.CodexCount), Detail: "live processes", Severity: sevForCount(s.CodexCount, 2, 5)},
		{Name: "tmux", Value: strconv.Itoa(s.TmuxSessionCount), Detail: "sessions", Severity: sevForCount(s.TmuxSessionCount, 3, 7)},
		{Name: "test/oracle", Value: strconv.Itoa(s.TestOracleCount), Detail: "panes", Severity: sevForCount(s.TestOracleCount, 2, 6)},
	}

	s.PressureSeverity = maxMetricSeverity(append(s.Memory, s.Agents...))
	s.CollectionWarning = strings.Join(nonEmpty(warnings), " · ")
	s.Notes = recommendations(s)
	return s, nil
}

func collectDarwinMemory() ([]metric, []string) {
	var out []metric
	var warnings []string

	memBytes := int64(0)
	if s, err := run(2*time.Second, "sysctl", "-n", "hw.memsize"); err == nil {
		memBytes = parseInt64(strings.TrimSpace(s))
	} else {
		warnings = append(warnings, "hw.memsize unavailable")
	}

	swapUsed, swapTotal := parseDarwinSwap()
	if swapTotal > 0 {
		pct := float64(swapUsed) / float64(swapTotal) * 100
		out = append(out, metric{Name: "swap", Value: fmt.Sprintf("%s/%s", bytesHuman(swapUsed), bytesHuman(swapTotal)), Detail: fmt.Sprintf("%.0f%% used", pct), Severity: pctSeverity(pct, 45, 75)})
	} else {
		out = append(out, metric{Name: "swap", Value: "unknown", Detail: "vm.swapusage", Severity: sevUnknown})
	}

	vm, err := run(2*time.Second, "vm_stat")
	if err == nil {
		pageSize := int64(16384)
		if strings.Contains(vm, "page size of") {
			pageSize = parseVMPageSize(vm, pageSize)
		}
		free := parseVMStat(vm, "Pages free") + parseVMStat(vm, "Pages speculative")
		wired := parseVMStat(vm, "Pages wired down")
		comp := parseVMStat(vm, "Pages occupied by compressor")
		if memBytes > 0 {
			used := memBytes - free*pageSize
			out = append([]metric{{Name: "ram", Value: fmt.Sprintf("%s/%s", bytesHuman(used), bytesHuman(memBytes)), Detail: fmt.Sprintf("%d%% not free", int(float64(used)/float64(memBytes)*100)), Severity: pctSeverity(float64(used)/float64(memBytes)*100, 82, 92)}}, out...)
		}
		out = append(out,
			metric{Name: "wired", Value: bytesHuman(wired * pageSize), Detail: "system/GPU/kernel", Severity: pctSeverity(percentOf(wired*pageSize, memBytes), 35, 50)},
			metric{Name: "compressor", Value: bytesHuman(comp * pageSize), Detail: "compressed pages", Severity: pctSeverity(percentOf(comp*pageSize, memBytes), 12, 20)},
		)
	} else {
		warnings = append(warnings, "vm_stat unavailable")
	}

	load := loadAverage()
	if load != "" {
		out = append(out, metric{Name: "load", Value: load, Detail: "1m/5m/15m", Severity: sevOK})
	}
	return out, warnings
}

func collectLinuxMemory() ([]metric, []string) {
	mem := readMeminfo()
	total := mem["MemTotal"] * 1024
	avail := mem["MemAvailable"] * 1024
	swapTotal := mem["SwapTotal"] * 1024
	swapFree := mem["SwapFree"] * 1024
	var out []metric
	if total > 0 {
		used := total - avail
		pct := float64(used) / float64(total) * 100
		out = append(out, metric{Name: "ram", Value: fmt.Sprintf("%s/%s", bytesHuman(used), bytesHuman(total)), Detail: fmt.Sprintf("%.0f%% pressure", pct), Severity: pctSeverity(pct, 82, 92)})
	}
	if swapTotal > 0 {
		swapUsed := swapTotal - swapFree
		pct := float64(swapUsed) / float64(swapTotal) * 100
		out = append(out, metric{Name: "swap", Value: fmt.Sprintf("%s/%s", bytesHuman(swapUsed), bytesHuman(swapTotal)), Detail: fmt.Sprintf("%.0f%% used", pct), Severity: pctSeverity(pct, 45, 75)})
	}
	if load := loadAverage(); load != "" {
		out = append(out, metric{Name: "load", Value: load, Detail: "1m/5m/15m", Severity: sevOK})
	}
	return out, nil
}

func collectProcesses() ([]processInfo, []string) {
	if runtime.GOOS == "darwin" {
		out, err := run(4*time.Second, "top", "-l", "1", "-o", "mem", "-n", "40", "-stats", "pid,command,cpu,mem,rsize,vsize,threads")
		if err == nil {
			return parseDarwinTop(out), nil
		}
		psOut, psErr := run(3*time.Second, "ps", "-axo", "pid,comm,rss,%cpu")
		if psErr == nil {
			return parsePS(psOut), []string{"top unavailable; using ps"}
		}
		return nil, []string{"process list unavailable"}
	}

	out, err := run(3*time.Second, "ps", "-axo", "pid,comm,rss,%cpu", "--sort=-rss")
	if err != nil {
		out, err = run(3*time.Second, "ps", "-axo", "pid,comm,rss,%cpu")
	}
	if err != nil {
		return nil, []string{"process list unavailable"}
	}
	return parsePS(out), nil
}

func collectProcessArgs() ([]processArgs, []string) {
	out, err := run(3*time.Second, "ps", "-axo", "pid,comm,args")
	if err != nil {
		return nil, []string{"full ps unavailable"}
	}
	var procs []processArgs
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		fields := strings.Fields(line)
		if len(fields) < 3 || !isDigits(fields[0]) {
			continue
		}
		procs = append(procs, processArgs{PID: fields[0], Comm: fields[1], Args: strings.Join(fields[2:], " ")})
	}
	return procs, nil
}

func collectTmuxSessions(procs []processArgs) ([]sessionInfo, []string) {
	out, err := run(2*time.Second, "tmux", "ls")
	if err != nil {
		return nil, nil
	}
	var sessions []sessionInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := strings.SplitN(line, ":", 2)[0]
		state := "detached"
		if strings.Contains(line, "attached") {
			state = "attached"
		}
		sessions = append(sessions, sessionInfo{
			Name:   name,
			State:  state,
			Agents: sessionAgents(name, procs),
			Age:    "",
		})
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].Name < sessions[j].Name })
	return sessions, nil
}

func sessionAgents(name string, procs []processArgs) string {
	counts := map[string]int{}
	for _, p := range procs {
		l := strings.ToLower(p.Args)
		if strings.Contains(l, strings.ToLower(name)) {
			switch {
			case strings.Contains(l, "masko-agent-wrap.sh codex"):
				counts["codex"]++
			case strings.Contains(l, "masko-agent-wrap.sh claudecode") || strings.Contains(l, "masko-agent-wrap.sh claude"):
				counts["claude"]++
			case isClaudeProcess(p):
				counts["claude"]++
			case isCodexProcess(p):
				counts["codex"]++
			case strings.Contains(l, "test-tui"):
				counts["test"]++
			case strings.Contains(l, "oracle-tui"):
				counts["oracle"]++
			}
		}
	}
	if len(counts) == 0 {
		return "shell"
	}
	keys := []string{"claude", "codex", "test", "oracle"}
	var parts []string
	for _, k := range keys {
		if counts[k] > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", k, counts[k]))
		}
	}
	return strings.Join(parts, " ")
}

func isClaudeProcess(p processArgs) bool {
	comm := baseName(p.Comm)
	l := strings.ToLower(p.Args)
	if strings.Contains(l, "claude helper") || strings.Contains(l, "claude.app/contents") {
		return false
	}
	return comm == "claude" || comm == "claude.exe"
}

func isCodexProcess(p processArgs) bool {
	comm := baseName(p.Comm)
	l := strings.ToLower(p.Args)
	if strings.Contains(l, "codexbar") || strings.Contains(l, "app-server") || strings.Contains(l, "visual studio code") || strings.Contains(l, "code helper") {
		return false
	}
	if comm == "codex" {
		return !strings.Contains(l, "@openai/codex")
	}
	return comm == "node" && strings.Contains(l, "/bin/codex") && !strings.Contains(l, "@openai/codex")
}

func parseDarwinTop(out string) []processInfo {
	var procs []processInfo
	sc := bufio.NewScanner(strings.NewReader(out))
	inRows := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "PID ") {
			inRows = true
			continue
		}
		if !inRows || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 || !isDigits(fields[0]) {
			continue
		}
		pid := fields[0]
		cpuIdx := -1
		for i := 1; i < len(fields); i++ {
			if strings.HasSuffix(fields[i], "%") || looksFloat(fields[i]) {
				cpuIdx = i
				break
			}
		}
		if cpuIdx < 2 || cpuIdx+2 >= len(fields) {
			continue
		}
		name := strings.Join(fields[1:cpuIdx], " ")
		procs = append(procs, processInfo{
			PID:     pid,
			Name:    name,
			CPU:     strings.TrimSuffix(fields[cpuIdx], "%"),
			Memory:  fields[cpuIdx+1],
			Threads: fields[len(fields)-1],
			Kind:    processKind(name),
		})
	}
	return procs
}

func parsePS(out string) []processInfo {
	var procs []processInfo
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		fields := strings.Fields(line)
		if len(fields) < 4 || !isDigits(fields[0]) {
			continue
		}
		rssKB := parseInt64(fields[len(fields)-2])
		name := strings.Join(fields[1:len(fields)-2], " ")
		procs = append(procs, processInfo{
			PID:    fields[0],
			Name:   name,
			Memory: bytesHuman(rssKB * 1024),
			CPU:    fields[len(fields)-1],
			Kind:   processKind(name),
		})
	}
	sort.Slice(procs, func(i, j int) bool { return memSortValue(procs[i].Memory) > memSortValue(procs[j].Memory) })
	if len(procs) > 40 {
		return procs[:40]
	}
	return procs
}

func processKind(name string) string {
	l := strings.ToLower(name)
	switch {
	case strings.Contains(l, "claude"):
		return "agent"
	case strings.Contains(l, "codex"):
		return "agent"
	case strings.Contains(l, "ghostty") || strings.Contains(l, "tmux"):
		return "term"
	case strings.Contains(l, "windowserver"):
		return "gui"
	case strings.Contains(l, "arc") || strings.Contains(l, "browser helper"):
		return "browser"
	case strings.Contains(l, "code helper") || strings.Contains(l, "visual studio code"):
		return "ide"
	case strings.Contains(l, "node") || strings.Contains(l, "tsserver") || strings.Contains(l, "pyright"):
		return "dev"
	case strings.Contains(l, "test-tui") || strings.Contains(l, "oracle-tui"):
		return "agent-ui"
	default:
		return "app"
	}
}

func processKindRu(kind string) string {
	switch kind {
	case "agent":
		return "агент"
	case "term":
		return "терминал"
	case "gui":
		return "окна"
	case "browser":
		return "браузер"
	case "ide":
		return "IDE"
	case "dev":
		return "dev"
	case "agent-ui":
		return "панель"
	default:
		return "app"
	}
}

func recommendations(s snapshot) []string {
	var out []string
	if s.ActiveAgentCount > s.LocalAgentLimit {
		out = append(out, fmt.Sprintf("active agents exceed local limit: %d/%d; sleep or move extra sessions", s.ActiveAgentCount, s.LocalAgentLimit))
	}
	if metricAtLeast(s.Memory, "swap", sevWarn) {
		out = append(out, "swap is high; killing idle processes helps more than detaching tmux")
	}
	if metricAtLeast(s.Memory, "compressor", sevWarn) {
		out = append(out, "compressor is high; expect lag when resuming old sessions")
	}
	if s.TmuxSessionCount > 6 {
		out = append(out, "many tmux sessions are alive; prefer paused/checkpointed worktrees")
	}
	if len(out) == 0 {
		out = append(out, "within configured limits")
	}
	return out
}

func translateNote(note string) string {
	switch {
	case strings.HasPrefix(note, "active agents exceed local limit"):
		return "живых агентов больше лимита; лишние лучше усыпить или закрыть"
	case strings.HasPrefix(note, "swap is high"):
		return "swap высокий; закрытие idle-процессов полезнее, чем detach tmux"
	case strings.HasPrefix(note, "compressor is high"):
		return "compressor высокий; старые сессии будут лагать при возврате"
	case strings.HasPrefix(note, "many tmux sessions"):
		return "много живых tmux-сессий; лучше paused/checkpoint вместо живых процессов"
	case strings.HasPrefix(note, "within configured limits"):
		return "в пределах настроенных лимитов"
	default:
		return note
	}
}

func run(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", ctx.Err()
	}
	return strings.TrimSpace(string(out)), err
}

func parseDarwinSwap() (used int64, total int64) {
	out, err := run(2*time.Second, "sysctl", "vm.swapusage")
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(out)
	for i := 0; i < len(fields)-2; i++ {
		switch fields[i] {
		case "total":
			total = parseFloatMB(fields[i+2])
		case "used":
			used = parseFloatMB(fields[i+2])
		}
	}
	return used, total
}

func parseFloatMB(s string) int64 {
	f, _ := strconv.ParseFloat(strings.TrimRight(s, "M"), 64)
	return int64(f * 1024 * 1024)
}

func parseVMPageSize(s string, fallback int64) int64 {
	idx := strings.Index(s, "page size of")
	if idx < 0 {
		return fallback
	}
	rest := s[idx:]
	for _, f := range strings.Fields(rest) {
		if isDigits(f) {
			return parseInt64(f)
		}
	}
	return fallback
}

func parseVMStat(s, key string) int64 {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+":") {
			v := strings.TrimSpace(strings.TrimSuffix(strings.SplitN(line, ":", 2)[1], "."))
			return parseInt64(v)
		}
	}
	return 0
}

func readMeminfo() map[string]int64 {
	out := map[string]int64{}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 {
			out[strings.TrimSuffix(fields[0], ":")] = parseInt64(fields[1])
		}
	}
	return out
}

func loadAverage() string {
	if runtime.GOOS == "linux" {
		if b, err := os.ReadFile("/proc/loadavg"); err == nil {
			fields := strings.Fields(string(b))
			if len(fields) >= 3 {
				return strings.Join(fields[:3], " ")
			}
		}
	}
	out, err := run(2*time.Second, "sysctl", "-n", "vm.loadavg")
	if err == nil {
		out = strings.Trim(out, "{} ")
		fields := strings.Fields(out)
		if len(fields) >= 3 {
			return strings.Join(fields[:3], " ")
		}
	}
	return ""
}

func countProcess(procs []processInfo, needle string) int {
	n := 0
	for _, p := range procs {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(needle)) {
			n++
		}
	}
	return n
}

func countMatchingArgs(procs []processArgs, match func(processArgs) bool) int {
	n := 0
	for _, p := range procs {
		if match(p) {
			n++
		}
	}
	return n
}

func firstProcesses(in []processInfo, n int) []processInfo {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func firstSessions(in []sessionInfo, n int) []sessionInfo {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func hasHot(metrics []metric) bool {
	for _, m := range metrics {
		if m.Severity >= sevWarn && m.Severity != sevUnknown {
			return true
		}
	}
	return false
}

func maxMetricSeverity(metrics []metric) severity {
	maxSev := sevOK
	for _, m := range metrics {
		if m.Severity != sevUnknown && m.Severity > maxSev {
			maxSev = m.Severity
		}
	}
	return maxSev
}

func metricAtLeast(metrics []metric, name string, sev severity) bool {
	for _, m := range metrics {
		if m.Name == name && m.Severity >= sev {
			return true
		}
	}
	return false
}

func pctSeverity(pct, warn, critical float64) severity {
	switch {
	case pct >= critical:
		return sevCritical
	case pct >= warn:
		return sevWarn
	default:
		return sevOK
	}
}

func limitSeverity(value, limit int) severity {
	if limit <= 0 {
		return sevUnknown
	}
	switch {
	case value > limit:
		return sevCritical
	case value == limit:
		return sevWarn
	default:
		return sevOK
	}
}

func sevForCount(value, warn, critical int) severity {
	switch {
	case value >= critical:
		return sevCritical
	case value >= warn:
		return sevWarn
	default:
		return sevOK
	}
}

func styleSeverity(sev severity) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(severityColor(sev))
}

func severityColor(sev severity) lipgloss.Color {
	switch sev {
	case sevOK:
		return okColor
	case sevWarn:
		return warnColor
	case sevCritical:
		return badColor
	default:
		return muted
	}
}

func severityLabel(sev severity) string {
	switch sev {
	case sevOK:
		return "ok"
	case sevWarn:
		return "warn"
	case sevCritical:
		return "critical"
	default:
		return "unknown"
	}
}

func symbol(sev severity) string {
	switch sev {
	case sevOK:
		return "●"
	case sevWarn:
		return "▲"
	case sevCritical:
		return "■"
	default:
		return "○"
	}
}

func percentOf(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(value) / float64(total) * 100
}

func bytesHuman(b int64) string {
	if b <= 0 {
		return "0B"
	}
	units := []string{"B", "K", "M", "G", "T"}
	f := float64(b)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i <= 1 {
		return fmt.Sprintf("%.0f%s", f, units[i])
	}
	return fmt.Sprintf("%.1f%s", f, units[i])
}

func memSortValue(s string) float64 {
	if s == "" {
		return 0
	}
	mult := 1.0
	switch s[len(s)-1] {
	case 'G':
		mult = 1024
	case 'M':
		mult = 1
	case 'K':
		mult = 1.0 / 1024
	}
	f, _ := strconv.ParseFloat(strings.TrimRight(s, "GMKB"), 64)
	return f * mult
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	rs := []rune(s)
	for lipgloss.Width(string(rs)+"…") > width && len(rs) > 0 {
		rs = rs[:len(rs)-1]
	}
	return string(rs) + "…"
}

func parseInt64(s string) int64 {
	s = strings.TrimSpace(strings.TrimRight(s, "."))
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func looksFloat(s string) bool {
	s = strings.TrimSuffix(s, "%")
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func emptyDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func baseName(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	return strings.ToLower(path)
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func round(f float64) int {
	return int(math.Round(f))
}
