package main

// wiki-tui — WikiPedik memory operations panel for the wikipedik tmux session.
//
// Prompt actions compose a curator prompt, copy it to the clipboard, type it
// into the dev chat pane input and jump focus there — the user reviews and
// presses Enter. Mechanical actions run directly and show their output.

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	home         = os.Getenv("HOME")
	vaultRoot    = filepath.Join(home, "Desktop", "WikiPedik")
	projectsRoot = filepath.Join(vaultRoot, "dev", "20-projects")
	workRoot     = filepath.Join(home, "Desktop", "Prokectfiles")
	scopeFile    = filepath.Join(home, ".cache", "wiki-menu-scope")
	devPane      = envOr("WIKI_DEV_PANE", "wikipedik:0.0")
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ─── styles ───

var (
	cAccent = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#A78BFA"}
	cDim    = lipgloss.AdaptiveColor{Light: "#9A9A9A", Dark: "#6B6B6B"}
	cOk     = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#7EE787"}
	cWarn   = lipgloss.AdaptiveColor{Light: "#B26A00", Dark: "#F0B72F"}

	stTitle   = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	stScope   = lipgloss.NewStyle().Bold(true).Foreground(cOk)
	stSection = lipgloss.NewStyle().Foreground(cDim).Bold(true).MarginTop(1)
	stKey     = lipgloss.NewStyle().Bold(true).Foreground(cAccent).Width(3)
	stName    = lipgloss.NewStyle().Bold(true).Width(13)
	stDesc    = lipgloss.NewStyle().Foreground(cDim)
	stCursor  = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
	stFlashOk = lipgloss.NewStyle().Foreground(cOk).Bold(true)
	stFlashWn = lipgloss.NewStyle().Foreground(cWarn).Bold(true)
	stHelp    = lipgloss.NewStyle().Foreground(cDim).MarginTop(1)
	stBox     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cAccent).
			Padding(1, 2)
	stStat = lipgloss.NewStyle().Foreground(cWarn)
)

// ─── actions ───

type action struct {
	key     string
	name    string
	desc    string
	section string
	prompt  func(scope string) string // nil => mechanical
	run     func(scope string) string // mechanical: returns output
}

func promptStatus(scope string) string {
	return fmt.Sprintf("Use skill wiki-status. Scope: %s. Отчитайся: счётчики инбоксов и salvage-зон, curated-страницы, synthesis-кандидаты, устаревшие health-файлы, последние записи лога, рекомендуемые действия. Файлы не менять.", scope)
}

func promptSync(scope string) string {
	return fmt.Sprintf("Use skill inbox-drain. Scope: %s. Разбери salvage-зоны (Phase 0) и candidate-записи инбоксов по всем фазам: дедуп, derivability-тест (spec-derivable отклоняй), по каждой записи предлагай target и жди подтверждения, evolution-связи, обнови индексы, product log и обязательно hot.md (Phase 7, fallback: python3 ~/dotfiles/scripts/wiki-hot-refresh.py %s).", scope, scope)
}

func promptSynth(scope string) string {
	return fmt.Sprintf("Use skill wiki-synthesize. Scope: %s. Обработай _synthesis-candidates.md и повторы lessons/gotchas между репо: предлагай product-паттерны до записи в shared/. Rule Promotion по планке скилла: повторилось 2+ раз И must-follow И ложится на конкретные glob-пути — предложи правило (paths: обязателен), жди моего подтверждения.", scope)
}

func promptLint(scope string) string {
	return fmt.Sprintf("Use skill wiki-lint. Scope: %s. Найди противоречия, устаревшие claims (Last verified), orphan-страницы, кандидатов на promotion и утечки приватного. Findings в health.md scope, событие в log.md. Изменения curated-страниц — только с моего подтверждения.", scope)
}

func promptIngest(string) string {
	since := time.Now().AddDate(0, 0, -14).Format("2006-01-02")
	return fmt.Sprintf("Use skill agent-history-ingest. --source all. Сначала прогони python3 ~/dotfiles/scripts/agent-session-digest.py --source all --since %s, затем обработай новые дайджесты из 10-wiki/sources/sessions/_digests/ (манифест .ingest-manifest.json, обработанные пропусти). Кластеризуй связанные сессии, candidate captures строго по durable-сигналам с derivability-тестом.", since)
}

