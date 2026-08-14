package cli

import (
	"fmt"
	"os"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/spf13/cobra"
)

func newCtxCmd() *cobra.Command {
	var plain bool

	cmd := &cobra.Command{
		Use:   "ctx [каталог]",
		Short: "Определить контекст каталога: компания/продукт/репозиторий",
		Long: "Отвечает на вопрос «в каком месте модели мира я нахожусь» — из любого\n" +
			"уровня вложенности, включая worktree-пул. Это единственный резолвер\n" +
			"контекста; скрипты и обёртки не должны разбирать пути сами.\n\n" +
			"Без аргумента берётся текущий каталог. Режим --plain печатает\n" +
			"key=value по строке — для оболочки и агентов.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := argAt(args, 0)
			if dir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("текущий каталог: %w", err)
				}
				dir = cwd
			}

			root, err := worldRoot()
			if err != nil {
				return err
			}

			ctx, err := workspace.Locate(root, dir)
			if err != nil {
				return err
			}

			if plain {
				printCtxPlain(cmd, ctx)
				return nil
			}

			r := ui.New(cmd.OutOrStdout())
			r.Blank()
			r.Line("  " + r.Accent(ctx.Ref()) + "  " + r.Muted(ctx.Kind))
			if ctx.Kind == "worktree" {
				r.Line("  " + r.Muted(fmt.Sprintf("задача %s · ветка %s · %s", ctx.Task, ctx.Branch, ctx.State)))
			}
			r.Line("  " + r.Muted(ctx.Path))
			r.Blank()
			return nil
		},
	}

	cmd.Flags().BoolVar(&plain, "plain", false, "key=value по строке, без оформления")
	return cmd
}

func printCtxPlain(cmd *cobra.Command, ctx workspace.Context) {
	out := cmd.OutOrStdout()
	write := func(key, value string) {
		if value != "" {
			fmt.Fprintf(out, "%s=%s\n", key, value)
		}
	}
	write("kind", ctx.Kind)
	write("company", ctx.Company)
	write("product", ctx.Product)
	write("repo", ctx.Repo)
	write("id", ctx.ID)
	write("task", ctx.Task)
	write("branch", ctx.Branch)
	write("state", ctx.State)
	write("path", ctx.Path)
}
