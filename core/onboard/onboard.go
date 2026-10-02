// Package onboard creates companies and products in the workspace.
//
// A company is a directory with .company-config and an SSH identity;
// a product is a directory with .product-config and cloned
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

	// SkipExternal omits the effects that reach outside the workspace: the
	// company SSH identity in ~/.ssh. Password managers are never invoked.
	SkipExternal bool
}

// vcsHost resolves the vcs[:host] shorthand.
func vcsHost(raw string) (vcs, host string, err error) {
	vcs, host, found := strings.Cut(raw, ":")
	var defaultHost string
	switch vcs {
	case "gitlab":
		defaultHost = "gitlab.com"
	case "github":
		defaultHost = "github.com"
	case "bitbucket":
		defaultHost = "bitbucket.org"
	case "local":
		defaultHost = "local"
	default:
		return "", "", fmt.Errorf("неизвестный VCS: %s", vcs)
	}
	if !found {
		host = defaultHost
	}
	if vcs == "local" && host != "local" {
		return "", "", fmt.Errorf("local VCS does not accept a remote host")
	}
	if err := validateHost(host); err != nil {
		return "", "", err
	}
	return vcs, host, nil
}

// Company publishes a complete workspace only after local files and SSH setup
// succeed. Failed setup leaves no partial company and can be retried; any SSH
// key or alias already created is preserved and reused.
func Company(opts CompanyOptions, out io.Writer) error {
	if err := world.ValidateSegment(opts.Slug); err != nil {
		return fmt.Errorf("company: %w", err)
	}
	if err := validateNamespace(opts.Namespace); err != nil {
		return err
	}
	if err := validateEmail(opts.Email); err != nil {
		return err
	}
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
	dir, err := world.SafePath(workRoot, co)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(dir); err == nil {
		return fmt.Errorf("компания %q уже существует: %s", co, dir)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("создание workspace root: %w", err)
	}

	title := strings.ToUpper(co[:1]) + co[1:]
	sshHost := ""
	if vcs != "local" {
		sshHost = vcs + "-" + co
	}
	agents, err := expandTemplate(filepath.Join(tpl, "AGENTS.md.company.tmpl"), map[string]string{
		"CO": co, "CO_TITLE": title, "VCS": vcs, "HOST": host,
		"NS": opts.Namespace, "SSH_HOST": sshHost, "EMAIL": opts.Email,
	})
	if err != nil {
		return err
	}
	config := fmt.Sprintf("slug: %s\nvcs: %s\nhost: %s\nnamespace: %s\nssh_host: %s\ngit_email: %s\n",
		co, vcs, host, opts.Namespace, sshHost, opts.Email)

	envrc := fmt.Sprintf(`# Git identity for %s
export GIT_AUTHOR_EMAIL="%s"
export GIT_COMMITTER_EMAIL="%s"

# Company-wide secrets will be wired here by the redesigned secrets layer
# (hq secret is a stub for now). Plaintext secrets never enter git.
`, co, opts.Email, opts.Email)
	readme := fmt.Sprintf(`# %s

VCS: %s @ %s
Namespace: %s
SSH alias: %s

## Products

(добавляются через `+"`new-project %s <product> [repos...]`"+`)
`, title, vcs, host, opts.Namespace, sshHost, co)
	staged, err := os.MkdirTemp(filepath.Dir(dir), ".hq-company-"+co+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	for name, body := range map[string]string{"AGENTS.md": agents, ".company-config": config, ".envrc": envrc, "README.md": readme} {
		if err := os.WriteFile(filepath.Join(staged, name), []byte(body), 0o644); err != nil {
			return fmt.Errorf("stage company %s: %w", co, err)
		}
	}
	keyPath := filepath.Join(home, ".ssh", co+"_id_ed25519")
	if vcs != "local" && !opts.SkipExternal {
		if err := ensureSSHIdentity(co, vcs, host, sshHost, keyPath, opts.Email, out); err != nil {
			return fmt.Errorf("company %q was not published; existing SSH files were preserved; retry the same command: %w", co, err)
		}
	}
	if _, err := world.SafePath(workRoot, co); err != nil {
		return err
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		return fmt.Errorf("company destination became occupied: %s", dir)
	}
	if err := os.Rename(staged, dir); err != nil {
		return fmt.Errorf("publish company %s: %w", co, err)
	}
	for _, name := range []string{"AGENTS.md", ".company-config", ".envrc", "README.md"} {
		fmt.Fprintf(out, "✓ %s\n", filepath.Join(dir, name))
	}

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
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return fmt.Errorf("create SSH directory: %w", err)
	}
	if info, err := os.Lstat(keyPath); os.IsNotExist(err) {
		comment := email
		if comment == "" {
			comment = co + "-machine"
		}
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", comment)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ssh-keygen: %w: %s", err, output)
		}
		fmt.Fprintf(out, "✓ SSH-ключ создан: %s\n", keyPath)
	} else if err != nil {
		return fmt.Errorf("inspect SSH key: %w", err)
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("SSH key is not a regular file: %s", keyPath)
	} else {
		fmt.Fprintf(out, "→ SSH-ключ уже есть: %s\n", keyPath)
	}

	config := filepath.Join(filepath.Dir(keyPath), "config")
	data, err := os.ReadFile(config)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read SSH config: %w", err)
	}
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
	if err := file.Close(); err != nil {
		return fmt.Errorf("закрытие %s: %w", config, err)
	}
	fmt.Fprintf(out, "✓ SSH-алиас %q добавлен в %s\n", sshHost, config)
	return nil
}
