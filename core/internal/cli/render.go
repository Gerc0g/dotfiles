package cli

import (
	"fmt"
	"io"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/setup"
	"github.com/charmbracelet/lipgloss"
)

// statusGlyph is what a person sees; statusWord is what a pipe sees. Splitting
// them keeps the output pretty in a terminal and parseable everywhere else,
// without a --format flag to remember.
var statusGlyph = map[setup.Status]string{
	setup.StatusOK:      "✓",
	setup.StatusMissing: "○",
	setup.StatusDrifted: "✗",
	setup.StatusSkipped: "–",
}

var statusWord = map[setup.Status]string{
	setup.StatusOK:      "ok",
	setup.StatusMissing: "missing",
	setup.StatusDrifted: "drifted",
	setup.StatusSkipped: "skipped",
}

func renderReports(out io.Writer, reports []setup.Report) {
	renderer := ui.New(out)

	nameWidth := 0
	for _, report := range reports {
		if width := lipgloss.Width(report.Step.Name); width > nameWidth {
			nameWidth = width
		}
	}

	if renderer.Rich() {
		renderer.Blank()
	}

	for _, report := range reports {
		mark := renderMark(renderer, report.Result.Status)

		// 4 leading spaces, the mark, two gaps and the padded name.
		used := 4 + 1 + 2 + nameWidth + 2
		detail := ui.Truncate(report.Result.Detail, renderer.Width()-used)

		if renderer.Rich() {
			renderer.Line(fmt.Sprintf("  %s  %-*s  %s",
				mark, nameWidth, renderer.Accent(report.Step.Name), renderer.Muted(detail)))
			continue
		}

		renderer.Line(fmt.Sprintf("%-7s %-*s  %s",
			statusWord[report.Result.Status], nameWidth, report.Step.Name, report.Result.Detail))
	}
}

func renderMark(renderer *ui.Renderer, status setup.Status) string {
	glyph := statusGlyph[status]

	switch status {
	case setup.StatusOK:
		return renderer.OK(glyph)
	case setup.StatusMissing:
		return renderer.Missing(glyph)
	case setup.StatusDrifted:
		return renderer.Drifted(glyph)
	default:
		return renderer.Muted(glyph)
	}
}

func renderSummary(out io.Writer, reports []setup.Report) {
	renderer := ui.New(out)
	counts := setup.CountByStatus(reports)

	var parts []string
	if n := counts[setup.StatusOK]; n > 0 {
		parts = append(parts, renderer.OK(fmt.Sprintf("%d в порядке", n)))
	}
	if n := counts[setup.StatusMissing]; n > 0 {
		parts = append(parts, renderer.Missing(ui.Plural(n, "не настроен", "не настроены", "не настроены")))
	}
	if n := counts[setup.StatusDrifted]; n > 0 {
		parts = append(parts, renderer.Drifted(ui.Plural(n, "расхождение", "расхождения", "расхождений")))
	}
	if n := counts[setup.StatusSkipped]; n > 0 {
		parts = append(parts, renderer.Muted(ui.Plural(n, "пропущен", "пропущено", "пропущено")))
	}

	renderer.Blank()
	renderer.Line("  " + joinDot(renderer, parts))
	if renderer.Rich() {
		renderer.Blank()
	}
}

func joinDot(renderer *ui.Renderer, parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += renderer.Muted(" · ")
		}
		out += part
	}
	return out
}

// needsAttention reports whether any step is missing or drifted.
func needsAttention(reports []setup.Report) bool {
	for _, report := range reports {
		if report.Result.Status.NeedsApply() {
			return true
		}
	}
	return false
}

// hasApplicableWork reports whether `hq setup` could actually fix anything.
// Check-only steps such as skill-links can fail without setup having a remedy,
// and telling the user to run setup in that case would be a lie.
func hasApplicableWork(reports []setup.Report) bool {
	for _, report := range reports {
		if report.Result.Status.NeedsApply() && report.Step.Applicable() {
			return true
		}
	}
	return false
}

// advice describes what to do about the failing steps, if anything.
func advice(reports []setup.Report) string {
	switch {
	case !needsAttention(reports):
		return ""
	case hasApplicableWork(reports):
		return "Запустите: hq setup"
	default:
		return "Автоматического исправления нет, нужно вмешаться руками"
	}
}

func newEnv() (setup.Env, error) {
	root, err := worldRoot()
	if err != nil {
		return setup.Env{}, err
	}
	return setup.NewEnv(root)
}
