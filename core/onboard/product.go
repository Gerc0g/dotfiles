package onboard

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/workspace"
	"github.com/Gerc0g/dotfiles/core/world"
)

// ProductOptions describes one new product and its repositories.
type ProductOptions struct {
	Company   string
	Product   string
	Namespace string // override; company namespace otherwise
	Repos     []string
}

// Product bootstraps a product: AGENTS.md, configs, cloned repos. Shell
// shortcuts are table-driven (shell/30-projects.zsh) and the company-level
// function is registered automatically, so no shell code is generated.
func Product(opts ProductOptions, out io.Writer) error {
	ctx, err := productContext(opts, false)
	if err != nil {
		return err
	}
	tpl, err := templatesDir()
	if err != nil {
		return err
	}

	co, prod := opts.Company, opts.Product
	prodDir, ns := ctx.dir, ctx.namespace
	// Preflight every context path before writing any of them. Existing files
	// are preserved, including caller-owned additions to the config.
	for _, name := range []string{"AGENTS.md", ".product-config", ".envrc"} {
		if _, err := world.SafePath(ctx.root, co, prod, name); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(prodDir, 0o755); err != nil {
		return fmt.Errorf("создание %s: %w", prodDir, err)
	}

	if err := ensureAgents(tpl, prodDir, "product", map[string]string{"CO": co, "PROD": prod}, out); err != nil {
		return err
	}

	if err := ensureEnvrc(prodDir, productEnvrc, out); err != nil {
		return err
	}

	config := fmt.Sprintf("slug: %s\nnamespace: %s\n", prod, ns)
	if err := ensureFile(prodDir, ".product-config", config, out); err != nil {
		return err
	}
	if err := provisionRepos(ctx, tpl, opts, out); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n✅ %s/%s создан (ns: %s)\n\nДальше:\n", co, prod, ns)
	fmt.Fprintf(out, "  1. onboard %s %s                    (заполнить TODO в product AGENTS.md)\n", co, prod)
	fmt.Fprintf(out, "  2. analyze-product %s %s            (опц: deep dive → docs/ARCHITECTURE.md)\n", co, prod)
	fmt.Fprintln(out, "  3. cd <repo> && onboard              (для каждого репо)")
	fmt.Fprintln(out, "  4. cd <repo> && dev-stack connect    (подключить к dev-stack)")
	fmt.Fprintf(out, "  5. %s %s                             (создать worktree и начать работу)\n", co, prod)
	defaultRepo := ""
	if len(opts.Repos) == 1 {
		defaultRepo = opts.Repos[0]
	}
	fmt.Fprintf(out, "\nОпционально — короткий алиас в _PROJECT_SHORTCUTS (shell/30-projects.zsh):\n  \"%s:%s:%s:%s\"\n",
		prod, co, prod, defaultRepo)
	return nil
}

// Repositories adds clones or existing Git checkouts to a registered product.
// It never writes the product's AGENTS.md, .product-config or .envrc.
func Repositories(opts ProductOptions, out io.Writer) error {
	if len(opts.Repos) == 0 {
		return fmt.Errorf("specify at least one repository")
	}
	ctx, err := productContext(opts, true)
	if err != nil {
		return err
	}
	tpl, err := templatesDir()
	if err != nil {
		return err
	}
	if err := provisionRepos(ctx, tpl, opts, out); err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ Репозитории %s/%s готовы\n", opts.Company, opts.Product)
	return nil
}

type productSetup struct {
	root, dir, namespace, remote string
}

func productContext(opts ProductOptions, requireProduct bool) (productSetup, error) {
	if err := validateProductOptions(opts); err != nil {
		return productSetup{}, err
	}
	root, err := world.Root()
	if err != nil {
		return productSetup{}, err
	}
	dir, err := world.SafePath(root, opts.Company, opts.Product)
	if err != nil {
		return productSetup{}, err
	}
	// Marker symlinks must not be followed by Scan.
	for _, parts := range [][]string{{opts.Company, ".company-config"}, {opts.Company, opts.Product, ".product-config"}} {
		path, err := world.SafePath(root, parts...)
		if err != nil {
			return productSetup{}, err
		}
		if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
			return productSetup{}, fmt.Errorf("config is not a regular file: %s", path)
		} else if err != nil && !os.IsNotExist(err) {
			return productSetup{}, err
		}
	}
	// Reject any unsafe repository destination before creating product files.
	for _, repo := range opts.Repos {
		if _, err := world.SafePath(root, opts.Company, opts.Product, repo); err != nil {
			return productSetup{}, err
		}
	}
	tree, err := world.Scan(root)
	if err != nil {
		return productSetup{}, err
	}
	var company *world.Company
	for _, candidate := range tree.Companies() {
		if candidate.Slug == opts.Company {
			company = &candidate
			break
		}
	}
	if company == nil {
		return productSetup{}, fmt.Errorf("компания %q не найдена — создай: new-company %s <vcs> <namespace>", opts.Company, opts.Company)
	}
	if err := validateCompanyConfig(company.Config); err != nil {
		return productSetup{}, fmt.Errorf("company %s config: %w", opts.Company, err)
	}
	ns := opts.Namespace
	if ns == "" {
		ns = company.Config.Get("namespace")
	}
	var existing *world.Product
	for _, candidate := range tree.Products(opts.Company) {
		if candidate.Slug == opts.Product {
			existing = &candidate
			break
		}
	}
	if existing != nil {
		if slug := existing.Config.Get("slug"); slug != "" && slug != opts.Product {
			return productSetup{}, fmt.Errorf("product config slug %q does not match directory %q", slug, opts.Product)
		}
		stored := existing.Config.Get("namespace")
		if err := validateNamespace(stored); err != nil {
			return productSetup{}, fmt.Errorf("existing product config: %w", err)
		}
		if opts.Namespace != "" && opts.Namespace != stored {
			return productSetup{}, fmt.Errorf("product %s/%s already uses namespace %q; --ns=%q conflicts; edit .product-config explicitly to change it", opts.Company, opts.Product, stored, opts.Namespace)
		}
		ns = stored
	} else if requireProduct {
		return productSetup{}, fmt.Errorf("product %s/%s is not registered; create it with hq onboard product first", opts.Company, opts.Product)
	}
	if err := validateNamespace(ns); err != nil {
		return productSetup{}, err
	}
	return productSetup{root: root, dir: dir, namespace: ns, remote: remoteBase(company.Config, ns)}, nil
}

