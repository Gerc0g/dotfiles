package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type answer struct {
	Path    string
	Name    string
	Prompt  string
	ModTime time.Time
}

type answersLoadedMsg []answer

type oracleDoneMsg struct {
	File string
	Err  error
}

type composeDoneMsg struct {
	File       string
	UserPrompt string
	Prompt     string
	Files      []string
	Err        error
}

type statusMsg string

type flowStage int

const (
	flowIdle flowStage = iota
	flowComposing
	flowReview
	flowSending
	flowDone
	flowError
)

type model struct {
	repo     string
	outDir   string
	reqDir   string
	lastFile string

	input    textinput.Model
	spinner  spinner.Model
	answers  []answer
	selected int

	width      int
	height     int
	running    bool
	status     string
	previewTop int
	startedAt  time.Time // when the current compose/send started

	stage             flowStage
	pendingPrompt     string
	pendingUserPrompt string
	pendingFile       string
	pendingFiles      []string
}

type elapsedTickMsg struct{}

func elapsedTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return elapsedTickMsg{} })
}

var (
	bg        = lipgloss.Color("#1b1b2a")
	panel     = lipgloss.Color("#242438")
	accent    = lipgloss.Color("#9adbc5")
	accent2   = lipgloss.Color("#f3b3c4")
	muted     = lipgloss.Color("#8f91a8")
	text      = lipgloss.Color("#e7e7f0")
	dim       = lipgloss.Color("#62647a")
	warning   = lipgloss.Color("#f0c674")
	errorTint = lipgloss.Color("#ff8fa3")

	screenStyle = lipgloss.NewStyle().Background(bg).Foreground(text)
	titleStyle  = lipgloss.NewStyle().Foreground(accent).Bold(true)
	inputStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1).Background(panel)
	listStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dim).Padding(0, 1).Background(panel)
	helpStyle   = lipgloss.NewStyle().Foreground(muted)
)

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

	outDir := filepath.Join(repo, ".agents", "oracle")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create oracle output directory: %s\n", err)
		os.Exit(1)
	}
	_ = os.WriteFile(filepath.Join(outDir, ".gitignore"), []byte("*\n!.gitignore\n"), 0o644)
	excludeAgentsDir(repo)
	reqDir := filepath.Join(outDir, "requests")
	if err := os.MkdirAll(reqDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create oracle requests directory: %s\n", err)
		os.Exit(1)
	}

	input := textinput.New()
	input.Placeholder = "Ask oracle..."
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
		repo:     repo,
		outDir:   outDir,
		reqDir:   reqDir,
		lastFile: filepath.Join(outDir, "latest.md"),
		input:    input,
		spinner:  spin,
		status:   "Ready",
	}

	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "oracle tui failed: %s\n", err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, loadAnswers(m.outDir))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case answersLoadedMsg:
		m.answers = []answer(msg)
		if m.selected >= len(m.answers) {
			m.selected = max(0, len(m.answers)-1)
		}
		return m, nil
	case oracleDoneMsg:
		m.running = false
		if msg.Err != nil {
			m.stage = flowError
		} else {
			m.stage = flowDone
		}
		if msg.Err != nil {
			m.status = "Oracle error (" + msg.Err.Error() + "): " + shortPath(m.repo, msg.File)
		} else {
			m.status = "Saved: " + shortPath(m.repo, msg.File)
		}
		m.input.SetValue("")
		m.input.Placeholder = "Ask oracle..."
		m.pendingPrompt = ""
		m.pendingUserPrompt = ""
		m.pendingFile = ""
		m.pendingFiles = nil
		m.selected = 0
		m.previewTop = 0
		return m, loadAnswers(m.outDir)
	case composeDoneMsg:
		m.running = false
		if msg.Err != nil {
			m.stage = flowError
			m.status = "Compose failed: " + msg.Err.Error()
			return m, nil
		}
		if strings.TrimSpace(msg.Prompt) == "" {
			m.stage = flowError
			m.status = "Compose failed: empty prompt"
			return m, nil
		}
		m.stage = flowReview
		m.pendingPrompt = strings.TrimSpace(msg.Prompt)
		m.pendingUserPrompt = msg.UserPrompt
		m.pendingFile = msg.File
		m.pendingFiles = msg.Files
		m.previewTop = 0
		m.input.SetValue("")
		m.input.Placeholder = "Review composed prompt. Enter sends, Esc cancels."
		m.status = fmt.Sprintf("Composed: %s · %d files · Enter sends · Esc cancels", shortPath(m.repo, msg.File), len(msg.Files))
		return m, nil
	case statusMsg:
		m.status = string(msg)
		return m, nil
	case elapsedTickMsg:
		if !m.running {
			return m, nil
		}
		return m, elapsedTick()
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
			if m.stage == flowReview {
				m.pendingPrompt = ""
				m.pendingUserPrompt = ""
				m.pendingFile = ""
				m.pendingFiles = nil
				m.stage = flowIdle
			}
			m.status = "Ready"
			m.input.SetValue("")
			m.input.Placeholder = "Ask oracle..."
			return m, nil
		case "n":
			if m.selected > 0 {
				m.selected--
				m.previewTop = 0
			}
			return m, nil
		case "p":
			if m.selected < len(m.answers)-1 {
				m.selected++
				m.previewTop = 0
			}
			return m, nil
		case "up", "k":
			m.previewTop = max(0, m.previewTop-1)
			return m, nil
		case "down", "j":
			m.previewTop++
			return m, nil
		case "pgup", "ctrl+u", "b":
			m.previewTop = max(0, m.previewTop-previewStep(m.height))
			return m, nil
		case "pgdown", "ctrl+d", "f", " ":
			m.previewTop += previewStep(m.height)
			return m, nil
		case "home":
			m.previewTop = 0
			return m, nil
		case "ctrl+r":
			m.status = "Refreshed"
			return m, loadAnswers(m.outDir)
		case "ctrl+y":
			if m.stage == flowReview && m.pendingPrompt != "" {
				m.status = "Copied composed prompt"
				return m, copyText(m.pendingPrompt)
			}
			if selected, ok := m.selectedAnswer(); ok {
				m.status = "Copied: " + selected.Name
				return m, copyFile(selected.Path)
			}
			return m, nil
		case "ctrl+o":
			if selected, ok := m.selectedAnswer(); ok {
				m.status = "Opened: " + selected.Name
				return m, openFile(selected.Path)
			}
			return m, nil
		case "enter":
			prompt := strings.TrimSpace(m.input.Value())
			if m.stage == flowReview && m.pendingPrompt != "" && !m.running {
				m.running = true
				m.stage = flowSending
				m.startedAt = time.Now()
				m.status = "Sending to oracle..."
				m.input.Placeholder = "Sending to oracle..."
				return m, tea.Batch(m.spinner.Tick, elapsedTick(), runOracle(m.repo, m.outDir, m.lastFile, m.pendingUserPrompt, m.pendingPrompt, m.pendingFiles))
			}
			if prompt != "" && !m.running {
				m.running = true
				m.stage = flowComposing
				m.startedAt = time.Now()
				m.status = "Composing Oracle prompt with Codex..."
				m.input.Placeholder = "Composing..."
				return m, tea.Batch(m.spinner.Tick, elapsedTick(), composeOraclePrompt(m.repo, m.reqDir, prompt))
			}
			m.status = "Type a prompt, then press Enter"
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if (m.stage == flowDone || m.stage == flowError) && strings.TrimSpace(m.input.Value()) != "" {
		m.stage = flowIdle
		m.input.Placeholder = "Ask oracle..."
	}
	if m.stage == flowReview {
		return m, nil
	}
	return m, cmd
}

