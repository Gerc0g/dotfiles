// Package onboard creates companies and products in the workspace.
//
// A company is a directory with .company-config, an SSH identity and a
// 1Password vault; a product is a directory with .product-config and cloned
// repositories. AGENTS.md files are rendered from ~/dotfiles/templates with
// an envsubst-compatible restricted variable list, so the templates stay
// editable without touching Go.
package onboard

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

// templatesDir locates ~/dotfiles/templates.
func templatesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "dotfiles", "templates"), nil
}

// expandTemplate renders a template with the restricted variable list.
// Unknown $VARS stay literal — the same contract envsubst '$A $B' gave.
func expandTemplate(path string, vars map[string]string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("чтение шаблона %s: %w", path, err)
	}
	return os.Expand(string(data), func(key string) string {
		if value, ok := vars[key]; ok {
			return value
		}
		return "$" + key
	}), nil
}

// CompanyOptions describes one new company.
type CompanyOptions struct {
	Slug string
	// VCS is the raw vcs[:host] argument.
	VCS       string
	Namespace string
	Email     string
}

// vcsHost resolves the vcs[:host] shorthand.
func vcsHost(raw string) (vcs, host string, err error) {
	vcs, host, found := strings.Cut(raw, ":")
	if found {
		return vcs, host, nil
	}
	switch vcs {
	case "gitlab":
		return vcs, "gitlab.com", nil
	case "github":
		return vcs, "github.com", nil
	case "bitbucket":
		return vcs, "bitbucket.org", nil
	case "local":
		return vcs, "local", nil
	default:
		return "", "", fmt.Errorf("неизвестный VCS: %s", vcs)
	}
}

// Company bootstraps a company workspace: AGENTS.md, configs, SSH identity,
// vault. One SSH key per company, never one global key.
func Company(opts CompanyOptions, out io.Writer) error {
	vcs, host, err := vcsHost(opts.VCS)
	if err != nil {
		return err
	}

	workRoot, err := world.Root()
	if err != nil {
		return err
	}
	tpl, err := templatesDir()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	co := opts.Slug
	dir := filepath.Join(workRoot, co)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("компания %q уже существует: %s", co, dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("создание %s: %w", dir, err)
	}

	title := strings.ToUpper(co[:1]) + co[1:]
	sshHost := ""
	if vcs != "local" {
		sshHost = vcs + "-" + co
	}
	vault := "Work-" + co

	agents, err := expandTemplate(filepath.Join(tpl, "AGENTS.md.company.tmpl"), map[string]string{
		"CO": co, "CO_TITLE": title, "VCS": vcs, "HOST": host,
		"NS": opts.Namespace, "SSH_HOST": sshHost, "VAULT": vault, "EMAIL": opts.Email,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ %s/AGENTS.md\n", dir)

	keyPath := filepath.Join(home, ".ssh", co+"_id_ed25519")
	if vcs != "local" {
		if err := ensureSSHIdentity(co, vcs, host, sshHost, keyPath, opts.Email, out); err != nil {
			return err
		}
	}

	config := fmt.Sprintf("slug: %s\nvcs: %s\nhost: %s\nnamespace: %s\nssh_host: %s\ngit_email: %s\n",
		co, vcs, host, opts.Namespace, sshHost, opts.Email)
	if err := os.WriteFile(filepath.Join(dir, ".company-config"), []byte(config), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ %s/.company-config\n", dir)

	envrc := fmt.Sprintf(`# Git identity for %s
export GIT_AUTHOR_EMAIL="%s"
export GIT_COMMITTER_EMAIL="%s"

# Company-wide secrets will be wired here by the redesigned secrets layer
# (hq secret is a stub for now). Plaintext secrets never enter git.
`, co, opts.Email, opts.Email)
	if err := os.WriteFile(filepath.Join(dir, ".envrc"), []byte(envrc), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ %s/.envrc (активировать: direnv allow)\n", dir)

	readme := fmt.Sprintf(`# %s

VCS: %s @ %s
Namespace: %s
SSH alias: %s
1Password vault: Work-%s

## Products

(добавляются через `+"`new-project %s <product> [repos...]`"+`)
`, title, vcs, host, opts.Namespace, sshHost, co, co)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ %s/README.md\n", dir)

	ensureVault(vault, out)

	fmt.Fprintf(out, "\n✅ Компания %q создана: %s\n\nДальше:\n", co, dir)
	if vcs != "local" {
		fmt.Fprintf(out, "  1. Добавь публичный ключ в %s:\n     pbcopy < %s.pub\n  2. Проверь: ssh -T %s\n", vcs, keyPath, sshHost)
	}
	fmt.Fprintf(out, "  3. cd %s && direnv allow\n", dir)
	fmt.Fprintf(out, "  4. Заполни TODO интерактивно: onboard %s\n", co)
	fmt.Fprintf(out, "  5. Создай продукты: new-project %s <product> [repos...]\n", co)
	return nil
}

// ensureSSHIdentity generates the company key and the ~/.ssh/config alias.
func ensureSSHIdentity(co, vcs, host, sshHost, keyPath, email string, out io.Writer) error {
	if _, err := os.Stat(keyPath); err != nil {
		comment := email
		if comment == "" {
			comment = co + "-machine"
		}
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", comment)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ssh-keygen: %w: %s", err, output)
		}
		fmt.Fprintf(out, "✓ SSH-ключ создан: %s\n", keyPath)
	} else {
		fmt.Fprintf(out, "→ SSH-ключ уже есть: %s\n", keyPath)
	}

	config := filepath.Join(filepath.Dir(keyPath), "config")
	data, _ := os.ReadFile(config)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "Host "+sshHost {
			fmt.Fprintf(out, "→ SSH-алиас %q уже в конфиге\n", sshHost)
			return nil
		}
	}

	entry := fmt.Sprintf(`
# === %s (%s @ %s) ===
Host %s
  HostName %s
  User git
  IdentityFile %s
  IdentitiesOnly yes
`, co, vcs, host, sshHost, host, keyPath)

	file, err := os.OpenFile(config, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("открытие %s: %w", config, err)
	}
	defer file.Close()
	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("запись %s: %w", config, err)
	}
	fmt.Fprintf(out, "✓ SSH-алиас %q добавлен в %s\n", sshHost, config)
	return nil
}

// ensureVault creates the 1Password vault when the CLI is signed in.
func ensureVault(vault string, out io.Writer) {
	if _, err := exec.LookPath("op"); err != nil {
		fmt.Fprintf(out, "⚠ op CLI не найден — создай vault вручную: op vault create %s\n", vault)
		return
	}
	if err := exec.Command("op", "vault", "list").Run(); err != nil {
		fmt.Fprintf(out, "⚠ 1Password CLI не залогинен — `secret signin`, затем: op vault create %s\n", vault)
		return
	}
	if exec.Command("op", "vault", "get", vault).Run() == nil {
		fmt.Fprintf(out, "→ 1Password vault %q уже существует\n", vault)
		return
	}
	if err := exec.Command("op", "vault", "create", vault).Run(); err != nil {
		fmt.Fprintf(out, "⚠ не удалось создать vault %q: %v\n", vault, err)
		return
	}
	fmt.Fprintf(out, "✓ 1Password vault %q создан\n", vault)
}
