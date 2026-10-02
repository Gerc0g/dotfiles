package cli

import (
	"errors"
	"fmt"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/spf13/cobra"
)

func newManager(opts ...workspace.Option) (*workspace.Manager, error) {
	root, err := worldRoot()
	if err != nil {
		return nil, err
	}
	return workspace.New(root, opts...)
}

func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"ws"},
		Short:   "Изолированные worktree под задачи",
		Long: "Одна задача — один worktree на собственной ветке agent/<задача>-<id>,\n" +
			"поэтому агент не пишет в тот чекаут, который открыт у тебя, и несколько\n" +
			"задач по одному репозиторию идут параллельно.\n\n" +
			"Удаление намеренно параноидальное: воркспейс исчезает, только если ты\n" +
			"назвал его поимённо или сошлись все проверки cleanup. Раньше уборка\n" +
			"срабатывала сама и дважды снесла живую работу.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	cmd.AddCommand(
		wsStartCmd(), wsListCmd(), wsStaleCmd(), wsPathCmd(),
		wsReadyCmd(), wsRemoveCmd(), wsCleanupCmd(), wsPruneCmd(),
		wsContextCmd(false), wsContextCmd(true),
	)
	return cmd
}

func wsPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path <компания> <продукт> <репозиторий> <id>",
		Short: "Путь управляемого воркспейса",
		Long: "Печатает каталог воркспейса, если он существует и управляем. Обёртки\n" +
			"переходят по нему, не зная раскладку пула.",
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
			fmt.Fprintln(cmd.OutOrStdout(), w.Path)
			return nil
		},
	}
}

func wsStartCmd() *cobra.Command {
	var requestID string
	cmd := &cobra.Command{
		Use:   "start <компания> <продукт> <репозиторий> <задача>",
		Short: "Создать worktree под задачу",
		Long: "Печатает путь созданного worktree в stdout, чтобы вызывающая оболочка\n" +
			"могла перейти в него: сменить каталог за пределами своего процесса\n" +
			"программа не может.",
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := newManager(workspace.WithWarnings(cmd.ErrOrStderr()))
			if err != nil {
				if requestID != "" {
					return fmt.Errorf("HQ_WORKTREE_NOT_CREATED: %w", err)
				}
				return err
			}
			w, err := m.StartWithRequestID(args[0], args[1], args[2], args[3], requestID)
			if err != nil {
				var notStarted *workspace.CreationNotStartedError
				if requestID != "" && errors.As(err, &notStarted) {
					return fmt.Errorf("HQ_WORKTREE_NOT_CREATED: %w", err)
				}
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), w.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Opaque creation request identity for launch reconciliation")
	return cmd
}

func wsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Показать воркспейсы",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := newManager()
			if err != nil {
				return err
			}
			all, err := m.List()
			if err != nil {
				return err
			}

			r := ui.New(cmd.OutOrStdout())
			if len(all) == 0 {
				r.Line("воркспейсов нет")
				return nil
			}

			r.Blank()
			for _, w := range all {
				r.Line(fmt.Sprintf("  %s  %s  %s",
					r.Accent(w.Ref()), w.Task, r.Muted(string(w.State))))
			}
			r.Blank()
			r.Line("  " + r.Muted(ui.Plural(len(all), "воркспейс", "воркспейса", "воркспейсов")))
			r.Blank()
			return nil
		},
	}
}

func wsStaleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stale [дней]",
		Short: "Активные воркспейсы без коммитов N дней",
		Long: "Cleanup их не трогает — в них может лежать незапушенная работа, — поэтому\n" +
			"они копятся молча. Этот список для ручного разбора.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			days := 3
			if len(args) == 1 {
				if _, err := fmt.Sscanf(args[0], "%d", &days); err != nil {
					return fmt.Errorf("дни должны быть числом: %q", args[0])
				}
			}

			m, err := newManager()
			if err != nil {
				return err
			}
			stale, err := m.Stale(days)
			if err != nil {
				return err
			}

			r := ui.New(cmd.OutOrStdout())
			if len(stale) == 0 {
				r.Line(fmt.Sprintf("нет простаивающих воркспейсов (>= %d дн.)", days))
				return nil
			}

			r.Blank()
			for _, s := range stale {
				r.Line(fmt.Sprintf("  %s  %s",
					r.Accent(s.Workspace.Ref()),
					r.Muted(fmt.Sprintf("простой %d дн. · незакоммичено %d · незапушено %d",
						s.IdleDays, s.Dirty, s.Unpushed))))
			}
			r.Blank()
			r.Line("  " + r.Muted("допушить незаконченное, чистые пометить ready, брошенные — remove"))
			r.Blank()
			return nil
		},
	}
}

func wsReadyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ready <компания> <продукт> <репозиторий> <id>",
		Short: "Пометить воркспейс завершённым",
		Long:  "Только после этого cleanup вправе его удалить.",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := newManager()
			if err != nil {
				return err
			}
			w, err := m.Ready(args[0], args[1], args[2], args[3])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s помечен ready\n", w.Ref())
			return nil
		},
	}
}

func wsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <компания> <продукт> <репозиторий> <id>",
		Short: "Удалить воркспейс поимённо",
		Long: "Артефакты сначала спасаются в вольт. Git сам откажется удалять worktree\n" +
			"с изменёнными отслеживаемыми файлами — этот отказ и есть страховка,\n" +
			"и он намеренно не подавляется.",
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := newManager()
			if err != nil {
				return err
			}
			if err := m.Remove(args[0], args[1], args[2], args[3]); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "удалён")
			return nil
		},
	}
}

func wsCleanupCmd() *cobra.Command {
	var opts workspace.CleanupOptions

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Удалить завершённые воркспейсы",
		Long: "Удаляет только те, у которых сошлось всё: помечен ready (или review влит),\n" +
			"пролежал в этом состоянии нужное число дней, дерево чистое, есть upstream\n" +
			"и ничего не осталось незапушенным. Любая невыполненная проверка оставляет\n" +
			"воркспейс на месте.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := newManager()
			if err != nil {
				return err
			}
			results, err := m.Cleanup(opts)
			if err != nil {
				return err
			}

			r := ui.New(cmd.OutOrStdout())
			removed := 0
			r.Blank()
			for _, res := range results {
				mark := r.Muted("·")
				if res.Removed {
					mark = r.OK("✓")
					removed++
				}
				r.Line(fmt.Sprintf("  %s %s  %s", mark, r.Accent(res.Workspace.Ref()), r.Muted(res.Reason)))
			}
			r.Blank()
			r.Line("  " + r.Muted(fmt.Sprintf("удалено %d из %d", removed, len(results))))
			r.Blank()
			return nil
		},
	}

	cmd.Flags().IntVar(&opts.Days, "days", 7, "сколько дней воркспейс должен быть ready")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "показать, ничего не удаляя")
	return cmd
}

func wsPruneCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "prune-branches",
		Short: "Удалить слитые или запушенные ветки agent/*",
		Long: "Ветка, на которой стоит живой worktree, не рассматривается вообще.\n" +
			"Остальные удаляются, только если влиты в базовую или полностью запушены.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := newManager()
			if err != nil {
				return err
			}
			lines, err := m.PruneBranches(dryRun)
			if err != nil {
				return err
			}

			r := ui.New(cmd.OutOrStdout())
			if len(lines) == 0 {
				r.Line("веток agent/* не найдено")
				return nil
			}
			r.Blank()
			for _, line := range lines {
				r.Line("  " + line)
			}
			r.Blank()
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "показать, ничего не удаляя")
	return cmd
}
