package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type testCommand struct {
	Label string
	Cmd   string
}

type runLog struct {
	Path    string
	Name    string
	Command string
	Exit    string
	ModTime time.Time
}

type dataLoadedMsg struct {
	commands []testCommand
	logs     []runLog
}

type runDoneMsg struct {
	File string
	Err  error
}

type statusMsg string

type tailTickMsg struct{}

func tailTick() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg { return tailTickMsg{} })
}

type model struct {
	repo      string
	outDir    string
	lastFile  string
	suggested string

	input   textinput.Model
	spinner spinner.Model

	commands []testCommand
	filtered []int
	selected int
	logs     []runLog
	logTop   int

	// run navigation + live tail
	logSelected int    // which past run is shown (0 = newest)
	runningFile string // path of the in-progress run's log
	followTail  bool   // auto-scroll the log to the end while running
	startedAt   time.Time

	width   int
	height  int
	running bool
	status  string
}

var (
	bg        = lipgloss.Color("#181b24")
	panel     = lipgloss.Color("#222635")
	accent    = lipgloss.Color("#9adbc5")
	accent2   = lipgloss.Color("#f3c969")
	muted     = lipgloss.Color("#8f96aa")
	text      = lipgloss.Color("#e8eaf2")
	dim       = lipgloss.Color("#5d6374")
	warning   = lipgloss.Color("#f0c674")
	errorTint = lipgloss.Color("#ff8fa3")
	okTint    = lipgloss.Color("#9adbc5")

	screenStyle = lipgloss.NewStyle().Background(bg).Foreground(text)
	titleStyle  = lipgloss.NewStyle().Foreground(accent).Bold(true)
	inputStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1).Background(panel)
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dim).Padding(0, 1).Background(panel)
	boxActive   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1).Background(panel)
	helpStyle   = lipgloss.NewStyle().Foreground(muted)

	badgeOK   = lipgloss.NewStyle().Foreground(bg).Background(okTint).Bold(true).Padding(0, 1)
	badgeFail = lipgloss.NewStyle().Foreground(bg).Background(errorTint).Bold(true).Padding(0, 1)
	badgeRun  = lipgloss.NewStyle().Foreground(bg).Background(accent2).Bold(true).Padding(0, 1)

	lineOK   = lipgloss.NewStyle().Foreground(okTint)
	lineFail = lipgloss.NewStyle().Foreground(errorTint)
	lineWarn = lipgloss.NewStyle().Foreground(warning)

	rePass = regexp.MustCompile(`(?i)\b(pass(ed)?|ok|success(ful)?|✓|✔|\d+ passed)\b`)
	reFail = regexp.MustCompile(`(?i)\b(fail(ed|ure)?|error|panic|traceback|assert(ion)?|✗|✘|\d+ failed|exception)\b`)
	reWarn = regexp.MustCompile(`(?i)\b(warn(ing)?|skip(ped)?|deprecat)\b`)
)

// highlightLine colors a log line by pass/fail/warn keywords.
func highlightLine(s string) string {
	switch {
	case reFail.MatchString(s):
		return lineFail.Render(s)
	case rePass.MatchString(s):
		return lineOK.Render(s)
	case reWarn.MatchString(s):
		return lineWarn.Render(s)
	}
	return s
}

// excludeAgentsDir hides the tooling's .agents/ output from git via the repo's
// info/exclude (works for both a normal checkout and a worktree). Otherwise
// untracked .agents/ makes the worktree look dirty and pins it in reap "hold".
func excludeAgentsDir(repo string) {
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--git-path", "info/exclude").Output()
	if err != nil {
		return
	}
	excl := strings.TrimSpace(string(out))
	if !filepath.IsAbs(excl) {
		excl = filepath.Join(repo, excl)
	}
	if data, err := os.ReadFile(excl); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == ".agents/" {
				return
			}
		}
	}
	_ = os.MkdirAll(filepath.Dir(excl), 0o755)
	f, err := os.OpenFile(excl, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(".agents/\n")
}

