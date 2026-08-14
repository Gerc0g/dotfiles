package cli

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/secret"
	"github.com/spf13/cobra"
)

func newSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Секреты через 1Password со скоупами",
		Long: "Айтемы живут в vault компании Work-<co> и именуются по скоупу:\n" +
			"_company__VAR, <product>__VAR, <product>__<repo>__VAR. Скоуп по\n" +
			"умолчанию выводится из каталога. Сгенерированные .envrc грузят\n" +
			"значения через secret-cache, чтобы direnv не дёргал Touch ID.\n\n" +
			"`secret signin` остаётся в оболочке — он мутирует окружение сессии.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	cmd.AddCommand(
		secretAddCmd(), secretEditCmd(), secretNameCmd(), secretEnvlineCmd(),
		secretListCmd(), secretGetCmd(), secretCacheCmd(),
	)
	return cmd
}

// secretScopeFlags parses the shared --company/--product/--repo selector.
type secretScopeFlags struct {
	company bool
	product bool
	repo    bool
}

func (f *secretScopeFlags) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.company, "company", false, "company-скоуп")
	cmd.Flags().BoolVar(&f.product, "product", false, "product-скоуп")
	cmd.Flags().BoolVar(&f.repo, "repo", false, "repo-скоуп")
}

func (f *secretScopeFlags) scope() secret.Scope {
	switch {
	case f.company:
		return secret.ScopeCompany
	case f.product:
		return secret.ScopeProduct
	case f.repo:
		return secret.ScopeRepo
	default:
		return secret.ScopeAuto
	}
}

// resolveSecret builds the context and item name for one var.
func resolveSecret(scope secret.Scope, varArg, explicitCompany string) (secret.Ctx, secret.Scope, string, error) {
	root, err := worldRoot()
	if err != nil {
		return secret.Ctx{}, scope, "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return secret.Ctx{}, scope, "", fmt.Errorf("текущий каталог: %w", err)
	}

	ctx, err := secret.Resolve(root, cwd, explicitCompany)
	if err != nil {
		return secret.Ctx{}, scope, "", err
	}
	if scope == secret.ScopeAuto {
		scope = ctx.AutoScope()
	}
	if err := ctx.Require(scope); err != nil {
		return secret.Ctx{}, scope, "", err
	}
	return ctx, scope, ctx.ItemName(scope, secret.VarName(varArg)), nil
}

func secretAddCmd() *cobra.Command {
	var flags secretScopeFlags
	cmd := &cobra.Command{
		Use:   "add <VAR> <VALUE> [company]",
		Short: "Создать секрет и прописать его в .envrc",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretUpsert(cmd, flags.scope(), args, false)
		},
	}
	flags.register(cmd)
	return cmd
}

func secretEditCmd() *cobra.Command {
	var flags secretScopeFlags
	cmd := &cobra.Command{
		Use:   "edit <VAR> <VALUE> [company]",
		Short: "Обновить секрет и убедиться в .envrc",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretUpsert(cmd, flags.scope(), args, true)
		},
	}
	flags.register(cmd)
	return cmd
}

func runSecretUpsert(cmd *cobra.Command, scope secret.Scope, args []string, edit bool) error {
	explicit := ""
	if len(args) == 3 {
		explicit = args[2]
	}
	ctx, scope, item, err := resolveSecret(scope, args[0], explicit)
	if err != nil {
		return err
	}
	varname := secret.VarName(args[0])
	vault := ctx.Vault()
	value := args[1]

	r := ui.New(cmd.OutOrStdout())
	if edit {
		if err := secret.EditItem(vault, item, value); err != nil {
			return fmt.Errorf("item %q не обновился в %q: %w", item, vault, err)
		}
		r.Line(r.OK("✓ ") + fmt.Sprintf("обновлён op://%s/%s", vault, item))
	} else {
		secret.EnsureVault(vault)
		if secret.ItemExists(vault, item) {
			return fmt.Errorf("item %q уже в %q — используй: hq secret edit --%s %s <value>",
				item, vault, scope, varname)
		}
		if err := secret.CreateItem(vault, item, value); err != nil {
			return err
		}
		r.Line(r.OK("✓ ") + fmt.Sprintf("создан op://%s/%s", vault, item))
	}

	added, envrc, err := ctx.EnsureEnvrc(scope, varname, vault, item)
	if err != nil {
		return err
	}
	if added {
		r.Line(r.OK("✓ ") + "добавлено в " + envrc)
		r.Line(r.Muted("  активировать: direnv allow " + envrc))
	} else {
		r.Line(r.Muted("→ " + envrc + " уже содержит export " + varname + "="))
	}
	return nil
}

