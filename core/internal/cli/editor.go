package cli

import (
	"errors"
	"fmt"

	"github.com/Gerc0g/dotfiles/core/editor"
	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/Gerc0g/dotfiles/core/world"
	"github.com/spf13/cobra"
)

func newEditorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "editor",
		Short: "Зеркало модели мира в редакторе",
		Long: "Держит список проектов VS Code Project Manager равным тому, что лежит\n" +
			"на диске: продукты, репозитории и живые worktree. Управляемые записи\n" +
			"помечены тегом dotfiles; чужие записи по умолчанию вычищаются.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	cmd.AddCommand(editorSyncCmd())
	return cmd
}

type editorSyncOptions struct {
	dryRun           bool
	check            bool
	reposOnly        bool
	productsOnly     bool
	noWorktrees      bool
	preserveExternal bool
	noPrune          bool
	noBackup         bool
	projectFile      string
}

func editorSyncCmd() *cobra.Command {
	var opts editorSyncOptions

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Синхронизировать projects.json с моделью мира",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEditorSync(cmd, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "показать изменения без записи")
	cmd.Flags().BoolVar(&opts.check, "check", false, "выйти с ошибкой, если файл не синхронизирован")
	cmd.Flags().BoolVar(&opts.reposOnly, "repos-only", false, "только репозитории")
	cmd.Flags().BoolVar(&opts.productsOnly, "products-only", false, "только продукты")
	cmd.Flags().BoolVar(&opts.noWorktrees, "no-worktrees", false, "без agent worktree")
	cmd.Flags().BoolVar(&opts.preserveExternal, "preserve-external", false, "сохранить записи, добавленные вручную")
	cmd.Flags().BoolVar(&opts.noPrune, "no-prune", false, "не удалять протухшие управляемые записи")
	cmd.Flags().BoolVar(&opts.noBackup, "no-backup", false, "не делать бэкап projects.json")
	cmd.Flags().StringVar(&opts.projectFile, "project-file", "", "путь к projects.json (по умолчанию — стандартный)")

	return cmd
}

func runEditorSync(cmd *cobra.Command, opts editorSyncOptions) error {
	if opts.reposOnly && opts.productsOnly {
		return errors.New("--repos-only и --products-only взаимоисключающие")
	}

	root, err := worldRoot()
	if err != nil {
		return err
	}
	tree, err := world.Scan(root)
	if err != nil {
		return err
	}

	path := opts.projectFile
	if path == "" {
		path, err = editor.DefaultProjectsFile()
		if err != nil {
			return err
		}
	}

	desired := editor.FromTree(tree, !opts.reposOnly, !opts.productsOnly)
	if !opts.noWorktrees {
		m, err := newManager()
		if err != nil {
			return err
		}
		list, err := m.List()
		if err != nil {
			return err
		}
		desired = append(desired, editor.FromWorktrees(workspace.EditorWorktrees(list))...)
	}

	syncOpts := editor.SyncOptions{
		Products:         !opts.reposOnly,
		Repos:            !opts.productsOnly,
		Prune:            !opts.noPrune,
		PreserveExternal: opts.preserveExternal,
		Backup:           !opts.noBackup,
		DryRun:           opts.dryRun || opts.check,
	}

	res, err := editor.Sync(path, desired, syncOpts)
	if err != nil {
		return err
	}

	r := ui.New(cmd.OutOrStdout())
	r.Line(r.Muted("файл: ") + path)
	r.Line(r.Muted("записей в модели: ") + fmt.Sprintf("%d", res.Desired))
	r.Line(r.Muted("изменения: ") + res.Stats.String())

	switch {
	case opts.check && res.Changed:
		return errors.New("не синхронизировано")
	case opts.check:
		r.Line(r.OK("уже синхронизировано"))
	case opts.dryRun:
		r.Line(r.Muted("dry-run: без записи"))
	case !res.Changed:
		r.Line(r.OK("уже синхронизировано"))
	default:
		if res.BackupPath != "" {
			r.Line(r.Muted("бэкап: ") + res.BackupPath)
		}
		r.Line(r.OK("синхронизировано"))
	}
	return nil
}
