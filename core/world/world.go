// Package world describes the on-disk workspace layout: company → product → repo.
//
// It is the single source of truth for that model. The CLI, the TUIs and any
// later daemon resolve paths through here instead of re-deriving them; the zsh
// layer this replaces had grown three slightly different implementations of the
// same scan, which is why `launch` and `setup-context` could disagree about
// which repos exist.
//
// Layout on disk:
//
//	<root>/<company>/.company-config      marks a company
//	<root>/<company>/<product>/.product-config  marks a product
//	<root>/<company>/<product>/<repo>/.git      marks a repo
//
// Anything without its marker is ignored, so scratch directories and the
// shared <root>/.worktrees pool never show up as workspaces.
package world

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Marker files that promote a directory to a company, product or repo.
const (
	companyMarker = ".company-config"
	productMarker = ".product-config"
	repoMarker    = ".git"
)

// RootEnv is the environment variable that overrides the workspace root. The
// server sets it to a different path, and every tool follows.
const RootEnv = "PROKECTFILES_ROOT"

// Company is a top-level workspace owner, e.g. neurodesk.
type Company struct {
	Slug   string
	Path   string
	Config Config
}

// Product groups repositories inside a company, e.g. neurodesk/agents.
type Product struct {
	Company string
	Slug    string
	Path    string
	Config  Config
}

// Repo is a git checkout inside a product.
type Repo struct {
	Company string
	Product string
	Slug    string
	Path    string
}

// Ref is the company/product/repo triple that identifies a repo.
func (r Repo) Ref() string {
	return fmt.Sprintf("%s/%s/%s", r.Company, r.Product, r.Slug)
}

// Tree is an immutable snapshot of the workspace root taken by Scan.
type Tree struct {
	root      string
	companies []Company
	products  []Product
	repos     []Repo
	warnings  []string
}

// Root reports the workspace root: $PROKECTFILES_ROOT when set, otherwise
// ~/Desktop/Prokectfiles.
func Root() (string, error) {
	if root := os.Getenv(RootEnv); root != "" {
		return root, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, "Desktop", "Prokectfiles"), nil
}

// Scan walks the workspace root once and returns everything it found. Directory
// entries that cannot be read are recorded as warnings rather than aborting the
// scan, so one unreadable company never hides the rest.
func Scan(root string) (*Tree, error) {
	if root == "" {
		return nil, errors.New("workspace root is empty")
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read workspace root %s: %w", root, err)
	}

	tree := &Tree{root: root}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		companyPath := filepath.Join(root, entry.Name())
		if !hasMarker(companyPath, companyMarker) {
			continue
		}

		config, err := readConfig(filepath.Join(companyPath, companyMarker))
		if err != nil {
			tree.warnings = append(tree.warnings, err.Error())
			continue
		}

		tree.companies = append(tree.companies, Company{
			Slug:   entry.Name(),
			Path:   companyPath,
			Config: config,
		})
		tree.scanProducts(entry.Name(), companyPath)
	}

	tree.sort()
	return tree, nil
}

// ScanDefault scans the root reported by Root.
func ScanDefault() (*Tree, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	return Scan(root)
}

func (t *Tree) scanProducts(company, companyPath string) {
	entries, err := os.ReadDir(companyPath)
	if err != nil {
		t.warnings = append(t.warnings, fmt.Sprintf("read company %s: %v", company, err))
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		productPath := filepath.Join(companyPath, entry.Name())
		if !hasMarker(productPath, productMarker) {
			continue
		}

		config, err := readConfig(filepath.Join(productPath, productMarker))
		if err != nil {
			t.warnings = append(t.warnings, err.Error())
			continue
		}

		t.products = append(t.products, Product{
			Company: company,
			Slug:    entry.Name(),
			Path:    productPath,
			Config:  config,
		})
		t.scanRepos(company, entry.Name(), productPath)
	}
}

