package cli

import (
	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/secret"
	"github.com/spf13/cobra"
)

func newSecretCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "secret",
		Short: "Секреты через 1Password (заглушка)",
		Long: "Слой секретов переделывается: старая реализация удалена, поверхность\n" +
			"держит место. Контракт будущего слоя — в докстринге core/secret\n" +
			"(vault Work-<co>, скоупные имена item'ов, TTL-кэш для direnv).\n\n" +
			"`secret signin` живёт в оболочке и работает как раньше.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return secret.ErrNotImplemented
			}
			r := ui.New(cmd.OutOrStdout())
			r.Line(r.Muted(secret.ErrNotImplemented.Error()))
			return nil
		},
	}
}