func (m model) View() string {
	w := m.width
	if w <= 0 {
		w = 96
	}
	inner := max(40, w-6)

	header := renderHeader(m, inner)
	inputBox := inputStyle.Width(inner - 2).Render(m.input.View())
	progress := renderProgress(m, inner)
	status := renderStatus(m, inner)
	content := renderContent(m, inner)
	help := helpStyle.Width(inner).Render("Enter compose/send · j/k scroll · f/b page · n/p answers · ^Y copy · ^O open · ^R refresh · Esc clear/cancel · ^Q quit")

	body := lipgloss.JoinVertical(lipgloss.Left, header, inputBox, progress, status, content, help)
	return screenStyle.Width(w).Height(max(m.height, lipgloss.Height(body))).Render(lipgloss.PlaceHorizontal(w, lipgloss.Left, body))
}

func renderHeader(m model, width int) string {
	repoName := filepath.Base(m.repo)
	left := titleStyle.Render("oracle")
	right := lipgloss.NewStyle().Foreground(muted).Render(repoName + "  ·  " + shortPath(m.repo, m.outDir))
	line := lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat("─", max(1, width-lipgloss.Width(left)-lipgloss.Width(right)-4)))
	return lipgloss.JoinHorizontal(lipgloss.Center, "  ", left, " ", line, " ", right)
}

