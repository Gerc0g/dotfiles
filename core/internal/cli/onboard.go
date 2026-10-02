package cli

import (
	"github.com/Gerc0g/dotfiles/core/onboard"
	"github.com/spf13/cobra"
)

func newOnboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Создание компаний, продуктов и репозиториев",
		Long: "Компания — каталог с .company-config, SSH-идентичностью (один ключ на\n" +
			"компанию). Продукт — каталог с .product-config и\n" +
			"клонами репозиториев. AGENTS.md рендерятся из ~/dotfiles/templates.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	cmd.AddCommand(onboardCompanyCmd(), onboardProductCmd(), onboardRepoCmd())
	return cmd
}

func onboardRepoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "repo <company> <product> <repo...>",
		Short: "Добавить репозитории без перезаписи контекста продукта",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return onboard.Repositories(onboard.ProductOptions{Company: args[0], Product: args[1], Repos: args[2:]}, cmd.OutOrStdout())
		},
	}
}

func onboardCompanyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "company <slug> <vcs[:host]> <namespace> [git-email]",
		Short: "Создать компанию",
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			return onboard.Company(onboard.CompanyOptions{
				Slug:      args[0],
				VCS:       args[1],
				Namespace: args[2],
				Email:     argAt(args, 3),
			}, cmd.OutOrStdout())
		},
	}
}

func onboardProductCmd() *cobra.Command {
	var ns string
	cmd := &cobra.Command{
		Use:   "product <company> <product> [repo...]",
		Short: "Создать продукт и склонировать репозитории",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return onboard.Product(onboard.ProductOptions{
				Company:   args[0],
				Product:   args[1],
				Namespace: ns,
				Repos:     args[2:],
			}, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&ns, "ns", "", "переопределить namespace компании")
	return cmd
}
