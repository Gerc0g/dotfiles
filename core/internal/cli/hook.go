package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Gerc0g/dotfiles/core/wiki"
	"github.com/spf13/cobra"
)

// sessionIDFromStdin reads the hook payload agents pass on stdin and pulls the
// session id out of it. Anything unreadable is not an error: the caller falls
// back to a directory-derived key, and a hook must never fail a prompt.
func sessionIDFromStdin(in io.Reader) string {
	// Hooks are fed their payload on a pipe. Run by hand from a terminal there
	// is nothing to read, and reading anyway would hang waiting for input.
	if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(in, 1<<20))
	if err != nil || len(data) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	for _, key := range []string{"session_id", "sessionId", "conversationId", "thread_id"} {
		if v, ok := payload[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

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
			out := wiki.SessionStartContext(agent, cwd)
			if out == "" {
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)

			// Record what this session was served, so its first prompt has
			// something to compare against and stays quiet.
			if hot := wiki.RepoHotFile(cwd); hot != "" {
				key := wiki.SessionKey(sessionIDFromStdin(cmd.InOrStdin()), agent, cwd)
				wiki.MarkHotSeen(home, key, hot)
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
			}
			// A session that ended outside a project repo — in the vault, or
			// anywhere else — has no scope of its own. That is ordinary, not a
			// failure: it still gets to lend its moment to the sweep below.

			// The repo just worked in is fine; lend the moment to whichever
			// repo is most behind. Backlogs collect where sessions do not.
			if err := wiki.SweepDrain(guards, true); err != nil {
				wiki.LogAutosync("hook session-end: sweep — %v", err)
			}
			return nil
		},
	}
	sessionEnd.Flags().StringVar(&agent, "agent", "claude", "claude или codex")

	promptSubmit := &cobra.Command{
		Use:   "prompt-submit",
		Short: "UserPromptSubmit: досылать то, что память узнала за время сессии",
		Long: "SessionStart вкладывает память один раз, и сессия живёт с этой копией\n" +
			"неделями — а куратор разбирает инбокс по нескольку раз в день.\n\n" +
			"Печатает только строки, появившиеся в hot.md с прошлого раза. Ничего\n" +
			"не изменилось — ничего не печатает, и это обычный случай.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil
			}
			wiki.TouchHook(home, agent, "prompt-submit")
			cwd, err := os.Getwd()
			if err != nil {
				return nil
			}
			hot := wiki.RepoHotFile(cwd)
			if hot == "" {
				return nil
			}
			key := wiki.SessionKey(sessionIDFromStdin(cmd.InOrStdin()), agent, cwd)
			if delta := wiki.HotDelta(home, key, hot); delta != "" {
				fmt.Fprintln(cmd.OutOrStdout(), wiki.PromptHookJSON(delta))
			}
			return nil
		},
	}
	promptSubmit.Flags().StringVar(&agent, "agent", "claude", "claude или codex")

	cmd.AddCommand(sessionStart, sessionEnd, promptSubmit)
	return cmd
}