func runRulesSync(scope string) string {
	out := capture("python3", filepath.Join(home, "dotfiles/scripts/wiki-rules-sync.py"), scope)
	out += "\n" + capture("python3", filepath.Join(home, "dotfiles/scripts/wiki-hot-refresh.py"), scope)
	return out
}

func runGitStatus(string) string {
	out := capture("git", "-C", vaultRoot, "status", "--short")
	if strings.TrimSpace(out) == "" {
		out = "✓ вольт чистый"
	}
	unpushed := strings.TrimSpace(capture("git", "-C", vaultRoot, "rev-list", "--count", "origin/main..HEAD"))
	return out + "\n\nunpushed commits: " + unpushed
}

func runPush(string) string {
	return capture("git", "-C", vaultRoot, "push")
}

var actions = []action{
	{key: "1", name: "status", desc: "что накопилось, что делать", section: "промпт → dev-чат", prompt: promptStatus},
	{key: "2", name: "sync", desc: "разобрать inbox + сейф", section: "промпт → dev-чат", prompt: promptSync},
	{key: "3", name: "synthesize", desc: "повторы → паттерны/правила", section: "промпт → dev-чат", prompt: promptSynth},
	{key: "4", name: "lint", desc: "протухание и противоречия", section: "промпт → dev-чат", prompt: promptLint},
	{key: "5", name: "ingest", desc: "майнинг сессий за 2 недели", section: "промпт → dev-чат", prompt: promptIngest},
	{key: "6", name: "rules-sync", desc: "правила → репо + hot.md", section: "механика", run: runRulesSync},
	{key: "7", name: "git status", desc: "вольт: незакоммиченное", section: "механика", run: runGitStatus},
	{key: "8", name: "push", desc: "вольт → GitHub", section: "механика", run: runPush},
	{key: "b", name: "bootstrap", desc: "подключить репо без памяти", section: "механика"},
}

// ─── helpers ───

func capture(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	s := strings.TrimRight(string(out), "\n")
	if err != nil && s == "" {
		s = "error: " + err.Error()
	}
	return s
}

func listDirs(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "_templates" {
			out = append(out, e.Name())
		}
	}
	return out
}

func scopePath(scope string) string {
	return filepath.Join(projectsRoot, filepath.FromSlash(scope))
}

func countCandidates(scope string) int {
	count := 0
	_ = filepath.Walk(scopePath(scope), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "_inbox.md" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if strings.TrimSpace(sc.Text()) == "Status: candidate" {
				count++
			}
		}
		return nil
	})
	return count
}

func missingMemoryRepos() []string {
	var out []string
	for _, co := range listDirs(workRoot) {
		for _, prod := range listDirs(filepath.Join(workRoot, co)) {
			for _, repo := range listDirs(filepath.Join(workRoot, co, prod)) {
				dir := filepath.Join(workRoot, co, prod, repo)
				if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
					continue
				}
				if _, err := os.Lstat(filepath.Join(dir, "docs", "knowledge")); err == nil {
					continue
				}
				out = append(out, co+"/"+prod+"/"+repo)
			}
		}
	}
	return out
}

func deliverPrompt(prompt string) string {
	pb := exec.Command("pbcopy")
	pb.Stdin = strings.NewReader(prompt)
	_ = pb.Run()

	session := strings.SplitN(devPane, ":", 2)[0]
	if exec.Command("tmux", "has-session", "-t", session).Run() == nil {
		_ = exec.Command("tmux", "send-keys", "-t", devPane, "-l", "--", prompt).Run()
		_ = exec.Command("tmux", "select-pane", "-t", devPane).Run()
		return "промпт в инпуте dev-чата (и в буфере) — проверь и жми Enter"
	}
	return "tmux-сессии нет — промпт в буфере обмена"
}

// ─── scope picker data ───

type pickLevel struct {
	title   string
	items   []string // first item may be the "whole scope" shortcut
	scopes  []string // scope value per item ("" = descend)
	descend []string // base scope to descend into ("" = leaf)
}

