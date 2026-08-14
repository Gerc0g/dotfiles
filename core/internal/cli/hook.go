package cli

import (
	"fmt"
	"os"

	"github.com/Gerc0g/dotfiles/core/wiki"
	"github.com/spf13/cobra"
)

func newHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hook",
		Short:  "Тела agent-хуков",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE:   func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	var agent string
	sessionStart := &cobra.Command{
		Use:   "session-start",
		Short: "SessionStart: hot.md-контекст текущего репо",
		Long: "Печатает hook-JSON с additionalContext или ничего. Никогда не падает:\n" +
			"хук не имеет права ломать старт сессии агента.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return nil
			}
			if out := wiki.SessionStartContext(agent, cwd); out != "" {
				fmt.Fprintln(cmd.OutOrStdout(), out)
			}
			return nil
		},
	}
	sessionStart.Flags().StringVar(&agent, "agent", "claude", "claude или codex")

	cmd.AddCommand(sessionStart)
	return cmd
}
