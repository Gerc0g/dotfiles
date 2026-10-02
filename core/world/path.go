package world

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidateSegment validates a company, product, repository or task slug.
// Slugs are single ASCII path components, never options or hidden entries.
func ValidateSegment(value string) error {
	if !segmentPattern.MatchString(value) {
		return fmt.Errorf("invalid slug %q: use letters, digits, '.', '_' or '-', starting with a letter or digit", value)
	}
	return nil
}

// SafePath resolves a path below root without following descendant symlinks.
// A configured root may itself be a symlink; fixed dotfile components are
// allowed, unlike user slugs validated by ValidateSegment. Missing paths are
// supported. This preflight check does not lock out concurrent directory swaps.
func SafePath(root string, parts ...string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("workspace root is empty")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `/\`) || strings.IndexFunc(part, unicode.IsControl) >= 0 {
			return "", fmt.Errorf("unsafe path component %q", part)
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	// Canonicalize the existing prefix, including macOS /var -> /private/var.
	ancestor := abs
	var missing []string
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect root %s: %w", ancestor, err)
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = filepath.Dir(ancestor)
	}
	canonical, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", fmt.Errorf("resolve root %s: %w", ancestor, err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root ancestor is not a directory: %s", canonical)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		canonical = filepath.Join(canonical, missing[i])
	}
	path := canonical
	for i, part := range parts {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing symlink beneath workspace root: %s", path)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("not a directory: %s", path)
		}
	}
	return path, nil
}