func companyLevel() pickLevel {
	var l pickLevel
	l.title = "Компания"
	for _, co := range listDirs(projectsRoot) {
		l.items = append(l.items, co)
		// company with direct repos/ (e.g. _platform) is selectable as-is
		if _, err := os.Stat(filepath.Join(projectsRoot, co, "repos")); err == nil {
			l.scopes = append(l.scopes, co)
			l.descend = append(l.descend, co)
		} else {
			l.scopes = append(l.scopes, "")
			l.descend = append(l.descend, co)
		}
	}
	return l
}

func childLevel(base string) pickLevel {
	var l pickLevel
	dir := scopePath(base)
	if _, err := os.Stat(filepath.Join(dir, "repos")); err == nil {
		l.title = "Репо (" + base + ")"
		l.items = append(l.items, "· весь scope: "+base)
		l.scopes = append(l.scopes, base)
		l.descend = append(l.descend, "")
		for _, repo := range listDirs(filepath.Join(dir, "repos")) {
			l.items = append(l.items, repo)
			l.scopes = append(l.scopes, base+"/"+repo)
			l.descend = append(l.descend, "")
		}
		return l
	}
	l.title = "Продукт (" + base + ")"
	l.items = append(l.items, "· вся компания: "+base)
	l.scopes = append(l.scopes, base)
	l.descend = append(l.descend, "")
	for _, prod := range listDirs(dir) {
		if _, err := os.Stat(filepath.Join(dir, prod, "repos")); err != nil {
			continue
		}
		l.items = append(l.items, prod)
		l.scopes = append(l.scopes, "")
		l.descend = append(l.descend, base+"/"+prod)
	}
	return l
}

// ─── model ───

type state int

const (
	stateMenu state = iota
	statePick
	stateOutput
	stateBootstrap
)

type model struct {
	st         state
	scope      string
	candidates int
	cursor     int
	flash      string
	flashWarn  bool

	pick     pickLevel
	pickBase string

	outTitle string
	output   string

	bootstrapItems []string
}

func initialModel() model {
	m := model{st: stateMenu}
	if b, err := os.ReadFile(scopeFile); err == nil {
		m.scope = strings.TrimSpace(string(b))
	}
	if m.scope == "" {
		m.st = statePick
		m.pick = companyLevel()
	} else {
		m.candidates = countCandidates(m.scope)
	}
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m *model) setScope(scope string) {
	m.scope = scope
	m.candidates = countCandidates(scope)
	_ = os.MkdirAll(filepath.Dir(scopeFile), 0o755)
	_ = os.WriteFile(scopeFile, []byte(scope+"\n"), 0o644)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	k := key.String()

	switch m.st {

	case stateMenu:
		switch k {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(actions)-1 {
				m.cursor++
			}
		case "s":
			m.st = statePick
			m.pick = companyLevel()
			m.pickBase = ""
			m.cursor = 0
		case "enter":
			return m.activate(actions[m.cursor])
		default:
			for _, a := range actions {
				if a.key == k {
					return m.activate(a)
				}
			}
		}

	case statePick:
		switch k {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.scope != "" {
				m.st = stateMenu
				m.cursor = 0
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.pick.items)-1 {
				m.cursor++
			}
		case "enter":
			i := m.cursor
			if i >= len(m.pick.items) {
				break
			}
			if m.pick.scopes[i] != "" && m.pick.descend[i] == "" {
				m.setScope(m.pick.scopes[i])
				m.st = stateMenu
				m.cursor = 0
			} else if m.pick.descend[i] != "" {
				m.pick = childLevel(m.pick.descend[i])
				m.cursor = 0
			}
		}

	case stateOutput:
		m.st = stateMenu
		m.cursor = 0

	case stateBootstrap:
		switch k {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.st = stateMenu
			m.cursor = 0
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.bootstrapItems)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.bootstrapItems) == 0 {
				m.st = stateMenu
				break
			}
			parts := strings.SplitN(m.bootstrapItems[m.cursor], "/", 3)
			m.outTitle = "bootstrap " + m.bootstrapItems[m.cursor]
			m.output = capture("zsh",
				filepath.Join(home, "dotfiles/skills-stash/wiki/scripts/wiki-bootstrap-product.sh"),
				parts[0], parts[1], parts[2])
			m.st = stateOutput
			m.cursor = 0
		}
	}
	return m, nil
}

