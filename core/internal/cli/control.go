package cli

import (
	"github.com/Gerc0g/dotfiles/core/control"
	"github.com/spf13/cobra"
)

func newControlCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{Use: "control", Short: "Типизированные операции серверного HQ через stdin", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return control.Serve(cmd.InOrStdin(), cmd.OutOrStdout())
	}}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "машинный формат JSON")
	return cmd
}
