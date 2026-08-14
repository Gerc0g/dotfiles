package cli

import (
	"fmt"
	"os"

	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/spf13/cobra"
)

func newFinishCmd() *cobra.Command {
	var opts workspace.FinishOptions

	cmd := &cobra.Command{
		Use:   "finish",
		Short: "Завершить задачу: verify, push, draft PR/MR",
		Long: "Прогоняет verify, пушит ветку задачи, открывает draft PR/MR в dev/main\n" +
			"и записывает состояние в метаданные воркспейса. Никогда не мерджит.\n\n" +
			"--no-review — низкоуровневый фолбэк: только push, без ревью и verify\n" +
			"(бывший agent-task-push). Интеграционные ветки в этом режиме защищены\n" +
			"переменной AGENT_ALLOW_INTEGRATION_PUSH=1.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("текущий каталог: %w", err)
			}
			opts.Out = cmd.OutOrStdout()
			return workspace.Finish(cwd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.Base, "base", "", "целевая ветка ревью (по умолчанию — из метаданных, затем dev/main)")
	cmd.Flags().StringVar(&opts.Title, "title", "", "заголовок PR/MR (по умолчанию — задача)")
	cmd.Flags().BoolVar(&opts.ReadyWithoutReview, "ready-without-review", false, "пометить ready, если ревью не открылось")
	cmd.Flags().BoolVar(&opts.NoReview, "no-review", false, "только push, без PR/MR")
	cmd.Flags().BoolVar(&opts.AllowDirty, "allow-dirty", false, "пушить при грязном дереве (только с --no-review)")
	return cmd
}