func (m model) activate(a action) (tea.Model, tea.Cmd) {
	switch {
	case a.prompt != nil:
		m.flash = deliverPrompt(a.prompt(m.scope))
		m.flashWarn = strings.Contains(m.flash, "нет")
	case a.name == "bootstrap":
		m.bootstrapItems = missingMemoryRepos()
		m.st = stateBootstrap
		m.cursor = 0
	case a.run != nil:
		m.outTitle = a.name
		m.output = a.run(m.scope)
		m.st = stateOutput
		if a.name == "rules-sync" || a.name == "push" {
			m.candidates = countCandidates(m.scope)
		}
	}
	return m, nil
}

// ─── view ───

func (m model) View() string {
	switch m.st {
	case statePick:
		return m.viewPick()
	case stateOutput:
		return m.viewOutput()
	case stateBootstrap:
		return m.viewBootstrap()
	}
	return m.viewMenu()
}

func (m model) header() string {
	stats := ""
	if m.candidates > 0 {
		stats = stStat.Render(fmt.Sprintf("  inbox: %d candidate", m.candidates))
	}
	return stTitle.Render("WikiPedik · память") + "\n" +
		stDesc.Render("scope: ") + stScope.Render(m.scope) + stats
}

func (m model) viewMenu() string {
	var b strings.Builder
	b.WriteString(m.header() + "\n")

	section := ""
	for i, a := range actions {
		if a.section != section {
			section = a.section
			b.WriteString(stSection.Render("── "+section+" ──") + "\n")
		}
		cursor := "  "
		if i == m.cursor {
			cursor = stCursor.Render("▸ ")
		}
		b.WriteString(fmt.Sprintf("%s%s%s%s\n",
			cursor, stKey.Render(a.key), stName.Render(a.name), stDesc.Render(a.desc)))
	}

	if m.flash != "" {
		style := stFlashOk
		if m.flashWarn {
			style = stFlashWn
		}
		b.WriteString("\n" + style.Render("→ "+m.flash) + "\n")
	}
	b.WriteString(stHelp.Render("↑↓/цифры · enter · s scope · q выход"))
	return stBox.Render(b.String())
}

func (m model) viewPick() string {
	var b strings.Builder
	b.WriteString(stTitle.Render(m.pick.title) + "\n\n")
	for i, item := range m.pick.items {
		cursor := "  "
		if i == m.cursor {
			cursor = stCursor.Render("▸ ")
		}
		b.WriteString(cursor + item + "\n")
	}
	b.WriteString(stHelp.Render("↑↓ · enter выбрать · esc назад · q выход"))
	return stBox.Render(b.String())
}

func (m model) viewOutput() string {
	var b strings.Builder
	b.WriteString(stTitle.Render(m.outTitle) + "\n\n")
	lines := strings.Split(m.output, "\n")
	if len(lines) > 30 {
		lines = append(lines[:30], stDesc.Render(fmt.Sprintf("… ещё %d строк", len(lines)-30)))
	}
	b.WriteString(strings.Join(lines, "\n") + "\n")
	b.WriteString(stHelp.Render("любая клавиша — меню"))
	return stBox.Render(b.String())
}

func (m model) viewBootstrap() string {
	var b strings.Builder
	b.WriteString(stTitle.Render("Репо без памяти") + "\n\n")
	if len(m.bootstrapItems) == 0 {
		b.WriteString(stFlashOk.Render("✓ все репо уже подключены") + "\n")
	}
	for i, item := range m.bootstrapItems {
		cursor := "  "
		if i == m.cursor {
			cursor = stCursor.Render("▸ ")
		}
		b.WriteString(cursor + item + "\n")
	}
	b.WriteString(stHelp.Render("enter подключить · esc назад"))
	return stBox.Render(b.String())
}

func main() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
