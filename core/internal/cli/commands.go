package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/spf13/cobra"
)

// newCommandsCmd renders docs/COMMANDS.md — the command registry that covers
// the whole platform surface, shell functions included, which is why the
// source stays a document rather than the cobra tree.
func newCommandsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commands [имя]",
		Short: "Реестр команд платформы (docs/COMMANDS.md)",
		Long: "Без аргумента — индекс всех команд по категориям; с именем — карточка\n" +
			"команды. В оболочке то же самое доступно как `?` и `? <cmd>`.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			doc := filepath.Join(home, "dotfiles", "docs", "COMMANDS.md")
			data, err := os.ReadFile(doc)
			if err != nil {
				return fmt.Errorf("реестр команд не найден: %s", doc)
			}
			lines := strings.Split(string(data), "\n")

			r := ui.New(cmd.OutOrStdout())
			if len(args) == 0 {
				renderCommandIndex(r, lines)
				return nil
			}
			return renderCommandCard(r, lines, args[0])
		},
	}
}

func renderCommandIndex(r *ui.Renderer, lines []string) {
	r.Blank()
	r.Line(" " + r.Bold("Personal Platform") + "    " + r.Muted("? <cmd> for details"))
	r.Blank()

	name := ""
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "### "):
			r.Line(" " + r.Accent(strings.TrimPrefix(line, "### ")))
		case strings.HasPrefix(line, "## "):
			name = strings.TrimPrefix(line, "## ")
		case strings.HasPrefix(line, "**Что:**"):
			what := strings.TrimSpace(strings.TrimPrefix(line, "**Что:**"))
			what = strings.NewReplacer("**", "", "`", "").Replace(what)
			r.Line(fmt.Sprintf("   %-22s %s", name, r.Muted(what)))
		}
	}
	r.Blank()
}

func renderCommandCard(r *ui.Renderer, lines []string, name string) error {
	var section []string
	inSection := false
	for _, line := range lines {
		if line == "## "+name {
			inSection = true
		} else if inSection && strings.HasPrefix(line, "## ") {
			break
		}
		if inSection {
			section = append(section, line)
		}
	}
	if len(section) == 0 {
		return fmt.Errorf("команда %q не найдена в реестре", name)
	}

	r.Blank()
	for _, line := range section {
		switch {
		case line == "---":
		case strings.HasPrefix(line, "## "):
			title := strings.TrimPrefix(line, "## ")
			r.Line(" " + r.Bold(title))
			r.Line(" " + r.Muted(strings.Repeat("─", len([]rune(title)))))
		case labelValue(line):
			label, value, _ := strings.Cut(strings.TrimPrefix(line, "**"), ":**")
			value = strings.TrimSpace(value)
			if value == "" {
				r.Blank()
				r.Line("   " + r.Bold(label+":"))
			} else {
				r.Line("   " + r.Bold(label+":") + " " + highlightTicks(r, value))
			}
		case strings.HasPrefix(line, "- "):
			r.Line("       " + r.Muted("•") + " " + highlightTicks(r, strings.TrimPrefix(line, "- ")))
		case line == "":
			r.Blank()
		default:
			r.Line("   " + highlightTicks(r, line))
		}
	}
	r.Blank()
	return nil
}

// labelValue matches the `**Label:** value` card lines.
func labelValue(line string) bool {
	if !strings.HasPrefix(line, "**") {
		return false
	}
	rest := strings.TrimPrefix(line, "**")
	i := strings.Index(rest, ":**")
	return i > 0 && !strings.Contains(rest[:i], "*")
}

// highlightTicks renders `code` spans in the accent colour.
func highlightTicks(r *ui.Renderer, s string) string {
	parts := strings.Split(s, "`")
	var b strings.Builder
	for i, part := range parts {
		if i%2 == 1 {
			b.WriteString(r.Accent(part))
		} else {
			b.WriteString(part)
		}
	}
	return b.String()
}