func main() {
	repo := "."
	if len(os.Args) > 1 {
		repo = os.Args[1]
	}
	absRepo, err := filepath.Abs(repo)
	if err == nil {
		repo = absRepo
	}
	suggested := ""
	if len(os.Args) > 2 {
		suggested = os.Args[2]
	}

	outDir := filepath.Join(repo, ".agents", "test-runs")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create test output directory: %s\n", err)
		os.Exit(1)
	}
	_ = os.WriteFile(filepath.Join(outDir, ".gitignore"), []byte("*\n!.gitignore\n"), 0o644)
	excludeAgentsDir(repo)

	input := textinput.New()
	input.Placeholder = "Filter tests or type command..."
	input.Focus()
	input.CharLimit = 800
	input.Prompt = "› "
	input.PromptStyle = lipgloss.NewStyle().Foreground(accent)
	input.TextStyle = lipgloss.NewStyle().Foreground(text)
	input.PlaceholderStyle = lipgloss.NewStyle().Foreground(dim)

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(accent2)

	m := model{
		repo:      repo,
		outDir:    outDir,
		lastFile:  filepath.Join(outDir, "latest.md"),
		suggested: suggested,
		input:     input,
		spinner:   spin,
		status:    "Ready",
	}

	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "test tui failed: %s\n", err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, loadData(m.repo, m.outDir, m.suggested))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m.recompute(), nil
	case dataLoadedMsg:
		m.commands = msg.commands
		m.logs = msg.logs
		return m.recompute(), nil
	case runDoneMsg:
		m.running = false
		m.runningFile = ""
		m.logSelected = 0
		if msg.Err != nil {
			m.status = "Failed: " + shortPath(m.repo, msg.File)
		} else {
			m.status = "Passed: " + shortPath(m.repo, msg.File)
		}
		return m.recompute(), loadData(m.repo, m.outDir, m.suggested)
	case statusMsg:
		m.status = string(msg)
		return m, nil
	case tailTickMsg:
		if !m.running {
			return m, nil
		}
		return m, tailTick()
	case spinner.TickMsg:
		if !m.running {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			return m, tea.Quit
		case "esc":
			m.input.SetValue("")
			m.status = "Ready"
			return m.recompute(), nil
		case "up":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down":
			if m.selected < len(m.filtered)-1 {
				m.selected++
			}
			return m, nil
		case "pgup", "ctrl+u":
			m.followTail = false
			m.logTop = max(0, m.logTop-previewStep(m.height))
			return m, nil
		case "pgdown", "ctrl+d":
			m.logTop += previewStep(m.height)
			return m, nil
		case "[", "shift+tab":
			// older run
			if !m.running && m.logSelected < len(m.logs)-1 {
				m.logSelected++
				m.logTop = 0
				m.followTail = false
			}
			return m, nil
		case "]", "tab":
			// newer run
			if !m.running && m.logSelected > 0 {
				m.logSelected--
				m.logTop = 0
				m.followTail = false
			}
			return m, nil
		case "ctrl+r":
			m.status = "Refreshed"
			return m.recompute(), loadData(m.repo, m.outDir, m.suggested)
		case "ctrl+y":
			if selected, ok := m.currentLog(); ok {
				m.status = "Copied: " + selected.Name
				return m, copyFile(selected.Path)
			}
			return m, nil
		case "ctrl+o":
			if selected, ok := m.currentLog(); ok {
				m.status = "Opened: " + selected.Name
				return m, openFile(selected.Path)
			}
			return m, nil
		case "enter":
			if m.running {
				return m, nil
			}
			if cmd, ok := m.commandToRun(); ok {
				m.running = true
				m.followTail = true
				m.logSelected = 0
				m.logTop = 0
				m.startedAt = time.Now()
				m.status = "Running: " + cmd.Label
				outputFile := filepath.Join(m.outDir, timestamp()+"-"+slugify(cmd.Label)+".md")
				m.runningFile = outputFile
				return m, tea.Batch(m.spinner.Tick, tailTick(), runTest(m.repo, outputFile, m.lastFile, cmd))
			}
			m.status = "No command selected"
			return m, nil
		}
	}

	old := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if old != m.input.Value() {
		m = m.recompute()
	}
	return m, cmd
}

