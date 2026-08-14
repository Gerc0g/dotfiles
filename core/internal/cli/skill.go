package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/skill"
	"github.com/spf13/cobra"
)

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Repo-owned скиллы агентов",
		Long: "Скиллы живут в ~/dotfiles/skills и симлинкуются в оба профиля\n" +
			"(~/.claude/skills, ~/.codex/skills). Каждый скилл ставится в оба —\n" +
			"раздельные профили были только способом где-то его забыть.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	cmd.AddCommand(skillListCmd(), skillNewCmd(), skillInstallCmd(), skillDoctorCmd())
	return cmd
}

func skillPaths() (skill.Paths, error) {
	return skill.DefaultPaths()
}

func skillListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Скиллы и статус установки",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := skillPaths()
			if err != nil {
				return err
			}
			infos, err := skill.List(p)
			if err != nil {
				return err
			}

			r := ui.New(cmd.OutOrStdout())
			if len(infos) == 0 {
				r.Line("скиллов нет")
				return nil
			}
			for _, info := range infos {
				var states []string
				for _, target := range p.Targets {
					states = append(states, renderLinkStatus(r, target, info.Status[target]))
				}
				r.Line(fmt.Sprintf("%-22s %s", info.Name, strings.Join(states, "  ")))
			}
			return nil
		},
	}
}

func renderLinkStatus(r *ui.Renderer, target string, status skill.LinkStatus) string {
	label := profileLabel(target)
	if status == skill.StatusLinked {
		return label + "=" + r.OK(string(status))
	}
	return label + "=" + r.Drifted(string(status))
}

// profileLabel shortens ~/.claude/skills to claude for the list view.
func profileLabel(target string) string {
	parts := strings.Split(target, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") && len(part) > 1 {
			return strings.TrimPrefix(part, ".")
		}
	}
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return target
}

func skillNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <имя>",
		Short: "Создать шаблон скилла",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := skillPaths()
			if err != nil {
				return err
			}
			file, err := skill.New(p, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "создан %s\n", file)
			return nil
		},
	}
}

func skillInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Симлинкнуть все скиллы в оба профиля",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := skillPaths()
			if err != nil {
				return err
			}
			linked, err := skill.Install(p)
			if err != nil {
				return err
			}
			for _, name := range linked {
				fmt.Fprintf(cmd.OutOrStdout(), "прилинкован %s\n", name)
			}
			return nil
		},
	}
}

func skillDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Проверить frontmatter, ссылки и сироты",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := skillPaths()
			if err != nil {
				return err
			}
			problems, err := skill.Doctor(p)
			if err != nil {
				return err
			}

			r := ui.New(cmd.OutOrStdout())
			if len(problems) == 0 {
				r.Line(r.OK("skill doctor: ok"))
				return nil
			}
			for _, problem := range problems {
				r.Line(r.Drifted("✗ ") + problem)
			}
			return errors.New(ui.Plural(len(problems), "проблема", "проблемы", "проблем"))
		},
	}
}
