package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
	"github.com/Gerc0g/dotfiles/core/knowledge"
	"github.com/Gerc0g/dotfiles/core/runner"
	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/Gerc0g/dotfiles/core/world"
	"github.com/spf13/cobra"
)

type lsOptions struct {
	flat  bool
	paths bool
	json  bool
}

func newLsCmd() *cobra.Command {
	var opts lsOptions

	cmd := &cobra.Command{
		Use:   "ls [компания [продукт]]",
		Short: "Показать компании, продукты и репозитории",
		Long: "Обходит рабочий корень и показывает то, что реально лежит на диске.\n\n" +
			"Каталог считается компанией, если в нём есть .company-config, продуктом —\n" +
			"если .product-config, репозиторием — если .git. Всё остальное\n" +
			"пропускается: временные папки и общий пул .worktrees сюда не попадают.\n\n" +
			"Корень берётся из PROKECTFILES_ROOT, иначе ~/Desktop/Prokectfiles.",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLs(cmd, args, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.flat, "flat", false, "по строке на репозиторий, без дерева")
	cmd.Flags().BoolVar(&opts.paths, "paths", false, "абсолютные пути вместо имён")
	cmd.Flags().BoolVar(&opts.json, "json", false, "JSON-каталог компаний, продуктов, репозиториев и управляемых worktree")
	cmd.MarkFlagsMutuallyExclusive("json", "flat")
	cmd.MarkFlagsMutuallyExclusive("json", "paths")

	return cmd
}

func runLs(cmd *cobra.Command, args []string, opts lsOptions) error {
	tree, err := world.ScanDefault()
	if err != nil {
		return err
	}

	company, product := argAt(args, 0), argAt(args, 1)
	if company != "" && !tree.HasCompany(company) {
		return fmt.Errorf("нет компании %q в %s", company, tree.RootPath())
	}
	if product != "" && !tree.HasProduct(company, product) {
		return fmt.Errorf("нет продукта %q в компании %q", product, company)
	}

	for _, warning := range tree.Warnings() {
		fmt.Fprintf(cmd.ErrOrStderr(), "предупреждение: %s\n", warning)
	}

	out := cmd.OutOrStdout()

	// Machine modes stay unstyled: these exist to be piped.
	if opts.json {
		return printCatalog(out, tree, company, product)
	}
	if opts.flat || opts.paths {
		printFlat(out, tree, company, product, opts.paths)
		return nil
	}

	printTree(out, tree, company, product)
	return nil
}

// Catalog DTOs keep the versioned CLI contract separate from domain structs,
// including private config values that must never leak into the JSON output.
type lsCatalog struct {
	Version   int                `json:"version"`
	Root      string             `json:"root"`
	Companies []lsCatalogCompany `json:"companies"`
	Research  *lsCatalogResearch `json:"research,omitempty"`
	Ordinary  *lsCatalogResearch `json:"ordinary,omitempty"`
}

type lsCatalogResearch struct {
	Path string `json:"path"`
}

type lsCatalogCompany struct {
	Slug     string             `json:"slug"`
	Path     string             `json:"path"`
	Products []lsCatalogProduct `json:"products"`
}

type lsCatalogProduct struct {
	Slug  string          `json:"slug"`
	Path  string          `json:"path"`
	Repos []lsCatalogRepo `json:"repos"`
}

type lsCatalogRepo struct {
	Name      string               `json:"name"`
	Path      string               `json:"path"`
	Worktrees []lsCatalogWorkspace `json:"worktrees"`
}

type lsCatalogWorkspace struct {
	ID                string          `json:"id"`
	Path              string          `json:"path"`
	Branch            string          `json:"branch"`
	State             workspace.State `json:"state"`
	Task              string          `json:"task"`
	CreationRequestID string          `json:"creationRequestId,omitempty"`
}

func printCatalog(out io.Writer, tree *world.Tree, company, product string) error {
	manager, err := workspace.New(tree.RootPath())
	if err != nil {
		return err
	}
	worktrees, err := manager.List()
	if err != nil {
		return err
	}
	byRepo := make(map[string][]lsCatalogWorkspace)
	for _, w := range worktrees {
		ref := (world.Repo{Company: w.Company, Product: w.Product, Slug: w.Repo}).Ref()
		byRepo[ref] = append(byRepo[ref], lsCatalogWorkspace{
			ID: w.ID, Path: w.Path, Branch: w.Branch, State: w.State, Task: w.Task, CreationRequestID: w.CreationRequestID,
		})
	}

	catalog := lsCatalog{Version: 1, Root: tree.RootPath(), Companies: []lsCatalogCompany{}}
	if directory, err := knowledge.ResearchDirectory(); err == nil {
		catalog.Research = &lsCatalogResearch{Path: directory}
	}
	if directory, err := runner.OrdinaryDirectory(); err == nil {
		catalog.Ordinary = &lsCatalogResearch{Path: directory}
	}
	for _, c := range tree.Companies() {
		if company != "" && c.Slug != company {
			continue
		}
		entry := lsCatalogCompany{Slug: c.Slug, Path: c.Path, Products: []lsCatalogProduct{}}
		for _, p := range tree.Products(c.Slug) {
			if product != "" && p.Slug != product {
				continue
			}
			group := lsCatalogProduct{Slug: p.Slug, Path: p.Path, Repos: []lsCatalogRepo{}}
			for _, repo := range tree.Repos(c.Slug, p.Slug) {
				children := byRepo[repo.Ref()]
				if children == nil {
					children = []lsCatalogWorkspace{}
				}
				group.Repos = append(group.Repos, lsCatalogRepo{Name: repo.Slug, Path: repo.Path, Worktrees: children})
			}
			entry.Products = append(entry.Products, group)
		}
		catalog.Companies = append(catalog.Companies, entry)
	}
	return json.NewEncoder(out).Encode(catalog)
}

func printFlat(out io.Writer, tree *world.Tree, company, product string, paths bool) {
	for _, repo := range tree.Repos(company, product) {
		if paths {
			fmt.Fprintln(out, repo.Path)
			continue
		}
		fmt.Fprintln(out, repo.Ref())
	}
}

// Box-drawing pieces for the tree view.
const (
	branch   = "├─ "
	lastItem = "└─ "
	pipe     = "│  "
	blank    = "   "
)

func printTree(out io.Writer, tree *world.Tree, company, product string) {
	renderer := ui.New(out)

	companies := tree.Companies()
	if company != "" {
		companies = filterCompanies(companies, company)
	}

	var products, repos int

	renderer.Blank()
	for _, c := range companies {
		renderer.Line(renderer.Bold(c.Slug))

		visible := tree.Products(c.Slug)
		if product != "" {
			visible = filterProducts(visible, product)
		}

		for i, p := range visible {
			last := i == len(visible)-1
			products++

			connector, indent := branch, pipe
			if last {
				connector, indent = lastItem, blank
			}

			children := tree.Repos(c.Slug, p.Slug)
			line := renderer.Muted(connector) + renderer.Accent(p.Slug)
			if len(children) == 0 {
				line += renderer.Muted("  (нет репозиториев)")
			}
			renderer.Line(line)

			for j, repo := range children {
				repos++
				childConnector := branch
				if j == len(children)-1 {
					childConnector = lastItem
				}
				renderer.Line(renderer.Muted(indent+childConnector) + repo.Slug)
			}
		}
	}

	renderer.Blank()
	renderer.Line(renderer.Muted(summaryLine(len(companies), products, repos)))
	renderer.Blank()
}

func summaryLine(companies, products, repos int) string {
	return fmt.Sprintf("%s · %s · %s",
		ui.Plural(companies, "компания", "компании", "компаний"),
		ui.Plural(products, "продукт", "продукта", "продуктов"),
		ui.Plural(repos, "репозиторий", "репозитория", "репозиториев"),
	)
}

func filterCompanies(all []world.Company, want string) []world.Company {
	var out []world.Company
	for _, item := range all {
		if item.Slug == want {
			out = append(out, item)
		}
	}
	return out
}

func filterProducts(all []world.Product, want string) []world.Product {
	var out []world.Product
	for _, item := range all {
		if item.Slug == want {
			out = append(out, item)
		}
	}
	return out
}

func argAt(args []string, index int) string {
	if index < len(args) {
		return args[index]
	}
	return ""
}