func (m model) recompute() model {
	query := strings.ToLower(strings.TrimSpace(m.input.Value()))
	m.filtered = m.filtered[:0]
	for i, command := range m.commands {
		haystack := strings.ToLower(command.Label + " " + command.Cmd)
		if query == "" || strings.Contains(haystack, query) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.selected >= len(m.filtered) {
		m.selected = max(0, len(m.filtered)-1)
	}
	return m
}

func (m model) View() string {
	w := m.width
	if w <= 0 {
		w = 96
	}
	inner := max(40, w-6)

	header := renderHeader(m, inner)
	inputBox := inputStyle.Width(inner - 2).Render(m.input.View())
	status := renderStatus(m, inner)
	content := renderContent(m, inner)
	help := helpStyle.Width(inner).Render("Enter run · ↑↓ select · [ ] runs · PgUp/PgDn log · ^Y copy · ^O open · ^R refresh · ^Q quit")

	body := lipgloss.JoinVertical(lipgloss.Left, header, inputBox, status, content, help)
	return screenStyle.Width(w).Height(max(m.height, lipgloss.Height(body))).Render(lipgloss.PlaceHorizontal(w, lipgloss.Left, body))
}

func renderHeader(m model, width int) string {
	repoName := filepath.Base(m.repo)
	left := titleStyle.Render("test")
	right := lipgloss.NewStyle().Foreground(muted).Render(repoName + "  ·  " + shortPath(m.repo, m.outDir))
	line := lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat("─", max(1, width-lipgloss.Width(left)-lipgloss.Width(right)-4)))
	return lipgloss.JoinHorizontal(lipgloss.Center, "  ", left, " ", line, " ", right)
}

func renderStatus(m model, width int) string {
	style := lipgloss.NewStyle().Foreground(muted).Padding(0, 2).Width(width)
	status := truncate(m.status, max(20, width-4))
	if strings.HasPrefix(m.status, "Failed") {
		style = style.Foreground(errorTint)
	}
	if strings.HasPrefix(m.status, "Passed") {
		style = style.Foreground(okTint)
	}
	if m.running {
		return style.Foreground(warning).Render(m.spinner.View() + " " + truncate(m.status, max(18, width-6)))
	}
	return style.Render(status)
}

func renderContent(m model, width int) string {
	height := max(6, m.height-9)
	if m.height <= 0 {
		height = 12
	}
	if width >= 104 {
		listWidth := max(36, width/3)
		previewWidth := max(42, width-listWidth-4)
		return lipgloss.JoinHorizontal(lipgloss.Top, renderCommands(m, listWidth, height), "  ", renderLog(m, previewWidth, height))
	}
	listHeight := max(5, height/2)
	previewHeight := max(5, height-listHeight-1)
	return lipgloss.JoinVertical(lipgloss.Left, renderCommands(m, width, listHeight), renderLog(m, width, previewHeight))
}

func renderCommands(m model, width, height int) string {
	if len(m.commands) == 0 {
		return boxStyle.Width(width - 2).Height(height).Render(lipgloss.NewStyle().Foreground(dim).Padding(2, 0).Render("No test commands detected."))
	}
	if len(m.filtered) == 0 {
		return boxStyle.Width(width - 2).Height(height).Render(lipgloss.NewStyle().Foreground(dim).Padding(2, 0).Render("No matching commands. Enter runs typed command."))
	}

	maxRows := height - 2
	start := 0
	if m.selected >= maxRows {
		start = m.selected - maxRows + 1
	}
	end := min(len(m.filtered), start+maxRows)

	rows := make([]string, 0, maxRows)
	for row := start; row < end; row++ {
		command := m.commands[m.filtered[row]]
		cursor := "  "
		rowStyle := lipgloss.NewStyle().Foreground(text)
		if row == m.selected {
			cursor = "› "
			rowStyle = rowStyle.Foreground(bg).Background(accent).Bold(true)
		}
		rowWidth := max(32, width-6)
		labelWidth := min(22, max(10, rowWidth/3))
		cmdWidth := max(12, rowWidth-labelWidth-4)
		line := fmt.Sprintf("%s%-*s  %s", cursor, labelWidth, truncate(command.Label, labelWidth), truncate(command.Cmd, cmdWidth))
		rows = append(rows, rowStyle.Width(width-6).Render(line))
	}

	return boxStyle.Width(width - 2).Height(height).Render(strings.Join(rows, "\n"))
}

