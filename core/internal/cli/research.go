package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/research"
	"github.com/spf13/cobra"
)

// These commands are the agent's toolbox, not the user's: the user works
// through chat only. Hence --plain everywhere — cheap, parseable output.
func newResearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research",
		Short: "Зона обучения: конспекты, связи, качество",
		Long: "Инструменты агента-писаря над зоной research. Единица — тема, которую\n" +
			"пользователь изучил сам; одна тема = один растущий файл. Всё\n" +
			"производное (индекс, отчёт качества, кандидаты в карты) вычисляется\n" +
			"здесь, а не ведётся руками.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return researchStatus(cmd) },
	}

	cmd.AddCommand(
		researchNewCmd(), researchListCmd(), researchLintCmd(),
		researchIndexCmd(), researchNextCmd(), researchMapCmd(), researchMapCandidatesCmd(), researchPathCmd(),
		researchGraphCmd(),
	)
	return cmd
}

func researchGraphCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "graph",
		Short: "Настроить граф Obsidian под учебную зону",
		Long: "Оставляет в графе только research (без индекса и журнала — они\n" +
			"ссылаются на всё и стягивают граф в звезду), красит темы по доменам,\n" +
			"показывает направление связей и сирот.\n\n" +
			"Прежние настройки сохраняются в graph.json.bak.<время>. После\n" +
			"применения перезапусти Obsidian: он держит настройки графа в памяти.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, backup, err := research.WriteGraph()
			if err != nil {
				return err
			}
			r := ui.New(cmd.OutOrStdout())
			r.Line(r.OK("✓ ") + "настройки графа: " + path)
			if backup != "" {
				r.Line(r.Muted("  прежние: " + backup))
			}
			r.Line(r.Muted("  перезапусти Obsidian, чтобы применилось"))
			return nil
		},
	}
}

func researchStatus(cmd *cobra.Command) error {
	zone, err := research.Scan()
	if err != nil {
		return err
	}
	findings := research.Lint(zone)

	byDomain := map[string]int{}
	solid := 0
	for _, topic := range zone.Topics {
		byDomain[topic.Domain]++
		if topic.Status == research.StatusSolid {
			solid++
		}
	}

	r := ui.New(cmd.OutOrStdout())
	r.Blank()
	r.Line("  " + r.Bold(zone.Root))
	r.Blank()
	if len(zone.Topics) == 0 {
		r.Line("  " + r.Muted("конспектов пока нет"))
		r.Blank()
		return nil
	}

	var parts []string
	for _, domain := range research.Domains {
		if byDomain[domain] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", domain, byDomain[domain]))
		}
	}
	r.Line(fmt.Sprintf("  %s  %s", r.Accent(ui.Plural(len(zone.Topics), "тема", "темы", "тем")),
		r.Muted(strings.Join(parts, " · "))))
	r.Line("  " + r.Muted(fmt.Sprintf("solid %d · growing %d · карт %d · источников %d",
		solid, len(zone.Topics)-solid, len(zone.Maps), len(zone.Sources))))

	if loose := research.Unmapped(zone); len(loose) > 0 {
		var names []string
		for i, topic := range loose {
			if i == 4 {
				names = append(names, fmt.Sprintf("и ещё %d", len(loose)-4))
				break
			}
			names = append(names, topic.Slug)
		}
		r.Blank()
		r.Line("  " + r.Drifted(ui.Plural(len(loose), "тема", "темы", "тем")+" вне карт") +
			"  " + r.Muted(strings.Join(names, ", ")))
		r.Line("  " + r.Muted("роадмап о них не знает — внести в маршрут или оставить как есть"))
	}

	errorsN, warnsN := 0, 0
	for _, finding := range findings {
		switch finding.Severity {
		case research.SeverityError:
			errorsN++
		case research.SeverityWarn:
			warnsN++
		}
	}
	r.Blank()
	switch {
	case errorsN > 0:
		r.Line("  " + r.Missing(fmt.Sprintf("качество: %d ошибок, %d предупреждений", errorsN, warnsN)) +
			r.Muted("  → hq research lint"))
	case warnsN > 0:
		r.Line("  " + r.Drifted(fmt.Sprintf("качество: %d предупреждений", warnsN)))
	default:
		r.Line("  " + r.OK("качество: чисто"))
	}
	r.Blank()
	return nil
}

func researchNewCmd() *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:   "new <домен>/<slug>",
		Short: "Создать тему из шаблона",
		Long: "Slug — lowercase kebab-case латиницей (ссылки переживают переименование\n" +
			"заголовка), --title — русское название в H1 и aliases.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, slug, found := strings.Cut(args[0], "/")
			if !found {
				return errors.New("нужно: hq research new <домен>/<slug> --title \"Название\"")
			}
			path, err := research.New(domain, slug, title)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "русский заголовок темы")
	return cmd
}

