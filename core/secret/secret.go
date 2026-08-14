// Package secret manages scoped 1Password items and their .envrc wiring.
//
// Naming is the contract: company items are `_company__VAR`, product items
// `<product>__VAR`, repo items `<product>__<repo>__VAR`, all in the company
// vault `Work-<company>`. The generated .envrc lines load values through the
// secret cache, so direnv does not hit Touch ID on every cd.
package secret

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/Gerc0g/dotfiles/core/world"
)

// Scope is where a secret belongs.
type Scope string

const (
	ScopeAuto    Scope = "auto"
	ScopeCompany Scope = "company"
	ScopeProduct Scope = "product"
	ScopeRepo    Scope = "repo"
)

// Ctx is the resolved secret context: slugs for naming, dirs for .envrc.
type Ctx struct {
	Company    string
	Product    string
	Repo       string
	CompanyDir string
	ProductDir string
	RepoDir    string
}

// Resolve builds the secret context for a directory. An explicit company
// overrides the walk entirely — items then land at company scope regardless
// of cwd. Slugs honour the config `slug` overrides, falling back to the
// directory name.
func Resolve(root, dir, explicitCompany string) (Ctx, error) {
	tree, err := world.Scan(root)
	if err != nil {
		return Ctx{}, err
	}

	if explicitCompany != "" {
		for _, c := range tree.Companies() {
			if c.Slug == explicitCompany {
				return Ctx{Company: companySlug(c), CompanyDir: c.Path}, nil
			}
		}
		return Ctx{}, fmt.Errorf("нет компании %q в %s", explicitCompany, root)
	}

	loc, err := workspace.Locate(root, dir)
	if err != nil {
		return Ctx{}, err
	}

	ctx := Ctx{}
	for _, c := range tree.Companies() {
		if c.Slug == loc.Company {
			ctx.Company = companySlug(c)
			ctx.CompanyDir = c.Path
		}
	}
	if ctx.Company == "" {
		return Ctx{}, fmt.Errorf("нет компании %q в %s", loc.Company, root)
	}

	if loc.Product != "" {
		ctx.Product = loc.Product
		ctx.ProductDir = filepath.Join(ctx.CompanyDir, loc.Product)
		for _, p := range tree.Products(loc.Company) {
			if p.Slug == loc.Product {
				if slug := p.Config.Get("slug"); slug != "" {
					ctx.Product = slug
				}
				ctx.ProductDir = p.Path
			}
		}
	}

	if loc.Repo != "" {
		ctx.Repo = loc.Repo
		// Secrets belong to the repo, not to one worktree of it: .envrc goes
		// into the main checkout even when resolved from a worktree.
		ctx.RepoDir = filepath.Join(ctx.ProductDir, loc.Repo)
	}

	return ctx, nil
}

func companySlug(c world.Company) string {
	if slug := c.Config.Get("slug"); slug != "" {
		return slug
	}
	return c.Slug
}

// AutoScope picks the narrowest scope the context supports.
func (c Ctx) AutoScope() Scope {
	switch {
	case c.Repo != "":
		return ScopeRepo
	case c.Product != "":
		return ScopeProduct
	default:
		return ScopeCompany
	}
}

// Require validates that the context carries what the scope needs.
func (c Ctx) Require(scope Scope) error {
	if c.Company == "" {
		return fmt.Errorf("company не определена: зайди в ~/Desktop/Prokectfiles/<co>/... или передай company последним аргументом")
	}
	switch scope {
	case ScopeProduct:
		if c.Product == "" {
			return fmt.Errorf("product scope требует cwd внутри product или repo")
		}
	case ScopeRepo:
		if c.Product == "" || c.Repo == "" {
			return fmt.Errorf("repo scope требует cwd внутри repo или managed worktree")
		}
	}
	return nil
}

// Vault is the company vault name.
func (c Ctx) Vault() string { return "Work-" + c.Company }

var slugUnsafe = regexp.MustCompile(`[^a-z0-9._-]+`)

// slugPart normalises a name the way item names always were built.
func slugPart(s string) string {
	return slugUnsafe.ReplaceAllString(strings.ToLower(s), "_")
}

// VarName normalises the variable name to uppercase.
func VarName(s string) string { return strings.ToUpper(s) }

// ItemName renders the scoped 1Password item title.
func (c Ctx) ItemName(scope Scope, varname string) string {
	switch scope {
	case ScopeProduct:
		return fmt.Sprintf("%s__%s", slugPart(c.Product), varname)
	case ScopeRepo:
		return fmt.Sprintf("%s__%s__%s", slugPart(c.Product), slugPart(c.Repo), varname)
	default:
		return fmt.Sprintf("_company__%s", varname)
	}
}

// EnvrcPath is where the scope's .envrc lives.
func (c Ctx) EnvrcPath(scope Scope) string {
	switch scope {
	case ScopeProduct:
		return filepath.Join(c.ProductDir, ".envrc")
	case ScopeRepo:
		return filepath.Join(c.RepoDir, ".envrc")
	default:
		return filepath.Join(c.CompanyDir, ".envrc")
	}
}

// EnvLine renders the export line the .envrc carries. The path is frozen into
// .envrc files across all repos, so it stays the bash shim — which execs the
// core — rather than a direct hq invocation direnv might not find on PATH.
func EnvLine(varname, vault, item string) string {
	return fmt.Sprintf(`export %s="$(bash "$HOME/dotfiles/scripts/secret-cache.sh" get %s %s credential)"`,
		varname, vault, item)
}

// EnsureEnvrc wires one secret into the scope's .envrc: creates the file,
// prepends source_up below company level, appends the export idempotently.
// It returns whether the line was added.
func (c Ctx) EnsureEnvrc(scope Scope, varname, vault, item string) (added bool, envrc string, err error) {
	envrc = c.EnvrcPath(scope)
	dir := filepath.Dir(envrc)
	if _, err := os.Stat(dir); err != nil {
		return false, envrc, fmt.Errorf("каталога для .envrc нет: %s", dir)
	}

	data, err := os.ReadFile(envrc)
	if err != nil && !os.IsNotExist(err) {
		return false, envrc, fmt.Errorf("чтение %s: %w", envrc, err)
	}
	content := string(data)

	if scope != ScopeCompany && !hasLine(content, "source_up") {
		content = "source_up\n\n" + content
	}

	exportRe := regexp.MustCompile(`(?m)^export ` + regexp.QuoteMeta(varname) + `=`)
	if exportRe.MatchString(content) {
		if err := writeFilePreserving(envrc, content, data); err != nil {
			return false, envrc, err
		}
		return false, envrc, nil
	}

	content += fmt.Sprintf("\n# Loaded from 1Password item: op://%s/%s/credential\n%s\n",
		vault, item, EnvLine(varname, vault, item))
	if err := os.WriteFile(envrc, []byte(content), 0o644); err != nil {
		return false, envrc, fmt.Errorf("запись %s: %w", envrc, err)
	}
	return true, envrc, nil
}

// writeFilePreserving writes only when the content actually changed (the
// source_up prepend may have happened even when the export already exists).
func writeFilePreserving(path, content string, original []byte) error {
	if content == string(original) {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("запись %s: %w", path, err)
	}
	return nil
}

func hasLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if l == line {
			return true
		}
	}
	return false
}
