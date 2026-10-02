package runner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type secretMask struct {
	Path      string
	Directory bool
}

// Project-local provider state and common plaintext credential files do not
// cross the task boundary. The source files themselves are never modified.
func secretMasks(root string, additionalRoots ...string) ([]secretMask, error) {
	out := []secretMask{}
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Name() == ".git" {
			if path != filepath.Join(root, ".git") {
				if d.Type()&fs.ModeSymlink != 0 {
					return fmt.Errorf("nested Git metadata symlink denied")
				}
				out = append(out, secretMask{path, d.IsDir()})
			}
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		masked := strings.HasPrefix(name, ".env") || name == ".netrc" || name == ".npmrc" || name == "id_rsa" || name == "id_ed25519" || name == "credentials.json" || name == ".ssh" || name == ".aws" || name == ".azure" || name == ".gcloud" || name == ".claude" || name == ".codex" || name == ".hq" || name == ".agent-workspace"
		if !masked {
			return nil
		}
		target := path
		directory := d.IsDir()
		if d.Type()&fs.ModeSymlink != 0 {
			resolved, e := filepath.EvalSymlinks(path)
			if e != nil {
				return nil
			}
			allowed := false
			for _, candidate := range append([]string{root}, additionalRoots...) {
				rel, e := filepath.Rel(candidate, resolved)
				if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil
			}
			target = resolved
			info, e := os.Stat(resolved)
			if e != nil {
				return e
			}
			directory = info.IsDir()
		}
		if !seen[target] {
			out = append(out, secretMask{target, directory})
			seen[target] = true
		}
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	return out, err
}
