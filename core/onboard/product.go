package onboard

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

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
	workRoot, err := world.Root()
	if err != nil {
		return err
	}
	tpl, err := templatesDir()
	if err != nil {
		return err
	}

	tree, err := world.Scan(workRoot)
	if err != nil {
		return err
	}
	var company *world.Company
	for _, c := range tree.Companies() {
		if c.Slug == opts.Company {
			company = &c
			break
		}
	}
	if company == nil {
		return fmt.Errorf("компания %q не найдена — создай: new-company %s <vcs> <namespace>",
			opts.Company, opts.Company)
	}

	co, prod := opts.Company, opts.Product
	ns := opts.Namespace
	if ns == "" {
		ns = company.Config.Get("namespace")
	}

	prodDir := filepath.Join(workRoot, co, prod)
	if err := os.MkdirAll(prodDir, 0o755); err != nil {
		return fmt.Errorf("создание %s: %w", prodDir, err)
	}

	agents, err := expandTemplate(filepath.Join(tpl, "AGENTS.md.product.tmpl"),
		map[string]string{"CO": co, "PROD": prod})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(prodDir, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ %s/AGENTS.md\n", prodDir)

	config := fmt.Sprintf("slug: %s\nnamespace: %s\n", prod, ns)
	if err := os.WriteFile(filepath.Join(prodDir, ".product-config"), []byte(config), 0o644); err != nil {
		return err
	}

	if err := ensureEnvrc(prodDir, productEnvrc, out); err != nil {
		return err
	}

	remote := remoteBase(company.Config, ns)
	for _, repo := range opts.Repos {
		if err := ensureRepo(tpl, prodDir, co, prod, repo, remote, out); err != nil {
			fmt.Fprintf(out, "✗ %s: %v\n", repo, err)
		}
	}

	syncEditor(workRoot, out)

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

const productEnvrc = `source_up

# Product-scoped secrets will be wired here by the redesigned secrets layer.
`

const repoEnvrc = `source_up

# Repo-scoped secrets will be wired here by the redesigned secrets layer.
# Shared dev infra (Postgres/Redis/Qdrant/…): wire endpoints + create the DB with
#   dev-stack connect
`

func ensureEnvrc(dir, content string, out io.Writer) error {
	path := filepath.Join(dir, ".envrc")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("запись %s: %w", path, err)
	}
	fmt.Fprintf(out, "✓ %s (активировать: direnv allow)\n", path)
	return nil
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
	repoDir := filepath.Join(prodDir, repo)
	if _, err := os.Stat(repoDir); err != nil {
		if remote != "" {
			url := fmt.Sprintf("%s/%s.git", remote, repo)
			cmd := exec.Command("git", "clone", url, repoDir)
			cmd.Stdout = out
			cmd.Stderr = out
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("clone %s: %w", url, err)
			}
		} else {
			if err := os.MkdirAll(repoDir, 0o755); err != nil {
				return err
			}
			if output, err := exec.Command("git", "-C", repoDir, "init", "-q").CombinedOutput(); err != nil {
				return fmt.Errorf("git init: %w: %s", err, output)
			}
		}
		fmt.Fprintf(out, "✓ склонирован %s\n", repo)
	}

	agentsPath := filepath.Join(repoDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); err != nil {
		agents, err := expandTemplate(filepath.Join(tpl, "AGENTS.md.repo.tmpl"),
			map[string]string{"CO": co, "PROD": prod, "REPO": repo})
		if err != nil {
			return err
		}
		if err := os.WriteFile(agentsPath, []byte(agents), 0o644); err != nil {
			return err
		}
	}

	return ensureEnvrc(repoDir, repoEnvrc, out)
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
