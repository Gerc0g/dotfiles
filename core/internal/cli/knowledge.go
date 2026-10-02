package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Gerc0g/dotfiles/core/control"
	"github.com/Gerc0g/dotfiles/core/knowledge"
	"github.com/spf13/cobra"
)

func newKnowledgeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "knowledge", Short: "Обслуживание серверной памяти Wikipedia"}
	var jsonOutput bool
	tick := &cobra.Command{Use: "tick", Short: "Запустить плановые задания отдельно для каждой области", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		response := control.Dispatch(control.Request{Version: 1, Operation: "knowledge.tick", Args: json.RawMessage(`{}`)})
		if !response.OK {
			return errors.New(response.Error.Code)
		}
		report, ok := response.Result.(knowledge.ScheduledReport)
		if !ok {
			return errors.New("invalid scheduled job response")
		}
		if jsonOutput {
			if err := json.NewEncoder(cmd.OutOrStdout()).Encode(report); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Memory jobs: %d, failed: %d\n", len(report.Jobs), report.Failed)
		}
		if report.Failed > 0 {
			return errors.New("one or more memory jobs failed; inspect scoped Wikipedia jobs")
		}
		return nil
	}}
	tick.Flags().BoolVar(&jsonOutput, "json", false, "машинный формат JSON")
	cmd.AddCommand(tick)
	return cmd
}