func renderStatus(m model, width int) string {
	style := lipgloss.NewStyle().Foreground(muted).Padding(0, 2).Width(width)
	status := truncate(m.status, max(20, width-4))
	if m.stage == flowError || strings.Contains(strings.ToLower(m.status), "error") {
		return style.Foreground(errorTint).Render("✗ " + status)
	}
	if m.stage == flowDone {
		return style.Foreground(accent).Render("✓ " + status)
	}
	if m.running {
		el := time.Since(m.startedAt).Round(time.Second).String()
		return style.Foreground(warning).Render(m.spinner.View() + " " + truncate(m.status, max(18, width-12)) + "  " + el)
	}
	return style.Render(status)
}

func renderProgress(m model, width int) string {
	type st struct{ label string; done, active bool }
	steps := []st{
		{"compose", m.stage == flowReview || m.stage == flowSending || m.stage == flowDone, m.stage == flowComposing},
		{"review", m.stage == flowSending || m.stage == flowDone, m.stage == flowReview},
		{"send", m.stage == flowDone, m.stage == flowSending},
		{"save", m.stage == flowDone, false},
	}
	parts := make([]string, 0, len(steps))
	for i, s := range steps {
		var seg string
		switch {
		case m.stage == flowError && s.active:
			seg = lipgloss.NewStyle().Foreground(bg).Background(errorTint).Bold(true).Padding(0, 1).Render("✗ " + s.label)
		case s.active:
			seg = lipgloss.NewStyle().Foreground(bg).Background(accent2).Bold(true).Padding(0, 1).Render(m.spinner.View() + " " + s.label)
		case s.done:
			seg = lipgloss.NewStyle().Foreground(accent).Bold(true).Render("✓ " + s.label)
		default:
			seg = lipgloss.NewStyle().Foreground(dim).Render("○ " + s.label)
		}
		parts = append(parts, seg)
		if i < len(steps)-1 {
			arrow := lipgloss.NewStyle().Foreground(dim).Render(" → ")
			if s.done {
				arrow = lipgloss.NewStyle().Foreground(accent).Render(" → ")
			}
			parts = append(parts, arrow)
		}
	}
	return lipgloss.NewStyle().Padding(0, 2).Width(width).Render(strings.Join(parts, ""))
}

func renderContent(m model, width int) string {
	height := max(6, m.height-10)
	if m.height <= 0 {
		height = 12
	}

	if width >= 104 {
		listWidth := max(36, width/3)
		previewWidth := max(42, width-listWidth-4)
		list := renderList(m, listWidth, height)
		preview := renderPreview(m, previewWidth, height)
		return lipgloss.JoinHorizontal(lipgloss.Top, list, "  ", preview)
	}

	listHeight := max(5, height/2)
	previewHeight := max(5, height-listHeight-1)
	return lipgloss.JoinVertical(lipgloss.Left, renderList(m, width, listHeight), renderPreview(m, width, previewHeight))
}

func renderList(m model, width, height int) string {
	var rows []string
	if len(m.answers) == 0 {
		empty := lipgloss.NewStyle().Foreground(dim).Padding(2, 0).Render("No oracle answers yet.")
		return listStyle.Width(width - 2).Height(height).Render(empty)
	}

	maxRows := height - 2
	start := 0
	if m.selected >= maxRows {
		start = m.selected - maxRows + 1
	}
	end := min(len(m.answers), start+maxRows)

	for i := start; i < end; i++ {
		item := m.answers[i]
		cursor := "  "
		rowStyle := lipgloss.NewStyle().Foreground(text)
		if i == m.selected {
			cursor = "› "
			rowStyle = rowStyle.Foreground(bg).Background(accent).Bold(true)
		}
		rowWidth := max(32, width-6)
		nameWidth := min(34, max(16, rowWidth/3))
		promptWidth := max(12, rowWidth-nameWidth-11)
		date := item.ModTime.Format("15:04")
		prompt := item.Prompt
		if prompt == "" {
			prompt = item.Name
		}
		line := fmt.Sprintf(
			"%s%-5s  %-*s  %s",
			cursor,
			date,
			promptWidth,
			truncate(prompt, promptWidth),
			truncate(item.Name, nameWidth),
		)
		rows = append(rows, rowStyle.Width(width-6).Render(line))
	}

	return listStyle.Width(width - 2).Height(height).Render(strings.Join(rows, "\n"))
}