func researchListCmd() *cobra.Command {
	var domain string
	var plain bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Список тем (проверить, нет ли уже такой)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			zone, err := research.Scan()
			if err != nil {
				return err
			}
			r := ui.New(cmd.OutOrStdout())
			for _, topic := range zone.Topics {
				if domain != "" && topic.Domain != domain {
					continue
				}
				if plain {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n",
						topic.Ref, topic.Status, topic.DisplayName(), strings.Join(topic.Aliases, ","))
					continue
				}
				r.Line(fmt.Sprintf("  %-28s %s %s", r.Accent(topic.Ref),
					topic.DisplayName(), r.Muted(topic.Status)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&domain, "domain", "", "только один домен")
	cmd.Flags().BoolVar(&plain, "plain", false, "машинный вывод: ref, status, title, aliases")
	return cmd
}

func researchLintCmd() *cobra.Command {
	var plain bool
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Проверить качество конспектов",
		Long: "Сторож читаемости: каркас, LaTeX, точность цитат, связи, дубли.\n" +
			"Свободную часть конспекта не трогает.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			zone, err := research.Scan()
			if err != nil {
				return err
			}
			findings := research.Lint(zone)

			if plain {
				for _, finding := range findings {
					fmt.Fprintln(cmd.OutOrStdout(), finding.String())
				}
				return nil
			}

			r := ui.New(cmd.OutOrStdout())
			if len(findings) == 0 {
				r.Line(r.OK("конспекты в порядке"))
				return nil
			}
			r.Blank()
			for _, finding := range findings {
				mark := r.Muted("·")
				switch finding.Severity {
				case research.SeverityError:
					mark = r.Missing("✗")
				case research.SeverityWarn:
					mark = r.Drifted("!")
				}
				r.Line(fmt.Sprintf("  %s %-22s %s", mark, r.Accent(finding.Ref), finding.Message))
			}
			r.Blank()
			return nil
		},
	}
	cmd.Flags().BoolVar(&plain, "plain", false, "severity\\trule\\tref\\tmessage")
	return cmd
}

func researchIndexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "index",
		Short: "Перегенерировать index.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			zone, err := research.Scan()
			if err != nil {
				return err
			}
			path, err := research.WriteIndex(zone)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)

			// Coverage in the roadmaps is derived from the same scan, so it is
			// refreshed here rather than waiting for someone to remember a
			// second command.
			changed, err := research.RefreshMaps(zone)
			if err != nil {
				return err
			}
			if changed > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "покрытие обновлено в %d карт(ах)\n", changed)
			}
			return nil
		},
	}
}

func researchNextCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "next [область]",
		Short: "Что изучать дальше по маршрутам",
		Long: "Карта — справочник области: в ней сотни пунктов, и смотреть на них\n" +
			"каждый раз бессмысленно. Маршрут уже упорядочен по предпосылкам, значит\n" +
			"«дальше» — это первый пункт без конспекта.\n\n" +
			"Без аргумента показывает по одному пункту из каждой области — чтобы было\n" +
			"из чего выбрать. С областью — очередь на несколько шагов вперёд.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			zone, err := research.Scan()
			if err != nil {
				return err
			}
			area := ""
			if len(args) == 1 {
				area = args[0]
			}

			items := research.Next(zone, area, limit)
			r := ui.New(cmd.OutOrStdout())
			r.Blank()
			if len(items) == 0 {
				r.Line("  " + r.Muted("маршрутов нет или всё пройдено"))
				r.Blank()
				return nil
			}
			for _, item := range items {
				r.Line(fmt.Sprintf("  %s  %s", r.Accent(item.Link), r.Muted(item.Area)))
				if item.Note != "" {
					r.Line("  " + r.Muted(ui.Truncate(item.Note, r.Width()-4)))
				}
			}
			r.Blank()
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 5, "сколько шагов показать для области")
	return cmd
}

func researchMapCmd() *cobra.Command {
	var title, domain string
	cmd := &cobra.Command{
		Use:   "map <slug>",
		Short: "Создать карту области: маршрут, пробелы, покрытие",
		Long: "Карта — это роадмап области: что изучить, в каком порядке и как далеко\n" +
			"вы продвинулись. Вложенность любая — разделы и подразделы обычными\n" +
			"заголовками, файловая система тут ни при чём.\n\n" +
			"Статус каждого пункта и полосы покрытия проставляются сами из статусов\n" +
			"конспектов: solid — закрыто, growing — в работе, нет файла — не начато.\n" +
			"Руками их не ставят, иначе карта начнёт врать. Ссылка на неизученное\n" +
			"здесь не ошибка — линтер карты не проверяет.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			zone, err := research.Scan()
			if err != nil {
				return err
			}
			path, err := research.NewMap(zone.Root, args[0], title, domain)
			if err != nil {
				return err
			}
			if _, err := research.RefreshMap(path, zone); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "русское название области")
	cmd.Flags().StringVar(&domain, "domain", "", "домен: ml, math, quant, swe")
	return cmd
}

func researchMapCandidatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "map-candidates",
		Short: "Теги, по которым пора собрать карту",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			zone, err := research.Scan()
			if err != nil {
				return err
			}
			for _, candidate := range research.MapCandidates(zone) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\t%s\n",
					candidate.Tag, len(candidate.Topics), strings.Join(candidate.Topics, ","))
			}
			return nil
		},
	}
}

func researchPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Путь зоны research",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := research.Root()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), root)
			return nil
		},
	}
}
