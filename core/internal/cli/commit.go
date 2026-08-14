package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/spf13/cobra"
)

// verifyEnv keeps the switch the agents already know: AGENT_GIT_VERIFY=0
// skips the make gate.
const verifyEnv = "AGENT_GIT_VERIFY"

func newCommitCmd() *cobra.Command {
	var noVerify bool

	cmd := &cobra.Command{
		Use:   "commit \"type(scope): описание\" -- <путь> [путь...]",
		Short: "Один логический коммит по явным путям",
		Long: "Коммитит ровно то, что названо: широкие пути (., :/, -A) запрещены,\n" +
			"индекс перед стейджингом сбрасывается, чтобы не увезти чужое. Перед\n" +
			"коммитом гоняется make verify (или test) репозитория. Не пушит:\n" +
			"завершение задачи — hq finish.\n\n" +
			"Идентичность проверяется по git_email компании, включая worktree.",
		Args: func(cmd *cobra.Command, args []string) error {
			if cmd.ArgsLenAtDash() != 1 {
				return errors.New("нужно: hq commit \"<сообщение>\" -- <путь> [путь...]")
			}
			if len(args) < 2 {
				return errors.New("после -- нужен хотя бы один путь")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := worldRoot()
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("текущий каталог: %w", err)
			}

			opts := workspace.CommitOptions{
				Message: args[0],
				Paths:   args[1:],
				Verify:  !noVerify && os.Getenv(verifyEnv) != "0",
				Out:     cmd.OutOrStdout(),
			}

			err = workspace.Commit(root, cwd, opts)
			if errors.Is(err, workspace.ErrNothingToCommit) {
				fmt.Fprintln(cmd.OutOrStdout(), "нечего коммитить в указанных путях")
				return nil
			}
			return err
		},
	}

	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "пропустить make verify/test")
	return cmd
}
