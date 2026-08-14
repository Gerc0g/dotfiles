package wiki

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

// Rules are binding lessons promoted by the curator. Source of truth lives in
// the vault (repos/<repo>/rules/, shared/rules/); RulesSync materialises them
// into checkouts as `.claude/rules/wiki-*.md` symlinks — main checkout and
// active worktrees — so Claude loads them lazily via `paths:` frontmatter.
// Codex sees the one-line digest through hot.md instead.

const rulesExcludeLine = ".claude/rules/wiki-*"

// desiredRuleLinks maps link name -> vault rule path for one repo.
func desiredRuleLinks(projects, co, prod, repo string, out io.Writer) map[string]string {
	links := map[string]string{}
	sources := []struct{ dir, prefix string }{
		{filepath.Join(projects, co, prod, "repos", repo, "rules"), "wiki-"},
		{filepath.Join(projects, co, prod, "shared", "rules"), "wiki-product-"},
	}

	for _, src := range sources {
		rules, _ := filepath.Glob(filepath.Join(src.dir, "*.md"))
		sort.Strings(rules)
		for _, rule := range rules {
			data, err := os.ReadFile(rule)
			if err != nil {
				continue
			}
			if !hasPathsFrontmatter(string(data)) {
				fmt.Fprintf(out, "⚠ пропуск (нет paths: frontmatter): %s\n", rule)
				continue
			}
			links[src.prefix+filepath.Base(rule)] = rule
		}
	}
	return links
}

// ruleCheckouts lists the main checkout and active worktrees of one repo.
func ruleCheckouts(workRoot, co, prod, repo string) []string {
	var dirs []string
	main := filepath.Join(workRoot, co, prod, repo)
	if _, err := os.Stat(filepath.Join(main, ".git")); err == nil {
		dirs = append(dirs, main)
	}

	pool := filepath.Join(workRoot, ".worktrees", co, prod, repo)
	entries, err := os.ReadDir(pool)
	if err != nil {
		return dirs
	}
	for _, entry := range entries {
		wt := filepath.Join(pool, entry.Name())
		if _, err := os.Stat(filepath.Join(wt, ".git")); err == nil {
			dirs = append(dirs, wt)
		}
	}
	return dirs
}

// syncRepoRules links desired rules into every checkout of one repo and
// prunes stale wiki-* links this tool created earlier.
func syncRepoRules(projects, workRoot, co, prod, repo string, dryRun bool, out io.Writer) (linked, pruned int) {
	links := desiredRuleLinks(projects, co, prod, repo, out)

	for _, checkout := range ruleCheckouts(workRoot, co, prod, repo) {
		rulesDir := filepath.Join(checkout, ".claude", "rules")

		var names []string
		for name := range links {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			target := links[name]
			dest := filepath.Join(rulesDir, name)
			if current, err := os.Readlink(dest); err == nil && current == target {
				continue
			}
			if dryRun {
				fmt.Fprintf(out, "линк: %s -> %s\n", dest, target)
			} else {
				_ = os.MkdirAll(rulesDir, 0o755)
				_ = os.Remove(dest)
				if err := os.Symlink(target, dest); err != nil {
					fmt.Fprintf(out, "⚠ линк %s: %v\n", dest, err)
					continue
				}
			}
			linked++
		}

		stale, _ := filepath.Glob(filepath.Join(rulesDir, "wiki-*"))
		for _, path := range stale {
			name := filepath.Base(path)
			if _, wanted := links[name]; wanted {
				continue
			}
			dest, err := os.Readlink(path)
			if err != nil {
				continue
			}
			// Only manage links we created: absolute symlinks into the vault.
			if !strings.HasPrefix(dest, projects) {
				continue
			}
			if dryRun {
				fmt.Fprintf(out, "прунинг: %s\n", path)
			} else {
				_ = os.Remove(path)
			}
			pruned++
		}

		if len(links) > 0 && !dryRun {
			ensureRepoExclude(checkout)
		}
	}
	return linked, pruned
}

// ensureRepoExclude keeps wiki-* rule links out of `git status`. Worktrees
// share excludes via the common git dir, so only real checkouts are touched.
func ensureRepoExclude(checkout string) {
	gitDir := filepath.Join(checkout, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return
	}
	exclude := filepath.Join(gitDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return
	}
	data, _ := os.ReadFile(exclude)
	for _, line := range strings.Split(string(data), "\n") {
		if line == rulesExcludeLine {
			return
		}
	}
	file, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(rulesExcludeLine + "\n")
}

// RulesSync materialises vault rules for every repo the scope covers. An
// empty scope covers everything. Repos come from the world scan, so scratch
// directories never receive links.
func RulesSync(scopeArg string, dryRun bool, out io.Writer) error {
	projects, err := ProjectsRoot()
	if err != nil {
		return err
	}
	workRoot, err := world.Root()
	if err != nil {
		return err
	}
	tree, err := world.Scan(workRoot)
	if err != nil {
		return err
	}

	var scope Scope
	if scopeArg != "" {
		if scope, err = ParseScope(scopeArg); err != nil {
			return err
		}
	}

	totalLinked, totalPruned := 0, 0
	for _, repo := range tree.Repos(scope.Company, scope.Product) {
		if scope.Repo != "" && repo.Slug != scope.Repo {
			continue
		}
		linked, pruned := syncRepoRules(projects, workRoot, repo.Company, repo.Product, repo.Slug, dryRun, out)
		totalLinked += linked
		totalPruned += pruned
	}

	verb := "изменено"
	if dryRun {
		verb = "изменилось бы"
	}
	fmt.Fprintf(out, "rules-sync: %s %d линков, удалено %d\n", verb, totalLinked, totalPruned)
	return nil
}
