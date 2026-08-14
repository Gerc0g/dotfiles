package cli

import (
	"errors"
	"fmt"

	"github.com/Gerc0g/dotfiles/core/setup"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Привести машину к заявленному состоянию",
		Long: "Создаёт каталоги, симлинки и фоновые задачи, которые нужны платформе.\n\n" +
			"Запускать можно сколько угодно раз: каждый шаг сначала проверяет и\n" +
			"действует только при расхождении. Ничего не удаляется — обычный файл\n" +
			"на месте симлинка отодвигается в копию с меткой времени, а ссылки,\n" +
			"созданные не этой командой, показываются, но не трогаются.\n\n" +
			"Установка пакетов (Homebrew, npm, CLI агентов) осталась в bootstrap.sh:\n" +
			"тот запускается один раз на чистой машине, а этот — каждый раз, когда\n" +
			"состояние разошлось.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := newEnv()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()

			if dryRun {
				reports := setup.Check(env, setup.Plan())
				renderReports(out, reports)
				renderSummary(out, reports)
				if hasApplicableWork(reports) {
					fmt.Fprintln(out, "  запустите без --dry-run, чтобы применить")
				} else if needsAttention(reports) {
					fmt.Fprintln(out, "  автоматически здесь ничего не применить")
				}
				return nil
			}

			reports, errs := setup.Apply(env, setup.Plan())
			renderReports(out, reports)
			renderSummary(out, reports)

			if len(errs) > 0 {
				for _, err := range errs {
					fmt.Fprintf(cmd.ErrOrStderr(), "ошибка: %v\n", err)
				}
				return fmt.Errorf("шагов с ошибкой: %d", len(errs))
			}

			if needsAttention(reports) {
				return errors.New("часть шагов требует ручного вмешательства (см. выше)")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "показать изменения, ничего не применять")

	return cmd
}
