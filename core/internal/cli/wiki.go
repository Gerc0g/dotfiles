package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/Gerc0g/dotfiles/core/wiki"
	"github.com/Gerc0g/dotfiles/core/world"
	"github.com/spf13/cobra"
)

func newWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "WikiPedik — долговременная память проектов",
		Long: "Вольт — один git-репозиторий markdown-страниц. Воркеры пишут сырые\n" +
			"заметки в _inbox.md, куратор (codex) сливает их в curated-страницы,\n" +
			"hot.md инжектится в каждую сессию агента. Здесь — детерминированная\n" +
			"половина цикла; LLM-половина остаётся кураторскими скиллами.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	cmd.AddCommand(
		wikiSyncCmd(), wikiStatusCmd(), wikiSynthesizeCmd(), wikiCommitCmd(),
		wikiAutocommitCmd(), wikiHotRefreshCmd(), wikiRulesSyncCmd(), wikiBootstrapCmd(),
		wikiDrainCmd(),
	)
	return cmd
}

// wikiDrainCmd is the background half of curation: the same curator skill,
// but launched without a human answering per-entry prompts. Guards keep it
// from spending tokens on nothing.
func wikiDrainCmd() *cobra.Command {
	var (
		commit    bool
		force     bool
		minCands  int
		intervalH int
	)

	var background, check, sweep bool

	cmd := &cobra.Command{
		Use:   "drain [company[/product[/repo]]]",
		Short: "Фоновый разбор inbox куратором (без подтверждений)",
		Long: "Тот же скилл inbox-drain и то же суждение куратора — дедуп, проверка\n" +
			"выводимости, ретирация, — но без подтверждения каждой записи: уверенное\n" +
			"применяется, спорное остаётся candidate с объяснением. Ревью — по диффу\n" +
			"вольта, а не по 138 диалогам подряд.\n\n" +
			"Без аргумента скоуп берётся из текущего каталога — так его вызывает\n" +
			"SessionEnd-хук для репозитория, в котором только что работали.\n\n" +
			"Два гварда против лишних трат: порог кандидатов и интервал между\n" +
			"запусками. Каждый запуск и каждый пропуск пишутся в журнал\n" +
			"(~/.cache/wikipedik-autosync.log).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			guards := wiki.DefaultDrainGuards()
			if sweep {
				return wiki.SweepDrain(guards, commit)
			}
			scope, err := drainScope(args)
			if err != nil {
				return err
			}
			if minCands > 0 {
				guards.MinCandidates = minCands
			}
			if intervalH > 0 {
				guards.Interval = time.Duration(intervalH) * time.Hour
			}
			if force {
				guards = wiki.DrainGuards{MinCandidates: 1}
			}

			// --check answers "would this run, and why" without spending a
			// single token. Anything that costs money needs a dry way to ask.
			if check {
				decision, err := wiki.ShouldDrain(scope, guards)
				if err != nil {
					return err
				}
				verdict := "пропуск"
				if decision.Run {
					verdict = "запустился бы"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (%s)\n", scope, verdict, decision.Reason)
				return nil
			}

			// Hooks must not wait: a drain takes minutes, and the guards are
			// checked in the child anyway.
			if background {
				return wiki.SpawnDrain(scope, guards, commit)
			}
			return wiki.AutoDrain(scope, guards, commit, cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "показать решение гвардов, ничего не запуская")
	cmd.Flags().BoolVar(&sweep, "sweep", false, "разобрать самый запущенный репо вольта, а не текущий")
	cmd.Flags().BoolVar(&background, "background", false, "запустить отдельным процессом и сразу вернуться")
	cmd.Flags().BoolVar(&commit, "commit", false, "закоммитить результат scope-атомарно")
	cmd.Flags().BoolVar(&force, "force", false, "игнорировать гварды (порог и интервал)")
	cmd.Flags().IntVar(&minCands, "min-candidates", 0, "порог кандидатов (по умолчанию 5)")
	cmd.Flags().IntVar(&intervalH, "interval-hours", 0, "минимум часов между разборами (по умолчанию 24)")
	return cmd
}

// drainScope takes the scope from the argument, or from the current
// directory when a hook calls it without one.
func drainScope(args []string) (wiki.Scope, error) {
	if len(args) == 1 {
		return wiki.ParseScope(args[0])
	}
	cwd, err := os.Getwd()
	if err != nil {
		return wiki.Scope{}, err
	}
	return wiki.ScopeForDir(cwd)
}

func parseScopeArg(args []string) (wiki.Scope, error) {
	if len(args) == 0 || args[0] == "" {
		return wiki.Scope{}, fmt.Errorf("нужен scope: company[/product[/repo]]")
	}
	return wiki.ParseScope(args[0])
}

func wikiSyncCmd() *cobra.Command {
	var opts wiki.SyncOptions
	cmd := &cobra.Command{
		Use:   "sync <company[/product[/repo]]>",
		Short: "Слить inbox в curated-страницы (куратор)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := parseScopeArg(args)
			if err != nil {
				return err
			}
			return wiki.Sync(scope, opts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&opts.Commit, "commit", false, "scope-атомарный коммит после синка")
	cmd.Flags().BoolVar(&opts.Push, "push", false, "коммит и push")
	return cmd
}

func wikiStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <company[/product[/repo]]>",
		Short: "Отчёт куратора без изменений файлов",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := parseScopeArg(args)
			if err != nil {
				return err
			}
			return wiki.Status(scope, cmd.OutOrStdout())
		},
	}
}

func wikiSynthesizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "synthesize <company[/product]>",
		Short: "Кросс-репо синтез паттернов (куратор)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := parseScopeArg(args)
			if err != nil {
				return err
			}
			return wiki.Synthesize(scope)
		},
	}
}

func wikiCommitCmd() *cobra.Command {
	var push bool
	cmd := &cobra.Command{
		Use:   "commit <company[/product[/repo]]>",
		Short: "Scope-атомарный коммит памяти",
		Long: "Коммитит только скоуп, его product log и корневой индекс. Автор —\n" +
			"git_email компании из .company-config. Грязные файлы вне скоупа\n" +
			"блокируют коммит.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := parseScopeArg(args)
			if err != nil {
				return err
			}
			return wiki.Commit(scope, push, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&push, "push", false, "запушить после коммита")
	return cmd
}

func wikiAutocommitCmd() *cobra.Command {
	var ifDue bool
	cmd := &cobra.Command{
		Use:   "autocommit",
		Short: "Catch-all коммит вольта + best-effort pull/push",
		Long: "Сметает всё грязное в один коммит (с secret-scan диффа), затем\n" +
			"pull --rebase и push, если есть remote. Сетевые проблемы только\n" +
			"предупреждают.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return wiki.Autocommit(ifDue, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&ifDue, "if-due", false, "только если прошлый запуск был больше суток назад")
	return cmd
}

func wikiHotRefreshCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "hot-refresh <company[/product[/repo]]>",
		Short: "Детерминированно пересобрать hot.md",
		Long: "Страховка на случай, когда куратор пропустил свою hot.md-фазу.\n" +
			"Безопасно запускать в любой момент.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := parseScopeArg(args)
			if err != nil {
				return err
			}
			return wiki.RefreshHot(scope, dryRun, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "показать цели без записи")
	return cmd
}

func wikiRulesSyncCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "rules-sync [company[/product[/repo]]]",
		Short: "Материализовать правила вольта в .claude/rules чекаутов",
		Long: "Симлинкует правила (repo + product) в main-чекауты и активные\n" +
			"worktree как .claude/rules/wiki-*.md. Правило без paths: frontmatter\n" +
			"пропускается — безусловная загрузка это раздувание контекста.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope := ""
			if len(args) == 1 {
				scope = args[0]
			}
			return wiki.RulesSync(scope, dryRun, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "показать изменения без записи")
	return cmd
}

func wikiBootstrapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap [company [product [repo]]]",
		Short: "Создать скелет памяти и симлинки в чекауты",
		Long: "Идемпотентно: существующие страницы не трогаются, пересоздаются\n" +
			"только managed-страницы графа. Privacy-граница структурная:\n" +
			"company-knowledge может указывать только внутрь своей компании.",
		Args: cobra.MaximumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWikiBootstrap(cmd, args)
		},
	}
}

func runWikiBootstrap(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	if err := wiki.BootstrapRoot(); err != nil {
		return err
	}

	tree, err := world.ScanDefault()
	if err != nil {
		return err
	}

	co, prod, repo := argAt(args, 0), argAt(args, 1), argAt(args, 2)
	if co == "" {
		return fmt.Errorf("нужна компания: hq wiki bootstrap <company> [product [repo]]\nкомпании: %s",
			companySlugs(tree))
	}
	if !tree.HasCompany(co) {
		return fmt.Errorf("нет компании %q (есть: %s)", co, companySlugs(tree))
	}

	fmt.Fprintf(out, "\n→ Bootstrapping company: %s\n", co)
	if err := wiki.BootstrapCompany(co, out); err != nil {
		return err
	}

	products := tree.Products(co)
	if prod != "" {
		if !tree.HasProduct(co, prod) {
			return fmt.Errorf("нет продукта %q в компании %q", prod, co)
		}
		products = products[:0]
		for _, p := range tree.Products(co) {
			if p.Slug == prod {
				products = append(products, p)
			}
		}
	}

	for _, p := range products {
		fmt.Fprintf(out, "\n→ Bootstrapping product: %s/%s\n", co, p.Slug)
		if err := wiki.BootstrapProduct(co, p.Slug, out); err != nil {
			return err
		}

		repos := tree.Repos(co, p.Slug)
		for _, r := range repos {
			if repo != "" && r.Slug != repo {
				continue
			}
			if err := wiki.BootstrapRepo(co, p.Slug, r.Slug, out); err != nil {
				fmt.Fprintf(out, "  ⚠ %v\n", err)
			}
		}
	}

	// Sanity checks over what was just bootstrapped.
	if prod != "" {
		for _, r := range tree.Repos(co, prod) {
			if repo != "" && r.Slug != repo {
				continue
			}
			wiki.SanityCheck(co, prod, r.Slug, out)
		}
	}

	fmt.Fprintln(out, "\n✅ wiki bootstrap завершён")
	fmt.Fprintln(out, "Дальше: открой вольт в Obsidian, проверь симлинки в одном репо,")
	fmt.Fprintf(out, "куратор обрабатывает inbox: hq wiki sync %s\n", co)
	return nil
}

func companySlugs(tree *world.Tree) string {
	names := ""
	for i, c := range tree.Companies() {
		if i > 0 {
			names += ", "
		}
		names += c.Slug
	}
	return names
}