func secretNameCmd() *cobra.Command {
	var flags secretScopeFlags
	cmd := &cobra.Command{
		Use:   "name <VAR> [company]",
		Short: "Показать имя айтема для переменной",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			explicit := ""
			if len(args) == 2 {
				explicit = args[1]
			}
			_, _, item, err := resolveSecret(flags.scope(), args[0], explicit)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), item)
			return nil
		},
	}
	flags.register(cmd)
	return cmd
}

func secretEnvlineCmd() *cobra.Command {
	var flags secretScopeFlags
	cmd := &cobra.Command{
		Use:   "envline <VAR> [company]",
		Short: "Показать export-строку для .envrc",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			explicit := ""
			if len(args) == 2 {
				explicit = args[1]
			}
			ctx, _, item, err := resolveSecret(flags.scope(), args[0], explicit)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), secret.EnvLine(secret.VarName(args[0]), ctx.Vault(), item))
			return nil
		},
	}
	flags.register(cmd)
	return cmd
}

func secretListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <vault>",
		Short: "Список айтемов vault (op item list)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := secret.OpPassthrough("item", "list", "--vault", args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func secretGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <op://path>",
		Short: "Прочитать значение (op read)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := secret.OpPassthrough("read", args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func secretCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Локальный TTL-кэш значений 1Password",
		Long: "Значения кэшируются в ~/.cache/dotfiles/secrets с TTL (30 дней по\n" +
			"умолчанию), чтобы direnv не дёргал Touch ID на каждый cd. Ключи и\n" +
			"раскладка совместимы с прежним secret-cache.sh.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}

	get := &cobra.Command{
		Use:   "get <vault> <item> [field] [ttl_days]",
		Short: "Значение из кэша, при протухании — из 1Password",
		Args:  cobra.RangeArgs(2, 4),
		RunE:  func(cmd *cobra.Command, args []string) error { return runCacheGet(cmd, args, false) },
	}
	refresh := &cobra.Command{
		Use:   "refresh <vault> <item> [field]",
		Short: "Принудительно перечитать из 1Password",
		Args:  cobra.RangeArgs(2, 3),
		RunE:  func(cmd *cobra.Command, args []string) error { return runCacheGet(cmd, args, true) },
	}
	clear := &cobra.Command{
		Use:   "clear [vault item [field]]",
		Short: "Очистить кэш целиком или одну запись",
		Args:  cobra.MaximumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return fmt.Errorf("нужно: clear (всё) или clear <vault> <item> [field]")
			}
			c, err := secret.NewCache()
			if err != nil {
				return err
			}
			vault, item, field := "", "", ""
			if len(args) >= 2 {
				vault, item = args[0], args[1]
			}
			if len(args) == 3 {
				field = args[2]
			}
			return c.Clear(vault, item, field)
		},
	}
	list := &cobra.Command{
		Use:   "list",
		Short: "Показать закэшированные записи",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := secret.NewCache()
			if err != nil {
				return err
			}
			lines, err := c.List()
			if err != nil {
				return err
			}
			for _, line := range lines {
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}

	cmd.AddCommand(get, refresh, clear, list)
	return cmd
}

func runCacheGet(cmd *cobra.Command, args []string, bypass bool) error {
	c, err := secret.NewCache()
	if err != nil {
		return err
	}
	if bypass {
		c.Bypass = true
	}

	field := ""
	if len(args) >= 3 {
		field = args[2]
	}
	var ttl time.Duration
	if len(args) == 4 {
		days, err := strconv.Atoi(args[3])
		if err != nil {
			return fmt.Errorf("ttl_days должен быть числом: %q", args[3])
		}
		ttl = time.Duration(days) * 24 * time.Hour
	}

	value, err := c.Get(args[0], args[1], field, ttl)
	if err != nil {
		return err
	}
	// The value is the output contract: no trailing newline, nothing else.
	fmt.Fprint(cmd.OutOrStdout(), value)
	return nil
}
