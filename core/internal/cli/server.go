package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// The previous server layer assumed exactly one machine at a fixed ssh alias:
// provisioning script, tmux sessions over ssh, a remote dev-stack, a tailnet
// route pin. Every piece hardcoded that single box, so adding a second would
// have meant rewriting all of them.
//
// It was removed rather than adapted. These commands hold the surface while the
// multi-server model is designed: what a server is, how it is registered, what
// runs on it, and how a workspace targets one.
const notImplemented = "серверный слой пока не реализован.\n\n" +
	"Старый был удалён целиком: он предполагал ровно одну машину по\n" +
	"фиксированному ssh-алиасу, и второй сервер в него не помещался.\n" +
	"Новая модель будет рассчитана на несколько машин."

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Управление серверами (заглушка)",
		Long: "Регистрация машин, на которых выполняется работа: сборка, инфраструктура,\n" +
			"агенты. Пока не реализовано — команды держат поверхность, чтобы модель\n" +
			"проектировалась целиком, а не дописывалась по кусочку.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "подключённых серверов нет")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), notImplemented)
			return nil
		},
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "connect <имя>",
			Short: "Подключить сервер (заглушка)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("%s", notImplemented)
			},
		},
		&cobra.Command{
			Use:   "remove <имя>",
			Short: "Отключить сервер (заглушка)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("%s", notImplemented)
			},
		},
	)

	return cmd
}
