package cli

import (
	"fmt"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/server"
	"github.com/spf13/cobra"
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Управление серверами (заглушка)",
		Long: "Регистрация машин, на которых выполняется работа: сборка, инфраструктура,\n" +
			"агенты. Пока не реализовано — команды держат поверхность, чтобы модель\n" +
			"проектировалась целиком, а не дописывалась по кусочку. Домен живёт в\n" +
			"core/server; полная реализация приземлится туда.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := server.Load()
			if err != nil {
				return err
			}

			r := ui.New(cmd.OutOrStdout())
			servers := registry.List()
			if len(servers) == 0 {
				r.Line("подключённых серверов нет")
				r.Blank()
				r.Line(r.Muted(server.ErrNotImplemented.Error()))
				return nil
			}
			for _, s := range servers {
				r.Line(fmt.Sprintf("  %s  %s", r.Accent(s.Name), r.Muted(s.Host)))
			}
			return nil
		},
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "connect <имя>",
			Short: "Подключить сервер (заглушка)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				registry, err := server.Load()
				if err != nil {
					return err
				}
				return registry.Connect(argAt(args, 0))
			},
		},
		&cobra.Command{
			Use:   "remove <имя>",
			Short: "Отключить сервер (заглушка)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				registry, err := server.Load()
				if err != nil {
					return err
				}
				return registry.Remove(argAt(args, 0))
			},
		},
	)

	return cmd
}
