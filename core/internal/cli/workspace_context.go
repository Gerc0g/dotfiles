package cli

import (
	"encoding/json"
	"fmt"

	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/spf13/cobra"
)

func wsContextCmd(repair bool) *cobra.Command {
	verb, short := "context-check", "Проверить контекст управляемого worktree без изменений"
	if repair {
		verb, short = "context-repair", "Добавить недостающий контекст, сохранив существующие файлы"
	}
	var asJSON bool
	cmd := &cobra.Command{
		Use: verb + " <компания> <продукт> <репозиторий> <id>", Short: short,
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := newManager()
			if err != nil {
				return err
			}
			w, err := m.Find(args[0], args[1], args[2], args[3])
			if err != nil {
				return err
			}
			var issues []workspace.ContextIssue
			if repair {
				issues, err = m.RepairContext(w)
			} else {
				issues, err = m.ContextIssues(w)
			}
			if asJSON {
				if issues == nil {
					issues = []workspace.ContextIssue{}
				}
				if encodeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
					Worktree string                   `json:"worktree"`
					Issues   []workspace.ContextIssue `json:"issues"`
				}{w.Path, issues}); encodeErr != nil {
					return encodeErr
				}
			} else if len(issues) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "контекст доступен: "+w.Path)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), workspace.ContextSummary(issues))
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Машиночитаемые пути и проблемы контекста")
	return cmd
}