func renderLog(m model, width, height int) string {
	box := boxStyle
	var badge, name, lines = "", "", []string{lipgloss.NewStyle().Foreground(dim).Render("Run a test command to preview its log here.")}
	pos := ""

	if m.running && m.runningFile != "" {
		box = boxActive
		badge = badgeRun.Render(m.spinner.View() + " RUN " + time.Since(m.startedAt).Round(time.Second).String())
		name = truncate(filepath.Base(m.runningFile), max(12, width-26))
		lines = readPreviewLines(m.runningFile)
		if len(lines) == 0 {
			lines = []string{lipgloss.NewStyle().Foreground(dim).Render("…starting…")}
		}
	} else if sel, ok := m.currentLog(); ok {
		if sel.Exit == "0" {
			badge = badgeOK.Render("PASS")
		} else {
			badge = badgeFail.Render("FAIL " + sel.Exit)
		}
		name = truncate(sel.Name, max(12, width-30))
		if len(m.logs) > 1 {
			pos = lipgloss.NewStyle().Foreground(muted).Render(fmt.Sprintf("  run %d/%d", m.logSelected+1, len(m.logs)))
		}
		lines = readPreviewLines(sel.Path)
		if len(lines) == 0 {
			lines = []string{lipgloss.NewStyle().Foreground(dim).Render("Empty log.")}
		}
	}

	visible := max(1, height-3)
	top := m.logTop
	if m.followTail {
		top = max(0, len(lines)-visible)
	}
	top = min(max(0, top), max(0, len(lines)-visible))
	end := min(len(lines), top+visible)
	rendered := make([]string, 0, visible)
	for _, line := range lines[top:end] {
		rendered = append(rendered, highlightLine(truncate(line, max(8, width-6))))
	}
	for len(rendered) < visible {
		rendered = append(rendered, "")
	}

	scroll := ""
	if len(lines) > visible {
		scroll = lipgloss.NewStyle().Foreground(muted).Render(fmt.Sprintf("  %d-%d/%d", top+1, end, len(lines)))
	}
	header := lipgloss.JoinHorizontal(lipgloss.Center,
		badge, "  ",
		lipgloss.NewStyle().Foreground(accent2).Bold(true).Render(name),
		pos, scroll,
	)
	return box.Width(width - 2).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, header, strings.Join(rendered, "\n")))
}

func (m model) commandToRun() (testCommand, bool) {
	query := strings.TrimSpace(m.input.Value())
	if len(m.filtered) > 0 {
		return m.commands[m.filtered[m.selected]], true
	}
	if query != "" {
		return testCommand{Label: "manual", Cmd: query}, true
	}
	return testCommand{}, false
}

func (m model) latestLog() (runLog, bool) {
	if len(m.logs) == 0 {
		return runLog{}, false
	}
	return m.logs[0], true
}

// currentLog is the run shown in the preview (selected, clamped).
func (m model) currentLog() (runLog, bool) {
	if len(m.logs) == 0 {
		return runLog{}, false
	}
	i := m.logSelected
	if i < 0 {
		i = 0
	}
	if i >= len(m.logs) {
		i = len(m.logs) - 1
	}
	return m.logs[i], true
}

func loadData(repo, outDir, suggested string) tea.Cmd {
	return func() tea.Msg {
		return dataLoadedMsg{
			commands: detectCommands(repo, suggested),
			logs:     loadLogs(outDir),
		}
	}
}

