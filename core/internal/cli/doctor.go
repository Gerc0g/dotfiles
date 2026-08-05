package cli

import (
	"errors"

	"github.com/Gerc0g/dotfiles/core/setup"
	"github.com/Gerc0g/dotfiles/core/world"
	"github.com/spf13/cobra"
)

func worldRoot() (string, error) {
	return world.Root()
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Проверить состояние машины, ничего не меняя",
		Long: "Проверяет ту же декларацию, которую применяет hq setup, и не трогает\n" +
			"ничего.\n\n" +
			"Онбординг выполняется один раз, а машина расходится месяцами. Это та\n" +
			"половина, которая ловит расхождение: профиль, потерявший базовые\n" +
			"правила, ссылку на удалённый скилл, выпавшую фоновую задачу.\n\n" +
			"Возвращает ненулевой код, если что-то не настроено или разошлось —\n" +
			"так его можно поставить проверкой в CI или предполётной в скрипт.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := newEnv()
			if err != nil {
				return err
			}

			reports := setup.Check(env, setup.Plan())
			renderReports(cmd.OutOrStdout(), reports)
			renderSummary(cmd.OutOrStdout(), reports)

			if hint := advice(reports); hint != "" {
				return errors.New("состояние машины расходится с декларацией. " + hint)
			}
			return nil
		},
	}
}
