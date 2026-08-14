// Package wiki manages WikiPedik — the durable project memory vault.
//
// The vault is one git repository of markdown under
// <vault>/dev/20-projects/<company>/<product>/repos/<repo>. Workers append
// raw captures to _inbox.md; the curator drains them into curated pages;
// hot.md is the small index SessionStart hooks inject into every agent
// session. This package owns the deterministic half of that cycle: scope
// resolution, counters, commits, hot.md rebuild, rule materialisation and
// skeleton bootstrap. The LLM half (drain, synthesis) stays with the curator
// skills — the package only launches them.
package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RootEnv overrides the vault location. Every consumer honours it — the zsh
// predecessor hardcoded ~/Desktop/WikiPedik in one of three places and
// silently diverged.
const RootEnv = "WIKIPEDIK_ROOT"

// VaultRoot is the git root of the vault.
func VaultRoot() (string, error) {
	if root := os.Getenv(RootEnv); root != "" {
		return root, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Desktop", "WikiPedik"), nil
}

// MemoryRoot is the dev zone the curator works in.
func MemoryRoot() (string, error) {
	root, err := VaultRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "dev"), nil
}

// ProjectsRoot is where per-company memory lives.
func ProjectsRoot() (string, error) {
	root, err := MemoryRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "20-projects"), nil
}

// Scope identifies company[/product[/repo]] memory.
type Scope struct {
	Company string
	Product string
	Repo    string
}

// ParseScope validates and splits a co[/prod[/repo]] string.
func ParseScope(s string) (Scope, error) {
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) == 0 || parts[0] == "" || len(parts) > 3 {
		return Scope{}, fmt.Errorf("scope должен быть company[/product[/repo]]: %q", s)
	}
	scope := Scope{Company: parts[0]}
	if len(parts) > 1 {
		scope.Product = parts[1]
	}
	if len(parts) > 2 {
		scope.Repo = parts[2]
	}
	return scope, nil
}

func (s Scope) String() string {
	out := s.Company
	if s.Product != "" {
		out += "/" + s.Product
	}
	if s.Repo != "" {
		out += "/" + s.Repo
	}
	return out
}

// Path is the memory directory of the scope inside the vault.
func (s Scope) Path() (string, error) {
	projects, err := ProjectsRoot()
	if err != nil {
		return "", err
	}
	switch {
	case s.Repo != "":
		return filepath.Join(projects, s.Company, s.Product, "repos", s.Repo), nil
	case s.Product != "":
		return filepath.Join(projects, s.Company, s.Product), nil
	default:
		return filepath.Join(projects, s.Company), nil
	}
}

// ProductLog is the product-level log page, when the scope names a product.
func (s Scope) ProductLog() (string, error) {
	if s.Product == "" {
		return "", nil
	}
	projects, err := ProjectsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(projects, s.Company, s.Product, "log.md"), nil
}

var statusLineRe = regexp.MustCompile(`(?m)^Status: (\w+)$`)

// CountStatus counts inbox entries with the given status under dir.
func CountStatus(dir, wanted string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "_inbox.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, match := range statusLineRe.FindAllStringSubmatch(string(data), -1) {
			if match[1] == wanted {
				count++
			}
		}
		return nil
	})
	return count
}

// HotStamps reports the refresh stamps of hot.md files under dir, at most 5.
func HotStamps(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "hot.md" || len(out) >= 5 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "<!-- last refreshed:") {
				out = append(out, fmt.Sprintf("%s:%d:%s", path, i+1, line))
				break
			}
		}
		return nil
	})
	return out
}