func detectCommands(repo, suggested string) []testCommand {
	var commands []testCommand
	seen := map[string]bool{}
	add := func(label, cmd string) {
		label = strings.TrimSpace(label)
		cmd = strings.TrimSpace(cmd)
		if label == "" || cmd == "" || seen[cmd] {
			return
		}
		seen[cmd] = true
		commands = append(commands, testCommand{Label: label, Cmd: cmd})
	}

	for _, c := range readCommandsTOML(filepath.Join(repo, ".agents", "commands.toml")) {
		add(c.Label, c.Cmd)
	}
	for _, c := range readMakefile(repo) {
		add(c.Label, c.Cmd)
	}
	for _, c := range readPackageJSON(filepath.Join(repo, "package.json")) {
		add(c.Label, c.Cmd)
	}
	addPythonDefaults(repo, add)
	if suggested != "" && !strings.Contains(suggested, "no test watcher auto-detected") {
		add("launch suggested", suggested)
	}
	if exists(filepath.Join(repo, "go.mod")) {
		add("go test all", "go test ./...")
	}
	if exists(filepath.Join(repo, "Cargo.toml")) {
		add("cargo test", "cargo test")
		add("cargo check", "cargo check")
	}
	return commands
}

func readCommandsTOML(path string) []testCommand {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var commands []testCommand
	var label, cmd string
	flush := func() {
		if label != "" && cmd != "" {
			commands = append(commands, testCommand{Label: label, Cmd: cmd})
		}
		label, cmd = "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "[[commands]]" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "label") {
			label = cleanAssignment(line)
		}
		if strings.HasPrefix(line, "cmd") {
			cmd = cleanAssignment(line)
		}
	}
	flush()
	return commands
}

func readMakefile(repo string) []testCommand {
	data, err := os.ReadFile(filepath.Join(repo, "Makefile"))
	if err != nil {
		return nil
	}
	// All real targets, not a whitelist. Skip pattern rules (%), special
	// targets (.PHONY etc.), and variable assignments.
	re := regexp.MustCompile(`^([a-zA-Z0-9][a-zA-Z0-9_.-]*):(?:[^=]|$)`)
	var commands []testCommand
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
			continue
		}
		match := re.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}
		name := match[1]
		if name == "PHONY" || strings.HasPrefix(name, ".") || strings.Contains(name, "%") || seen[name] {
			continue
		}
		seen[name] = true
		commands = append(commands, testCommand{Label: "make " + name, Cmd: "make " + name})
	}
	return commands
}

func readPackageJSON(path string) []testCommand {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var parsed struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return nil
	}
	allowed := regexp.MustCompile(`^(dev|start|test|test:.*|lint|format|typecheck|check|build|preview)$`)
	var names []string
	for name := range parsed.Scripts {
		if allowed.MatchString(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	commands := make([]testCommand, 0, len(names))
	for _, name := range names {
		commands = append(commands, testCommand{Label: name, Cmd: "npm run " + name})
	}
	return commands
}

func addPythonDefaults(repo string, add func(string, string)) {
	if !exists(filepath.Join(repo, "pyproject.toml")) && !exists(filepath.Join(repo, "setup.py")) {
		return
	}
	add("pytest all", "uv run pytest -v --tb=short")
	if exists(filepath.Join(repo, "tests", "unit")) {
		add("pytest unit", "uv run pytest tests/unit -v --tb=short")
	}
	if exists(filepath.Join(repo, "tests", "integration")) {
		add("pytest integration", "uv run pytest tests/integration -v --tb=short")
	}
	add("ruff check", "uv run ruff check .")
	add("ruff format", "uv run ruff format .")
	if data, err := os.ReadFile(filepath.Join(repo, "pyproject.toml")); err == nil && strings.Contains(string(data), "mypy") {
		add("mypy", "uv run mypy .")
	}
}

func loadLogs(outDir string) []runLog {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil
	}
	logs := make([]runLog, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "latest.md" {
			continue
		}
		path := filepath.Join(outDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		command, exit := readRunMeta(path)
		logs = append(logs, runLog{Path: path, Name: entry.Name(), Command: command, Exit: exit, ModTime: info.ModTime()})
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].ModTime.After(logs[j].ModTime)
	})
	return logs
}

