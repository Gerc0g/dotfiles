package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/user"
	"strconv"

	"github.com/Gerc0g/dotfiles/core/serversetup"
	"github.com/Gerc0g/dotfiles/core/setup"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var dryRun bool
	var mode, bundle, privateURL, daemonUser string

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Привести машину к заявленному состоянию",
		Long: "Создаёт каталоги, симлинки и фоновые задачи, которые нужны платформе.\n\n" +
			"Запускать можно сколько угодно раз: каждый шаг сначала проверяет и\n" +
			"действует только при расхождении. Ничего не удаляется — обычный файл\n" +
			"на месте симлинка отодвигается в копию с меткой времени, а ссылки,\n" +
			"созданные не этой командой, показываются, но не трогаются.\n\n" +
			"Установка пакетов (Homebrew, npm, CLI агентов) осталась в bootstrap.sh:\n" +
			"тот запускается один раз на чистой машине, а этот — каждый раз, когда\n" +
			"состояние разошлось.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if mode != "workstation" && mode != "server" {
				return errors.New("--mode must be workstation or server")
			}
			if mode == "server" {
				if bundle == "" || privateURL == "" {
					return errors.New("server setup requires --bundle and --private-url")
				}
				account, err := user.Lookup(daemonUser)
				if err != nil {
					return fmt.Errorf("daemon user %q must already exist: %w", daemonUser, err)
				}
				uid, err := strconv.Atoi(account.Uid)
				if err != nil {
					return err
				}
				gid, err := strconv.Atoi(account.Gid)
				if err != nil {
					return err
				}
				report, err := serversetup.Run(serversetup.Options{Bundle: bundle, PrivateURL: privateURL, User: daemonUser, UID: uid, GID: gid}, dryRun)
				if err != nil {
					return err
				}
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(report)
			}
			if bundle != "" || privateURL != "" || cmd.Flags().Changed("user") {
				return errors.New("--bundle, --private-url and --user require --mode server")
			}
			env, err := newEnv()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()

			if dryRun {
				reports := setup.Check(env, setup.Plan())
				renderReports(out, reports)
				renderSummary(out, reports)
				if hasApplicableWork(reports) {
					fmt.Fprintln(out, "  запустите без --dry-run, чтобы применить")
				} else if needsAttention(reports) {
					fmt.Fprintln(out, "  автоматически здесь ничего не применить")
				}
				return nil
			}

			reports, errs := setup.Apply(env, setup.Plan())
			renderReports(out, reports)
			renderSummary(out, reports)

			if len(errs) > 0 {
				for _, err := range errs {
					fmt.Fprintf(cmd.ErrOrStderr(), "ошибка: %v\n", err)
				}
				return fmt.Errorf("шагов с ошибкой: %d", len(errs))
			}

			if needsAttention(reports) {
				return errors.New("часть шагов требует ручного вмешательства (см. выше)")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "показать изменения, ничего не применять")
	cmd.Flags().StringVar(&mode, "mode", "workstation", "режим: workstation или server")
	cmd.Flags().StringVar(&bundle, "bundle", "", "каталог проверенного серверного релиза с manifest.json")
	cmd.Flags().StringVar(&privateURL, "private-url", "", "существующий приватный HTTPS-адрес сервера")
	cmd.Flags().StringVar(&daemonUser, "user", "agent", "существующий непривилегированный пользователь сервера")

	return cmd
}
