package setup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// VaultEnv overrides where the WikiPedik vault lives.
	VaultEnv = "WIKIPEDIK_ROOT"
	// VaultRemoteEnv overrides the clone source. Set it to an empty string to
	// skip cloning and start from an empty local vault instead.
	VaultRemoteEnv = "WIKIPEDIK_REMOTE"

	defaultVaultRemote = "git@github.com:JustChimera/WikiPedik.git"
)

// defaultVault is where the vault lives unless VaultEnv says otherwise.
func defaultVault(home string) string {
	return filepath.Join(home, "Desktop", "WikiPedik")
}

// vaultStep makes sure the knowledge vault exists as a git repository.
//
// The vault is data, not configuration: it holds the memory the agents write
// and the notes the user reads, and it is the one thing here that cannot be
// regenerated. So the step is deliberately timid — it will clone into an empty
// path or create a fresh skeleton, but it never touches a directory that
// already has content it did not put there.
func vaultStep() Step {
	return Step{
		Name:  "wikipedik-vault",
		About: "вольт знаний существует и находится под git",
		check: func(env Env) Result {
			root := env.Vault

			info, err := os.Stat(root)
			if os.IsNotExist(err) {
				return missing("%s", short(root))
			}
			if err != nil {
				return drifted("%s: %v", short(root), err)
			}
			if !info.IsDir() {
				return drifted("%s существует, но это не каталог", short(root))
			}

			if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
				return ok("%s", short(root))
			}

			empty, err := isEmptyDir(root)
			if err != nil {
				return drifted("%s: %v", short(root), err)
			}
			if empty {
				return missing("%s пуст", short(root))
			}

			return drifted("%s не под git — разберитесь вручную, там могут быть заметки", short(root))
		},
		apply: func(env Env) error {
			root := env.Vault

			if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
				return nil
			}

			if err := ensureDir(root); err != nil {
				return err
			}

			empty, err := isEmptyDir(root)
			if err != nil {
				return err
			}
			if !empty {
				return fmt.Errorf("%s не пуст и не под git; перенесите или удалите содержимое вручную", root)
			}

			remote, explicit := os.LookupEnv(VaultRemoteEnv)
			if !explicit {
				remote = defaultVaultRemote
			}

			if remote != "" {
				return cloneVault(remote, root)
			}
			return scaffoldVault(root)
		},
	}
}

// cloneVault clones into an existing empty directory. A failure is returned as
// is: on a fresh machine it usually means SSH access is not set up yet, and
// silently falling back to an empty skeleton would look like success while the
// real notes stayed on the server.
func cloneVault(remote, root string) error {
	cmd := exec.Command("git", "clone", remote, root)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("клонирование %s: %w: %s", remote, err, bytes.TrimSpace(out.Bytes()))
	}
	return nil
}

// scaffoldVault creates the minimum structure for a brand-new vault: the
// agent-memory half, the study half and the brand half.
func scaffoldVault(root string) error {
	dirs := []string{
		filepath.Join(root, "dev"),
		filepath.Join(root, "research", "00-inbox"),
		filepath.Join(root, "research", "10-wiki"),
		filepath.Join(root, "Personal Brand"),
	}
	for _, dir := range dirs {
		if err := ensureDir(dir); err != nil {
			return err
		}
	}

	for _, name := range []string{"index.md", "log.md"} {
		path := filepath.Join(root, "research", "10-wiki", name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				return fmt.Errorf("создать %s: %w", path, err)
			}
		}
	}

	if err := exec.Command("git", "-C", root, "init", "-q").Run(); err != nil {
		return fmt.Errorf("git init в %s: %w", root, err)
	}
	return nil
}

func isEmptyDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
