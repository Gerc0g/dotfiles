// Package ui renders hq output.
//
// Two rules shape everything here. Styling is decided by the destination, not
// by a flag: a terminal gets colour and box drawing, a pipe gets plain text,
// because the same commands are read by a person and parsed by an agent.
// And colours are adaptive, so the output stays legible on a light terminal
// as well as a dark one.
package ui

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Fallback width when the destination is not a terminal. Wide enough for a
// full path, narrow enough to stay readable in a pasted log.
const fallbackWidth = 100

// Palette entries are adaptive: the first value is used on light terminals,
// the second on dark ones.
var (
	colorOK      = lipgloss.AdaptiveColor{Light: "#0F7B3F", Dark: "#4ADE80"}
	colorMissing = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colorDrifted = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	colorMuted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	colorAccent  = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}
)

// Renderer draws to one destination with styling suited to it.
type Renderer struct {
	out   io.Writer
	width int
	rich  bool

	ok      lipgloss.Style
	missing lipgloss.Style
	drifted lipgloss.Style
	muted   lipgloss.Style
	accent  lipgloss.Style
	bold    lipgloss.Style
}

// New builds a Renderer for out, detecting terminal width and colour support.
func New(out io.Writer) *Renderer {
	renderer := &Renderer{out: out, width: fallbackWidth}

	if file, isFile := out.(*os.File); isFile {
		fd := int(file.Fd())
		if term.IsTerminal(fd) {
			renderer.rich = true
			if width, _, err := term.GetSize(fd); err == nil && width > 0 {
				renderer.width = width
			}
		}
	}

	plain := lipgloss.NewStyle()
	if !renderer.rich {
		renderer.ok, renderer.missing, renderer.drifted = plain, plain, plain
		renderer.muted, renderer.accent, renderer.bold = plain, plain, plain
		return renderer
	}

	renderer.ok = lipgloss.NewStyle().Foreground(colorOK)
	renderer.missing = lipgloss.NewStyle().Foreground(colorMissing)
	renderer.drifted = lipgloss.NewStyle().Foreground(colorDrifted)
	renderer.muted = lipgloss.NewStyle().Foreground(colorMuted)
	renderer.accent = lipgloss.NewStyle().Foreground(colorAccent)
	renderer.bold = lipgloss.NewStyle().Bold(true)

	return renderer
}

// Width reports the usable line width.
func (r *Renderer) Width() int { return r.width }

// Rich reports whether the destination supports styling.
func (r *Renderer) Rich() bool { return r.rich }

// Out exposes the destination for callers that write plain machine output.
func (r *Renderer) Out() io.Writer { return r.out }

// OK styles a healthy result.
func (r *Renderer) OK(text string) string { return r.ok.Render(text) }

// Missing styles something that has not been set up yet.
func (r *Renderer) Missing(text string) string { return r.missing.Render(text) }

// Drifted styles something that exists but is wrong.
func (r *Renderer) Drifted(text string) string { return r.drifted.Render(text) }

// Muted styles secondary text such as paths and hints.
func (r *Renderer) Muted(text string) string { return r.muted.Render(text) }

// Accent styles a primary name.
func (r *Renderer) Accent(text string) string { return r.accent.Render(text) }

// Bold styles emphasis.
func (r *Renderer) Bold(text string) string { return r.bold.Render(text) }

// Line writes one already-composed line.
func (r *Renderer) Line(text string) {
	io.WriteString(r.out, text+"\n")
}

// Blank writes an empty line.
func (r *Renderer) Blank() {
	io.WriteString(r.out, "\n")
}

// Truncate shortens text to fit width, keeping the tail visible when the text
// is a path — the end of a path carries more meaning than its prefix.
func Truncate(text string, width int) string {
	if width <= 0 || lipgloss.Width(text) <= width {
		return text
	}
	if width <= 1 {
		return "…"
	}

	runes := []rune(text)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[1:]
	}

	return "…" + string(runes)
}