func renderPreview(m model, width, height int) string {
	title := lipgloss.NewStyle().Foreground(accent2).Bold(true).Render("preview")
	if selected, ok := m.selectedAnswer(); ok {
		title = lipgloss.NewStyle().Foreground(accent2).Bold(true).Render(truncate(selected.Name, max(12, width-6)))
	}

	bodyStyle := lipgloss.NewStyle().Foreground(text)
	lines := []string{lipgloss.NewStyle().Foreground(dim).Render("Select an oracle answer to preview it here.")}
	if m.stage == flowReview && m.pendingPrompt != "" {
		title = lipgloss.NewStyle().Foreground(accent2).Bold(true).Render("composed prompt")
		lines = readPreviewText(m.pendingPrompt)
		if len(m.pendingFiles) > 0 {
			fileLines := []string{lipgloss.NewStyle().Foreground(accent).Render("Attached files:")}
			for _, path := range m.pendingFiles {
				fileLines = append(fileLines, lipgloss.NewStyle().Foreground(muted).Render("- "+path))
			}
			fileLines = append(fileLines, "")
			lines = append(fileLines, lines...)
		}
	}
	if selected, ok := m.selectedAnswer(); ok {
		if m.stage != flowReview || m.pendingPrompt == "" {
			lines = readPreviewLines(selected.Path)
			if len(lines) == 0 {
				lines = []string{lipgloss.NewStyle().Foreground(dim).Render("Empty answer.")}
			}
		}
	}

	visible := max(1, height-3)
	top := min(max(0, m.previewTop), max(0, len(lines)-visible))
	end := min(len(lines), top+visible)
	rendered := make([]string, 0, visible)
	for _, line := range lines[top:end] {
		rendered = append(rendered, bodyStyle.Render(truncate(line, max(8, width-6))))
	}
	for len(rendered) < visible {
		rendered = append(rendered, "")
	}

	scroll := ""
	if len(lines) > visible {
		scroll = lipgloss.NewStyle().Foreground(muted).Render(fmt.Sprintf(" %d-%d/%d", top+1, end, len(lines)))
	}
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, scroll)
	return listStyle.Width(width - 2).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, header, strings.Join(rendered, "\n")))
}

func (m model) selectedAnswer() (answer, bool) {
	if m.selected < 0 || m.selected >= len(m.answers) {
		return answer{}, false
	}
	return m.answers[m.selected], true
}

func loadAnswers(outDir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(outDir)
		if err != nil {
			return answersLoadedMsg{}
		}
		items := make([]answer, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "latest.md" {
				continue
			}
			path := filepath.Join(outDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}
			items = append(items, answer{
				Path:    path,
				Name:    entry.Name(),
				Prompt:  readPrompt(path),
				ModTime: info.ModTime(),
			})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].ModTime.After(items[j].ModTime)
		})
		return answersLoadedMsg(items)
	}
}

