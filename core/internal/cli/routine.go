package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/routine"
	"github.com/spf13/cobra"
)

func newRoutineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routine [command]",
		Short: "Фоновые задачи: реестр, запуск, журнал",
		Long: "Всё, что работает на этой машине без вас: расписание, последний исход,\n" +
			"и во что это обходится. Часть задач запускает агента, то есть тратит\n" +
			"подписку — забытая задача жжёт деньги молча, поэтому у агентских\n" +
			"routine всегда есть потолок запусков за сутки.\n\n" +
			"Свои задачи описываются в ~/.config/hq/routines.yaml: имя, расписание\n" +
			"и промпт. Расписание держит launchd, плисты генерируются из реестра.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return listRoutines(cmd) },
	}
	cmd.AddCommand(routineRunCmd(), routineLogsCmd(), routineToggleCmd(true), routineToggleCmd(false))
	return cmd
}

func routinePaths() (home, dotfiles string, err error) {
	home, err = os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	return home, filepath.Join(home, "dotfiles"), nil
}

func listRoutines(cmd *cobra.Command) error {
	home, dotfiles, err := routinePaths()
	if err != nil {
		return err
	}
	all, loadErr := routine.Load(home, dotfiles)
	state := routine.LoadState(home)
	render := ui.New(cmd.OutOrStdout())

	render.Line(render.Bold("Фоновые задачи"))
	render.Blank()
	for _, r := range all {
		status := state[r.Name]

		mark := render.OK("●")
		switch {
		case status.Disabled:
			mark = render.Muted("○")
		case status.Outcome == routine.OutcomeFailed:
			mark = render.Drifted("●")
		case status.Last.IsZero():
			mark = render.Missing("●")
		}

		cost := ""
		if r.Kind.SpendsTokens() {
			cost = render.Drifted(fmt.Sprintf("   тратит токены, %d/%d за сутки",
				status.RunsToday(time.Now()), r.Ceiling()))
		}

		render.Line(fmt.Sprintf("%s %s  %s%s", mark, render.Accent(r.Name),
			render.Muted(r.Schedule.String()), cost))
		render.Line("  " + r.About)
		render.Line("  " + render.Muted(lastRunLine(status)))
		if r.Source != "" {
			render.Line("  " + render.Muted("объявлена в "+shortHome(home, r.Source)))
		}
		render.Blank()
	}

	if loadErr != nil {
		render.Line(render.Drifted("Конфиг пользовательских routine не прочитан:"))
		render.Line(fmt.Sprintf("  %v", loadErr))
		return nil
	}
	render.Line(render.Muted("журнал: hq routine logs · запуск: hq routine run <имя>"))
	return nil
}

func lastRunLine(status routine.Status) string {
	if status.Last.IsZero() {
		return "ещё не запускалась"
	}
	line := fmt.Sprintf("последний запуск %s, %s",
		status.Last.Format("02.01 15:04"), status.Outcome)
	if status.DurationMS > 0 {
		line += fmt.Sprintf(" за %s", (time.Duration(status.DurationMS) * time.Millisecond).Round(time.Second))
	}
	if status.Detail != "" {
		line += " — " + status.Detail
	}
	return line
}

func routineRunCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "run <имя>",
		Short: "Запустить задачу сейчас",
		Long: "Проверяет гварды и выполняет задачу, записывая исход в общий журнал.\n" +
			"Это же вызывает launchd по расписанию — ручной запуск ничем не особенный.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, dotfiles, err := routinePaths()
			if err != nil {
				return err
			}
			r, err := routine.Find(home, dotfiles, args[0])
			if err != nil {
				return err
			}
			return routine.Run(home, r, force, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "игнорировать гварды интервала и суточного потолка")
	return cmd
}

func routineLogsCmd() *cobra.Command {
	var lines int
	cmd := &cobra.Command{
		Use:   "logs [имя]",
		Short: "Общий журнал фоновых задач",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _, err := routinePaths()
			if err != nil {
				return err
			}
			var source string
			if len(args) == 1 {
				source = args[0]
			}
			entries := routine.Tail(home, source, lines)
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "журнал пуст")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(entries, "\n"))
			return nil
		},
	}
	cmd.Flags().IntVarP(&lines, "lines", "n", 40, "сколько строк показать")
	return cmd
}

func routineToggleCmd(enable bool) *cobra.Command {
	verb, short := "disable", "Выключить задачу, не удаляя её историю"
	if enable {
		verb, short = "enable", "Включить выключенную задачу"
	}
	return &cobra.Command{
		Use:   verb + " <имя>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, dotfiles, err := routinePaths()
			if err != nil {
				return err
			}
			r, err := routine.Find(home, dotfiles, args[0])
			if err != nil {
				return err
			}
			if err := routine.SetDisabled(home, r.Name, !enable); err != nil {
				return err
			}
			if enable {
				fmt.Fprintf(cmd.OutOrStdout(), "%s включена\n", r.Name)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s выключена — расписание остаётся, но запуск пропускается\n", r.Name)
			}
			return nil
		},
	}
}

func shortHome(home, path string) string {
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