func provisionRepos(ctx productSetup, tpl string, opts ProductOptions, out io.Writer) error {
	var failures []error
	for _, repo := range opts.Repos {
		if err := ensureRepo(tpl, ctx.dir, opts.Company, opts.Product, repo, ctx.remote, out); err != nil {
			failure := fmt.Errorf("repository %s: %w", repo, err)
			failures = append(failures, failure)
			fmt.Fprintf(out, "✗ %v\n", failure)
		}
	}
	syncEditor(ctx.root, out)
	if len(failures) != 0 {
		return fmt.Errorf("repository setup incomplete; successful work was preserved; retry failed repositories with hq onboard repo %s %s <repo...>: %w", opts.Company, opts.Product, errors.Join(failures...))
	}
	return nil
}

const productEnvrc = `source_up

# Product-scoped secrets will be wired here by the redesigned secrets layer.
`

const repoEnvrc = `source_up

# Repo-scoped secrets will be wired here by the redesigned secrets layer.
# Shared dev infra (Postgres/Redis/Qdrant/…): wire endpoints + create the DB with
#   dev-stack connect
`

func ensureEnvrc(dir, content string, out io.Writer) error {
	return ensureFile(dir, ".envrc", content, out)
}

func ensureFile(dir, name, content string, out io.Writer) error {
	path, err := world.SafePath(dir, name)
	if err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("not a regular file: %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	_, writeErr := file.WriteString(content)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Fprintf(out, "✓ %s\n", path)
	return nil
}

func ensureAgents(tpl, dir, kind string, vars map[string]string, out io.Writer) error {
	path, err := world.SafePath(dir, "AGENTS.md")
	if err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("not a regular file: %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	agents, err := expandTemplate(filepath.Join(tpl, "AGENTS.md."+kind+".tmpl"), vars)
	if err != nil {
		return err
	}
	return ensureFile(dir, "AGENTS.md", agents, out)
}

// remoteBase resolves the clone prefix from the company config.
func remoteBase(config world.Config, ns string) string {
	sshHost := config.Get("ssh_host")
	if sshHost != "" && sshHost != "local" {
		return "git@" + sshHost + ":" + ns
	}
	if config.Get("vcs") == "local" {
		return ""
	}
	return "git@" + config.Get("host") + ":" + ns
}

// ensureRepo clones (or inits) one repository and seeds its AGENTS.md/.envrc.
func ensureRepo(tpl, prodDir, co, prod, repo, remote string, out io.Writer) error {
	if err := world.ValidateSegment(repo); err != nil {
		return err
	}
	repoDir, err := world.SafePath(prodDir, repo)
	if err != nil {
		return err
	}
	if info, statErr := os.Lstat(repoDir); os.IsNotExist(statErr) {
		staged, err := os.MkdirTemp(prodDir, ".hq-repo-"+repo+"-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(staged)
		if remote != "" {
			url := fmt.Sprintf("%s/%s.git", remote, repo)
			cmd := exec.Command("git", "clone", url, staged)
			cmd.Stdout = out
			cmd.Stderr = out
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("clone %s: %w", url, err)
			}
		} else {
			if output, err := exec.Command("git", "-C", staged, "init", "-q").CombinedOutput(); err != nil {
				return fmt.Errorf("git init: %w: %s", err, output)
			}
		}
		if err := seedRepo(tpl, staged, co, prod, repo, io.Discard); err != nil {
			return err
		}
		if _, err := world.SafePath(prodDir, repo); err != nil {
			return err
		}
		if _, err := os.Lstat(repoDir); !os.IsNotExist(err) {
			return fmt.Errorf("repository destination became occupied: %s", repoDir)
		}
		if err := os.Rename(staged, repoDir); err != nil {
			return fmt.Errorf("publish repository %s: %w", repo, err)
		}
		fmt.Fprintf(out, "✓ репозиторий готов: %s\n", repoDir)
		return nil
	} else if statErr != nil {
		return statErr
	} else if !info.IsDir() {
		return fmt.Errorf("repository path is not a directory: %s", repoDir)
	}
	marker, err := world.SafePath(prodDir, repo, ".git")
	if err != nil {
		return err
	}
	if _, err := os.Stat(marker); err != nil {
		return fmt.Errorf("existing directory is not a Git checkout: %s: %w", repoDir, err)
	}
	output, err := exec.Command("git", "-C", repoDir, "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return fmt.Errorf("validate existing Git checkout %s: %w: %s", repoDir, err, output)
	}
	toplevel, err := filepath.EvalSymlinks(strings.TrimSpace(string(output)))
	if err != nil || toplevel != repoDir {
		return fmt.Errorf("Git checkout root does not match repository path: %s", repoDir)
	}
	return seedRepo(tpl, repoDir, co, prod, repo, out)
}

func seedRepo(tpl, dir, co, prod, repo string, out io.Writer) error {
	for _, name := range []string{"AGENTS.md", ".envrc"} {
		if _, err := world.SafePath(dir, name); err != nil {
			return err
		}
	}
	if err := ensureAgents(tpl, dir, "repo", map[string]string{"CO": co, "PROD": prod, "REPO": repo}, out); err != nil {
		return err
	}
	return ensureEnvrc(dir, repoEnvrc, out)
}

// syncEditor refreshes the editor project list, best effort.
func syncEditor(workRoot string, out io.Writer) {
	m, err := workspace.New(workRoot)
	if err != nil {
		return
	}
	if err := m.SyncEditorProjects(); err != nil {
		fmt.Fprintf(out, "⚠ VS Code Project Manager sync: %v\n", err)
		return
	}
	fmt.Fprintln(out, "✓ VS Code Project Manager synced")
}
