package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	"github.com/Gerc0g/dotfiles/core/devstack"
	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/spf13/cobra"
)

func newDevstackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devstack",
		Short: "Общая локальная dev-инфраструктура",
		Long: "Один docker-compose стек (Postgres, Redis, Qdrant, ClickHouse, MinIO,\n" +
			"observability, Langfuse, Ollama, …) для всех проектов. Проекты\n" +
			"подключаются управляемым блоком в .envrc, который резолвит адреса из\n" +
			"DEV_STACK_HOST на каждый cd.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	cmd.AddCommand(
		dsURLsCmd(), dsEnvrcCmd(), dsConnectCmd(), dsDoctorCmd(),
		dsComposeCmd("up", "Поднять весь стек или названные сервисы", "start", "-d"),
		dsComposeCmd("down", "Остановить (данные сохраняются)", "stop"),
		dsComposeCmd("restart", "Перезапустить сервисы", ""),
		dsComposeCmd("pull", "Обновить образы", ""),
		dsComposeCmd("logs", "Логи (-f)", "", "-f"),
		dsComposeCmd("ps", "Состояние сервисов", "status"),
		dsInteractiveCmd("psql", "psql в dev-postgres", "dev-postgres"),
		dsInteractiveCmd("redis-cli", "redis-cli в dev-redis", "dev-redis"),
		dsInteractiveCmd("ch", "clickhouse-client в dev-clickhouse", "dev-clickhouse"),
		dsDBEnsureCmd(), dsDBCreateCmd(), dsDBDropCmd(), dsNukeCmd(),
	)
	return cmd
}

func requireDocker() error {
	if !devstack.DockerRunning() {
		return fmt.Errorf("docker не запущен (DEV_STACK_HOST=%s) — запусти OrbStack", devstack.Host())
	}
	return nil
}

// devstackCtx resolves product/repo/db from args or cwd.
func devstackCtx(args []string) (devstack.Ctx, error) {
	root, err := worldRoot()
	if err != nil {
		return devstack.Ctx{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return devstack.Ctx{}, fmt.Errorf("текущий каталог: %w", err)
	}
	product, repo := "", ""
	if len(args) == 2 {
		product, repo = args[0], args[1]
	}
	return devstack.ResolveCtx(root, cwd, product, repo)
}

func dsURLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "urls",
		Short: "Адреса всех сервисов (учитывает DEV_STACK_HOST)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := devstack.Host()
			fmt.Fprintf(cmd.OutOrStdout(), `Dev stack @ %[1]s

  core
    Postgres      %[1]s:5432            user=dev pass=dev
    Redis         %[1]s:6379
  storage / vectors
    MinIO  S3     http://%[1]s:9000     key=dev secret=devsecret123
    MinIO  UI     http://%[1]s:9001
    Qdrant        http://%[1]s:6333     (gRPC %[1]s:6334)
    ClickHouse    http://%[1]s:8123     user=dev pass=dev  (HTTP only; native :9000 internal)
  observability   (push telemetry via OTLP — Prometheus does NOT scrape your app)
    OTel ingest   %[1]s:4317 (grpc) / %[1]s:4318 (http)
    Prometheus    http://%[1]s:9090
    Grafana       http://%[1]s:3030     user=dev pass=dev  (Prom/Loki/Tempo pre-wired)
    Loki          http://%[1]s:3100
    Tempo         http://%[1]s:3200
  llm / ml
    Langfuse      http://%[1]s:3001
    Ollama        http://%[1]s:11434
  bi / viewer
    Metabase      http://%[1]s:3002
    CloudBeaver   http://%[1]s:8978
  hub / proxy
    Homepage      http://%[1]s/          (landing; Traefik name-routing *.%[1]s.sslip.io)
    Traefik dash  http://%[1]s:8080

Connect a project:  cd <repo> && dev-stack connect
`, h)
			return nil
		},
	}
}

func dsEnvrcCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "envrc [product repo]",
		Short: "Показать .envrc-блок без записи",
		Args:  cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := devstackCtx(args)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), devstack.EnvrcBlock(ctx.DB))
			return nil
		},
	}
}

func dsConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect [product repo]",
		Short: "Подключить проект: .envrc-блок + БД + direnv allow",
		Args:  cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := devstackCtx(args)
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			r := ui.New(cmd.OutOrStdout())
			added, envrc, err := devstack.EnsureEnvrcBlock(cwd, ctx.DB)
			if err != nil {
				return err
			}
			if added {
				r.Line(r.OK("✓ ") + "dev-stack блок добавлен в " + envrc)
			} else {
				r.Line(r.OK("✓ ") + ".envrc уже содержит dev-stack блок")
			}
			r.Line(r.Muted(fmt.Sprintf("  product=%s repo=%s db=%s host=$%s (%s)",
				ctx.Product, ctx.Repo, ctx.DB, devstack.HostEnv, devstack.Host())))

			if err := requireDocker(); err != nil {
				return err
			}
			if err := devstack.EnsureDB(ctx.DB, cmd.OutOrStdout()); err != nil {
				return err
			}

			if _, err := exec.LookPath("direnv"); err == nil {
				if exec.Command("direnv", "allow", cwd).Run() == nil {
					r.Line(r.OK("✓ ") + "direnv allow")
				}
			}
			r.Line(r.Muted("→ проверить: dev-stack doctor"))
			return nil
		},
	}
}

func dsDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Достижимость стека и подключён ли текущий проект",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := ui.New(cmd.OutOrStdout())
			host := devstack.Host()
			r.Line(fmt.Sprintf("dev-stack doctor @ %s", host))

			r.Line("доступность:")
			for _, res := range devstack.ProbeHost(host) {
				if res.Reachable {
					r.Line("  " + r.OK("✓") + fmt.Sprintf(" %s (%d)", res.Name, res.Port))
				} else {
					r.Line("  " + r.Missing("✗") + fmt.Sprintf(" %s (%d) НЕДОСТУПЕН", res.Name, res.Port))
				}
			}

			r.Line("проект:")
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if devstack.HasEnvrcBlock(cwd) {
				r.Line("  " + r.OK("✓") + " ./.envrc содержит dev-stack блок")
			} else {
				r.Line("  " + r.Missing("✗") + " ./.envrc не подключён — dev-stack connect")
			}

			if ctx, err := devstackCtx(nil); err == nil && devstack.DockerRunning() {
				if devstack.DBExists(ctx.DB) {
					r.Line("  " + r.OK("✓") + fmt.Sprintf(" БД %q существует", ctx.DB))
				} else {
					r.Line("  " + r.Missing("✗") + fmt.Sprintf(" БД %q нет — dev-stack db-ensure %s", ctx.DB, ctx.DB))
				}
			}
			return nil
		},
	}
}

// dsComposeCmd wraps one docker compose lifecycle verb.
func dsComposeCmd(verb, short, alias string, extra ...string) *cobra.Command {
	var aliases []string
	if alias != "" {
		aliases = []string{alias}
	}
	return &cobra.Command{
		Use:     verb + " [сервис...]",
		Aliases: aliases,
		Short:   short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireDocker(); err != nil {
				return err
			}
			return devstack.Compose(append(append([]string{verb}, extra...), args...)...)
		},
	}
}

// dsInteractiveCmd execs an interactive client inside a stack container.
func dsInteractiveCmd(name, short, container string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireDocker(); err != nil {
				return err
			}
			switch name {
			case "psql":
				db := "postgres"
				if len(args) == 1 {
					db = args[0]
				}
				return devstack.Interactive(container, "psql", "-U", "dev", db)
			case "redis-cli":
				return devstack.Interactive(container, "redis-cli")
			default:
				return devstack.Interactive(container, "clickhouse-client", "-u", "dev", "--password", "dev")
			}
		},
	}
}

func dsDBEnsureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "db-ensure <имя>",
		Short: "Идемпотентный CREATE DATABASE",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireDocker(); err != nil {
				return err
			}
			return devstack.EnsureDB(args[0], cmd.OutOrStdout())
		},
	}
}

func dsDBCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "db-create <имя>",
		Short: "CREATE DATABASE (падает, если есть)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireDocker(); err != nil {
				return err
			}
			if err := devstack.CreateDB(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ БД %q создана\n", args[0])
			return nil
		},
	}
}

func dsDBDropCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "db-drop <имя>",
		Short: "DROP DATABASE с подтверждением",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireDocker(); err != nil {
				return err
			}
			if !confirm(cmd, fmt.Sprintf("⚠ Удалить базу %q? (y/N) ", args[0])) {
				return nil
			}
			if err := devstack.DropDB(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ БД %q удалена\n", args[0])
			return nil
		},
	}
}

func dsNukeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "nuke",
		Short: "Снести стек вместе с данными",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireDocker(); err != nil {
				return err
			}
			if !confirm(cmd, fmt.Sprintf("⚠ Это снесёт все данные dev-stack @ %s. Продолжить? (y/N) ", devstack.Host())) {
				return nil
			}
			if err := devstack.Compose("down", "-v"); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Снесено")
			return nil
		},
	}
}

// confirm asks a y/N question on the command's stdin.
func confirm(cmd *cobra.Command, prompt string) bool {
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		return false
	}
	return scanner.Text() == "y"
}
