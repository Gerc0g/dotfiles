package runner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// prepareGit imports data-only Git metadata. No host config, hook, helper,
// credential, linked-worktree registry or alternate object directory is exposed.
func prepareGit(b Binding, destination string) (string, error) {
	common, gitDir, err := gitLocations(b)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(destination, 0700); err != nil {
		return "", err
	}
	source, err := os.OpenRoot(common)
	if err != nil {
		return "", err
	}
	defer source.Close()
	for _, name := range []string{"objects", "refs", "packed-refs", "shallow"} {
		if err = copyDataTree(source, name, destination); err != nil {
			return "", err
		}
	}
	taskGit, err := os.OpenRoot(gitDir)
	if err != nil {
		return "", err
	}
	defer taskGit.Close()
	for _, name := range []string{"HEAD", "index"} {
		if err = copyDataTree(taskGit, name, destination); err != nil {
			return "", err
		}
	}
	if err = os.WriteFile(filepath.Join(destination, "config"), []byte("[core]\n\trepositoryformatversion = 0\n\tbare = false\n[credential]\n\thelper =\n[user]\n\tname = HQ Agent\n\temail = agent@localhost\n"), 0600); err != nil {
		return "", err
	}
	return destination, nil
}
func copyDataTree(source *os.Root, name, destination string) error {
	if _, err := source.Lstat(name); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return fs.WalkDir(source.FS(), name, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink Git data denied: %s", p)
		}
		if p == "objects/info/alternates" || p == "objects/info/http-alternates" {
			return fmt.Errorf("external Git object alternates denied")
		}
		dest := filepath.Join(destination, p)
		if d.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("non-regular Git data denied")
		}
		raw, err := source.ReadFile(p)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return err
		}
		return os.WriteFile(dest, raw, 0600)
	})
}
