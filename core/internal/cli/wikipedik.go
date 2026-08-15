package cli

import (
	"fmt"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/wiki"
	"github.com/spf13/cobra"
)

func newWikipedikCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wikipedik",
		Short: "Вольт и его личные зоны: research и brand",
		Long: "Вольт шире памяти проектов. `hq wiki` владеет dev-зоной (контур памяти\n" +
			"вокруг проектов); `hq wikipedik` — сам вольт и личные зоны: research\n" +
			"(обучение) и brand (статьи, контент). Структура зон намеренно не\n" +
			"навязывается — каркас первичен, пайплайны отдельным разговором.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWikipedikOverview(cmd)
		},
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "autosync",
			Short: "Страховка: закоммитить вольт, если агент этого не сделал",
			Long: "Вызывается Stop-хуком после каждого ответа агента. Если вольт чист —\n" +
				"молчит и ничего не делает. Если нет — коммитит с сообщением,\n" +
				"собранным по изменённым файлам, и пушит.\n\n" +
				"Существует потому, что модель систематически пропускает служебные\n" +
				"фазы: правило в контракте — это ускорение, а не гарантия.",
			Hidden: true,
			Args:   cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				root, err := wiki.VaultRoot()
				if err != nil {
					return err
				}
				if len(wiki.Dirty(root)) == 0 {
					return nil
				}
				return wiki.AutocommitMessage(false, wiki.AutoMessage(root), cmd.OutOrStdout())
			},
		},
		&cobra.Command{
			Use:   "path [зона]",
			Short: "Путь зоны (root, dev, research, brand)",
			Long: "Печатает каталог зоны; обёртка в оболочке делает cd. Каталог не\n" +
				"создаётся — это делает вход в зону.",
			Args: cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				path, err := wiki.ZonePath(argAt(args, 0))
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), path)
				return nil
			},
		},
		wikipedikSyncCmd(),
	)
	return cmd
}

func wikipedikSyncCmd() *cobra.Command {
	var message string
	var ifDue bool

	cmd := &cobra.Command{
		Use:   "sync [краткое описание]",
		Short: "Закоммитить весь вольт и запушить",
		Long: "Коммитит всё некоммиченное в вольте одним коммитом, затем best-effort\n" +
			"pull --rebase и push. Перед коммитом дифф сканируется на секреты.\n\n" +
			"Описание — кратко и по-русски; префикс Conventional Commit ставится\n" +
			"автоматически, если его нет. Это то, чем агент завершает каждое\n" +
			"изменение вольта.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && message == "" {
				message = args[0]
			}
			return wiki.AutocommitMessage(ifDue, wiki.VaultCommitMessage(message), cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "краткое описание изменения")
	cmd.Flags().BoolVar(&ifDue, "if-due", false, "только если прошлый запуск был больше суток назад")
	return cmd
}

func runWikipedikOverview(cmd *cobra.Command) error {
	zones, err := wiki.ZoneOverview()
	if err != nil {
		return err
	}
	state, err := wiki.VaultStatus()
	if err != nil {
		return err
	}

	r := ui.New(cmd.OutOrStdout())
	r.Blank()
	r.Line("  " + r.Bold(state.Root))
	if state.LastCommit != "" {
		r.Line("  " + r.Muted("последний коммит: "+state.LastCommit))
	}
	r.Blank()

	for _, zone := range zones {
		mark := r.OK("✓")
		detail := ui.Plural(zone.Files, "файл", "файла", "файлов")
		if !zone.Exists {
			mark = r.Muted("·")
			detail = "зоны нет — появится при входе (wikipedik " + zone.Name + ")"
		}
		r.Line(fmt.Sprintf("  %s %s  %s", mark, r.Accent(fmt.Sprintf("%-9s", zone.Name)),
			r.Muted(zone.About+" · "+detail)))
	}

	r.Blank()
	switch {
	case state.Dirty > 0:
		r.Line("  " + r.Drifted(fmt.Sprintf("незакоммичено: %d", state.Dirty)) +
			r.Muted("  → hq wikipedik sync"))
	default:
		r.Line("  " + r.OK("вольт чист"))
	}
	if state.Unpushed > 0 {
		r.Line("  " + r.Drifted(fmt.Sprintf("незапушено коммитов: %d", state.Unpushed)))
	}
	r.Blank()
	return nil
}