func composeOraclePrompt(repo, reqDir, userPrompt string) tea.Cmd {
	return func() tea.Msg {
		outputFile := filepath.Join(reqDir, timestamp()+"-"+slugify(userPrompt)+".md")
		instructions := buildComposerPrompt(userPrompt)

		args := []string{
			"--sandbox", "read-only",
			"--ask-for-approval", "never",
			"exec",
			"--ephemeral",
			"--skip-git-repo-check",
			"-C", repo,
			"-o", outputFile,
			"-",
		}
		cmd := exec.Command("codex", args...)
		cmd.Dir = repo
		cmd.Stdin = strings.NewReader(instructions)
		cmd.Env = codexEnv()
		out, err := cmd.CombinedOutput()
		data, readErr := os.ReadFile(outputFile)
		if readErr != nil && err == nil {
			err = readErr
		}
		prompt := strings.TrimSpace(string(data))
		if prompt == "" && len(out) > 0 {
			prompt = strings.TrimSpace(cleanCodexOutput(string(out)))
			_ = os.WriteFile(outputFile, []byte(prompt+"\n"), 0o644)
		}
		oraclePrompt, files := parseComposedPrompt(repo, prompt)
		if oraclePrompt == "" {
			oraclePrompt = prompt
		}
		if oraclePrompt != prompt {
			_ = os.WriteFile(outputFile, []byte(prompt+"\n"), 0o644)
		}
		if err != nil {
			detail := strings.TrimSpace(cleanCodexOutput(string(out)))
			if detail != "" {
				err = fmt.Errorf("%w: %s", err, truncate(detail, 240))
			}
		}
		return composeDoneMsg{File: outputFile, UserPrompt: userPrompt, Prompt: oraclePrompt, Files: files, Err: err}
	}
}

func runOracle(repo, outDir, lastFile, userPrompt, oraclePrompt string, files []string) tea.Cmd {
	return func() tea.Msg {
		outputFile := filepath.Join(outDir, timestamp()+"-"+slugify(userPrompt)+".md")
		var file bytes.Buffer
		fmt.Fprintf(&file, "# Oracle Answer\n\n")
		fmt.Fprintf(&file, "- Time: `%s`\n", time.Now().Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&file, "- Repo: `%s`\n", repo)
		fmt.Fprintf(&file, "- Prompt: `%s`\n", userPrompt)
		if len(files) > 0 {
			fmt.Fprintf(&file, "- Files: `%s`\n", strings.Join(files, "`, `"))
		}
		fmt.Fprintf(&file, "\n---\n\n")

		args := []string{"--engine", "browser", "--browser-hide-window", "-p", oraclePrompt}
		for _, path := range files {
			args = append(args, "--file", path)
		}
		cmd := exec.Command("oracle", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		cleaned := cleanOracleOutput(string(out))
		if cleaned != "" {
			fmt.Fprintf(&file, "## Answer\n\n%s\n", cleaned)
		}
		if cleaned == "" {
			// Empty answer is a failure even if the process exited 0.
			fmt.Fprintf(&file, "## Answer\n\n_(oracle returned no answer)_\n")
			if err == nil {
				err = fmt.Errorf("empty answer")
			}
			fmt.Fprintf(&file, "\n```\n%s\n```\n", strings.TrimSpace(string(out)))
		}

		if writeErr := os.WriteFile(outputFile, file.Bytes(), 0o644); writeErr != nil {
			return oracleDoneMsg{File: outputFile, Err: writeErr}
		}
		_ = os.WriteFile(lastFile, file.Bytes(), 0o644)
		return oracleDoneMsg{File: outputFile, Err: err}
	}
}

func parseComposedPrompt(repo, composed string) (string, []string) {
	lines := strings.Split(strings.ReplaceAll(composed, "\r\n", "\n"), "\n")
	section := ""
	foundContract := false
	promptLines := make([]string, 0, len(lines))
	fileLines := make([]string, 0)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch strings.ToLower(trimmed) {
		case "prompt:":
			section = "prompt"
			foundContract = true
			continue
		case "files:":
			section = "files"
			foundContract = true
			continue
		}

		switch section {
		case "prompt":
			promptLines = append(promptLines, line)
		case "files":
			fileLines = append(fileLines, line)
		}
	}

	if !foundContract {
		return strings.TrimSpace(composed), nil
	}
	return strings.TrimSpace(strings.Join(promptLines, "\n")), normalizeOracleFiles(repo, fileLines)
}

func normalizeOracleFiles(repo string, lines []string) []string {
	seen := make(map[string]bool)
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		path := strings.TrimSpace(line)
		path = strings.TrimPrefix(path, "-")
		path = strings.TrimSpace(path)
		path = strings.Trim(path, "`\"'")
		for _, sep := range []string{" — ", " – ", " -- ", " # "} {
			if idx := strings.Index(path, sep); idx >= 0 {
				path = strings.TrimSpace(path[:idx])
			}
		}
		if strings.HasSuffix(path, ")") {
			if idx := strings.LastIndex(path, " ("); idx >= 0 {
				path = strings.TrimSpace(path[:idx])
			}
		}
		if path == "" || strings.EqualFold(path, "none") || strings.EqualFold(path, "n/a") {
			continue
		}
		if !safeOracleFilePath(repo, path) {
			continue
		}
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}
	return files
}