func runTest(repo, outputFile, lastFile string, command testCommand) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()

		// Write the header first, then stream the process output straight into
		// the same file so the tail-tick can show it live as it grows.
		f, err := os.Create(outputFile)
		if err != nil {
			return runDoneMsg{File: outputFile, Err: err}
		}
		fmt.Fprintf(f, "# Test Run\n\n")
		fmt.Fprintf(f, "- Time: `%s`\n", start.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(f, "- Repo: `%s`\n", repo)
		fmt.Fprintf(f, "- Label: `%s`\n", command.Label)
		fmt.Fprintf(f, "- Command: `%s`\n\n---\n\n## Output\n\n", command.Cmd)
		_ = f.Sync()

		cmd := exec.Command("bash", "-lc", command.Cmd)
		cmd.Dir = repo
		cmd.Stdout = f
		cmd.Stderr = f
		runErr := cmd.Run()

		exitText := "0"
		if runErr != nil {
			exitText = runErr.Error()
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				exitText = fmt.Sprintf("%d", exitErr.ExitCode())
			}
		}
		fmt.Fprintf(f, "\n\n---\n\n- Exit: `%s`\n", exitText)
		fmt.Fprintf(f, "- Duration: `%s`\n", time.Since(start).Round(time.Millisecond))
		_ = f.Close()

		if data, e := os.ReadFile(outputFile); e == nil {
			_ = os.WriteFile(lastFile, data, 0o644)
		}
		return runDoneMsg{File: outputFile, Err: runErr}
	}
}

func cleanTestOutput(out []byte) []byte {
	s := strings.ReplaceAll(string(out), "\r\n", "\n")
	return []byte(strings.TrimRight(s, "\n") + "\n")
}

func copyFile(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return statusMsg("Copy failed: " + err.Error())
		}
		cmd := exec.Command("pbcopy")
		cmd.Stdin = bytes.NewReader(data)
		if err := cmd.Run(); err != nil {
			return statusMsg("Copy failed: " + err.Error())
		}
		return statusMsg("Copied: " + filepath.Base(path))
	}
}

func openFile(path string) tea.Cmd {
	return func() tea.Msg {
		if err := exec.Command("open", path).Run(); err != nil {
			return statusMsg("Open failed: " + err.Error())
		}
		return statusMsg("Opened: " + filepath.Base(path))
	}
}

func readPreviewLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"Cannot read file: " + err.Error()}
	}
	raw := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, strings.TrimRight(line, "\r"))
	}
	return lines
}

func readRunMeta(path string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var command, exit string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- Command: `") && strings.HasSuffix(line, "`") {
			command = strings.TrimSuffix(strings.TrimPrefix(line, "- Command: `"), "`")
		}
		if strings.HasPrefix(line, "- Exit: `") && strings.HasSuffix(line, "`") {
			exit = strings.TrimSuffix(strings.TrimPrefix(line, "- Exit: `"), "`")
		}
	}
	return command, exit
}

func cleanAssignment(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	value := strings.TrimSpace(parts[1])
	if len(value) >= 2 {
		if value[0] == '"' && value[len(value)-1] == '"' {
			if unquoted, err := strconv.Unquote(value); err == nil {
				return unquoted
			}
		}
		if value[0] == '\'' && value[len(value)-1] == '\'' {
			value = value[1 : len(value)-1]
		}
	}
	return value
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func timestamp() string {
	return time.Now().Format("20060102-150405")
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "test"
	}
	if len([]rune(slug)) > 48 {
		return string([]rune(slug)[:48])
	}
	return slug
}

func shortPath(repo, path string) string {
	rel, err := filepath.Rel(repo, path)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:max(0, n-1)]) + "…"
}

func previewStep(height int) int {
	if height <= 0 {
		return 6
	}
	return max(3, (height-12)/2)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
