// Package charmstack is the canonical manifest of our console/TUI development
// stack: the full Charm ecosystem, pinned in one go.mod so that gopls indexes
// every library at once. It exists so the editor (and coding agents via the
// LSP tool) get exact signatures, hovers, completions and go-to-definition for
// the whole stack — a single "LSP server over the libraries".
//
// This is not meant to run. The blank references below anchor each library so
// LSP operations (hover / goToDefinition / workspaceSymbol) resolve against a
// concrete call site. Build a real TUI in tools/<name>/ and import what you need.
//
// Full reference and decision guide: docs/platform/charm-stack.md
package charmstack

import (
	// Core framework + widgets + styling.
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	ltable "github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/lipgloss/tree"

	// Forms, markdown, SSH, logging, animation, CLI scaffolding.
	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/wish"

	// Low-level helpers we already rely on (ANSI-safe truncation, etc).
	"github.com/charmbracelet/x/ansi"
)

// Anchors for LSP. Each line references one entry point per library so that
// `hover` / `goToDefinition` have a position to resolve, and so go mod tidy
// keeps every dependency required.
var (
	_ = tea.NewProgram   // app: Model/Update/View event loop
	_ = lipgloss.NewStyle // styling: colors, borders, padding, layout
	_ = ltable.New        // lipgloss static table (render-only)
	_ = list.New          // lipgloss static list
	_ = tree.New          // lipgloss static tree

	_ = progress.New  // bubbles: gradient progress bar (ViewAs for static)
	_ = spinner.New   // bubbles: spinner
	_ = table.New     // bubbles: interactive table
	_ = textinput.New // bubbles: text input
	_ = viewport.New  // bubbles: scrollable viewport

	_ = huh.NewForm      // forms / prompts / wizards
	_ = glamour.Render   // markdown -> styled terminal output
	_ = wish.NewServer   // serve Bubble Tea apps over SSH
	_ = log.New          // structured colorful logging
	_ = harmonica.NewSpring // physics-based animation (springs)
	_ = fang.Execute     // styled Cobra CLI scaffolding
	_ = ansi.Truncate    // ANSI-aware string truncation
)
