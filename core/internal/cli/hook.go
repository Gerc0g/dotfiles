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
			home, _ := os.UserHomeDir()
			wiki.TouchHook(home, agent, "session-start")
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

	sessionEnd := &cobra.Command{
		Use:   "session-end",
		Short: "SessionEnd: отдать инбокс текущего репо куратору",
		Long: "Проверяет гварды и, если надо, отцепляет разбор в фон. Никогда не падает\n" +
			"и ничего не ждёт: хук не имеет права держать завершение сессии.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			wiki.TouchHook(home, agent, "session-end")
			cwd, err := os.Getwd()
			if err != nil {
				return nil
			}
			// Log the firing before anything else. Without this a silent log
			// is ambiguous — a hook that never ran and one that ran and failed
			// look identical, which is how a dead trigger hid for three days.
			wiki.LogAutosync("hook session-end: %s (%s)", cwd, agent)

			guards := wiki.DefaultDrainGuards()
			scope, err := wiki.ScopeForDir(cwd)
			if err == nil {
				started, err := wiki.SpawnDrainStarted(scope, guards, true)
				if err != nil {
					wiki.LogAutosync("hook session-end: %v", err)
				}
				if started {
					return nil
				}
			} else {
				wiki.LogAutosync("hook session-end: скоуп не определён — %v", err)
			}

			// The repo just worked in is fine; lend the moment to whichever
			// repo is most behind. Backlogs collect where sessions do not.
			if err := wiki.SweepDrain(guards, true); err != nil {
				wiki.LogAutosync("hook session-end: sweep — %v", err)
			}
			return nil
		},
	}
	sessionEnd.Flags().StringVar(&agent, "agent", "claude", "claude или codex")

	cmd.AddCommand(sessionStart, sessionEnd)
	return cmd
}