func safeOracleFilePath(repo, path string) bool {
	if strings.ContainsAny(path, "\x00\r\n") {
		return false
	}
	checkPath := strings.TrimPrefix(path, "!")
	if checkPath == "" || filepath.IsAbs(checkPath) {
		return false
	}
	clean := filepath.Clean(checkPath)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false
	}
	segments := strings.Split(filepath.ToSlash(clean), "/")
	for _, segment := range segments {
		switch segment {
		case ".git", ".agents", "node_modules", "dist", "build", "coverage":
			return false
		}
	}
	base := filepath.Base(clean)
	if base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") {
		return false
	}
	return repo != ""
}

func buildComposerPrompt(userPrompt string) string {
	return strings.TrimSpace(fmt.Sprintf(`Use the oracle skill.

You are preparing a single high-signal prompt for the Oracle CLI.
Read only the repo context needed to make the prompt useful.
Do not modify files.
Do not include secrets or raw .env values.

Return only the final prompt text to send to Oracle. No markdown fences, no commentary.
Use this exact output contract:

PROMPT:
<the complete prompt to send to Oracle>

FILES:
- <relative path or glob to attach>

Use an empty FILES block only for purely conversational requests that need no repo context.
In FILES, each bullet must contain only the path/glob, no explanation text.

The Oracle prompt must be standalone and include:
- concise project/repo briefing;
- relevant directories, files, commands, and constraints you discovered;
- the user's exact question or goal;
- what answer format Oracle should return;
- any attached file paths and why they matter.

File rules:
- Use repo-relative paths or globs only.
- Do not include .env files, private keys, credentials, node_modules, dist, build, coverage, .git, or generated .agents output.
- Prefer a small, truthful file set over whole-repo attachment.

User request:
%s`, userPrompt))
}

func codexEnv() []string {
	env := os.Environ()
	home, err := os.UserHomeDir()
	if err == nil {
		env = append(env, "CODEX_HOME="+filepath.Join(home, ".codex-new"))
	}
	env = append(env, "TERM=xterm-256color")
	return env
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

func copyText(text string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return statusMsg("Copy failed: " + err.Error())
		}
		return statusMsg("Copied composed prompt")
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

func readPrompt(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- Prompt: `") && strings.HasSuffix(line, "`") {
			return strings.TrimSuffix(strings.TrimPrefix(line, "- Prompt: `"), "`")
		}
	}
	return ""
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

func readPreviewText(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, strings.TrimRight(line, "\r"))
	}
	return lines
}

func cleanCodexOutput(output string) string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(stripANSI(line))
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "WARNING:") || strings.HasPrefix(trimmed, "Unable to open session log file") {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, "\n")
}

func cleanOracleOutput(output string) string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	inAnswer := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(stripANSI(line))
		if trimmed == "" {
			if inAnswer && len(cleaned) > 0 && cleaned[len(cleaned)-1] != "" {
				cleaned = append(cleaned, "")
			}
			continue
		}
		if strings.EqualFold(trimmed, "Answer:") {
			inAnswer = true
			continue
		}
		if isOracleNoise(trimmed) {
			continue
		}
		if inAnswer {
			cleaned = append(cleaned, trimmed)
			continue
		}
		cleaned = append(cleaned, trimmed)
	}

	for len(cleaned) > 0 && cleaned[len(cleaned)-1] == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}
	return strings.Join(cleaned, "\n")
}

func isOracleNoise(line string) bool {
	switch {
	case strings.HasPrefix(line, "🧿 oracle "):
		return true
	case strings.HasPrefix(line, "Launching browser mode"):
		return true
	case strings.HasPrefix(line, "This run can take"):
		return true
	case strings.Contains(line, " · gpt-") && strings.Contains(line, " ↑") && strings.Contains(line, " ↓"):
		return true
	default:
		return false
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			i++
			if i < len(s) && s[i] == '[' {
				for i < len(s) && (s[i] < '@' || s[i] > '~') {
					i++
				}
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
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
		return "oracle"
	}
	if len(slug) > 48 {
		return strings.Trim(slug[:48], "-")
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