func (t *Tree) scanRepos(company, product, productPath string) {
	entries, err := os.ReadDir(productPath)
	if err != nil {
		t.warnings = append(t.warnings, fmt.Sprintf("read product %s/%s: %v", company, product, err))
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		repoPath := filepath.Join(productPath, entry.Name())
		// A worktree checkout carries .git as a file, not a directory, so the
		// marker is checked by existence rather than by type.
		if !hasMarker(repoPath, repoMarker) {
			continue
		}

		t.repos = append(t.repos, Repo{
			Company: company,
			Product: product,
			Slug:    entry.Name(),
			Path:    repoPath,
		})
	}
}

func (t *Tree) sort() {
	sort.Slice(t.companies, func(i, j int) bool {
		return t.companies[i].Slug < t.companies[j].Slug
	})
	sort.Slice(t.products, func(i, j int) bool {
		if t.products[i].Company != t.products[j].Company {
			return t.products[i].Company < t.products[j].Company
		}
		return t.products[i].Slug < t.products[j].Slug
	})
	sort.Slice(t.repos, func(i, j int) bool {
		if t.repos[i].Company != t.repos[j].Company {
			return t.repos[i].Company < t.repos[j].Company
		}
		if t.repos[i].Product != t.repos[j].Product {
			return t.repos[i].Product < t.repos[j].Product
		}
		return t.repos[i].Slug < t.repos[j].Slug
	})
}

// RootPath reports the root this tree was scanned from.
func (t *Tree) RootPath() string { return t.root }

// Warnings reports directories that could not be read during the scan.
func (t *Tree) Warnings() []string { return append([]string(nil), t.warnings...) }

// Companies returns every company, sorted by slug.
func (t *Tree) Companies() []Company { return append([]Company(nil), t.companies...) }

// Products returns the products of one company, or every product when company
// is empty.
func (t *Tree) Products(company string) []Product {
	var out []Product
	for _, product := range t.products {
		if company == "" || product.Company == company {
			out = append(out, product)
		}
	}
	return out
}

// Repos returns the repos under company/product. Empty arguments widen the
// selection, so Repos("", "") returns everything.
func (t *Tree) Repos(company, product string) []Repo {
	var out []Repo
	for _, repo := range t.repos {
		if company != "" && repo.Company != company {
			continue
		}
		if product != "" && repo.Product != product {
			continue
		}
		out = append(out, repo)
	}
	return out
}

// FindRepo resolves a repo by slug across the whole tree. It reports every
// match, because slugs are only unique within a product.
func (t *Tree) FindRepo(slug string) []Repo {
	var out []Repo
	for _, repo := range t.repos {
		if repo.Slug == slug {
			out = append(out, repo)
		}
	}
	return out
}

// HasCompany reports whether the company exists in this tree.
func (t *Tree) HasCompany(company string) bool {
	for _, candidate := range t.companies {
		if candidate.Slug == company {
			return true
		}
	}
	return false
}

// HasProduct reports whether company/product exists in this tree.
func (t *Tree) HasProduct(company, product string) bool {
	for _, candidate := range t.products {
		if candidate.Company == company && candidate.Slug == product {
			return true
		}
	}
	return false
}

func hasMarker(dir, marker string) bool {
	_, err := os.Stat(filepath.Join(dir, marker))
	return err == nil
}

// IsCompanyDir reports whether dir carries the company marker. The Is*Dir
// helpers exist so path-based resolvers (ctx, scripts) share the marker
// definition with Scan instead of re-hardcoding file names.
func IsCompanyDir(dir string) bool { return hasMarker(dir, companyMarker) }

// IsProductDir reports whether dir carries the product marker.
func IsProductDir(dir string) bool { return hasMarker(dir, productMarker) }

// IsRepoDir reports whether dir carries the repo marker (.git as dir or file).
func IsRepoDir(dir string) bool { return hasMarker(dir, repoMarker) }
