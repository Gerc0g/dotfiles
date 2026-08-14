package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

// Context describes where a directory sits in the workspace model. It is the
// single answer to "which company/product/repo am I in?" — the question the
// zsh layer used to answer with five slightly different parsers.
type Context struct {
	// Kind is company, product, repo or worktree.
	Kind    string
	Company string
	Product string
	Repo    string
	// Worktree-only fields.
	ID     string
	Task   string
	Branch string
	State  string
	// Path is the root directory of the located unit, not the queried dir.
	Path string
}

// Ref renders the context as its company[/product[/repo[/id]]] path.
func (c Context) Ref() string {
	parts := []string{c.Company}
	for _, part := range []string{c.Product, c.Repo, c.ID} {
		if part == "" {
			break
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "/")
}

// Locate resolves the workspace context of a directory under root. Both the
// regular layout <root>/<co>/<prod>/<repo> and the worktree pool
// <root>/.worktrees/<co>/<prod>/<repo>/<id> are recognised, from any depth
// inside them.
func Locate(root, dir string) (Context, error) {
	rootAbs, dirAbs, err := normalize(root, dir)
	if err != nil {
		return Context{}, err
	}

	rel, err := filepath.Rel(rootAbs, dirAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return Context{}, fmt.Errorf("вне рабочего корня %s: %s", root, dir)
	}
	if rel == "." {
		return Context{}, fmt.Errorf("это сам рабочий корень: %s", root)
	}

	parts := strings.Split(filepath.ToSlash(rel), "/")
	if parts[0] == filepath.Base(poolDirName) {
		return locateWorktree(rootAbs, parts, dir)
	}
	return locateCheckout(rootAbs, parts, dir)
}

// poolDirName matches Manager.worktrees relative to the root.
const poolDirName = ".worktrees"

func locateWorktree(rootAbs string, parts []string, dir string) (Context, error) {
	if len(parts) < 5 {
		return Context{}, fmt.Errorf("не воркспейс: %s", dir)
	}

	wtDir := filepath.Join(rootAbs, parts[0], parts[1], parts[2], parts[3], parts[4])
	meta := filepath.Join(wtDir, MetaFile)
	if _, err := os.Stat(meta); err != nil {
		return Context{}, fmt.Errorf("не управляемый worktree: %s", wtDir)
	}

	w, err := readMeta(meta)
	if err != nil {
		return Context{}, err
	}

	ctx := Context{
		Kind:    "worktree",
		Company: w.Company,
		Product: w.Product,
		Repo:    w.Repo,
		ID:      w.ID,
		Task:    w.Task,
		Branch:  w.Branch,
		State:   string(w.State),
		Path:    wtDir,
	}
	// The path spells the triple out too; metadata wins, the path backfills.
	if ctx.Company == "" {
		ctx.Company = parts[1]
	}
	if ctx.Product == "" {
		ctx.Product = parts[2]
	}
	if ctx.Repo == "" {
		ctx.Repo = parts[3]
	}
	return ctx, nil
}

func locateCheckout(rootAbs string, parts []string, dir string) (Context, error) {
	companyDir := filepath.Join(rootAbs, parts[0])
	if !world.IsCompanyDir(companyDir) {
		return Context{}, fmt.Errorf("не компания (нет .company-config): %s", companyDir)
	}

	ctx := Context{Kind: "company", Company: parts[0], Path: companyDir}

	if len(parts) < 2 {
		return ctx, nil
	}
	productDir := filepath.Join(companyDir, parts[1])
	if !world.IsProductDir(productDir) {
		return ctx, nil
	}
	ctx.Kind = "product"
	ctx.Product = parts[1]
	ctx.Path = productDir

	if len(parts) < 3 {
		return ctx, nil
	}
	repoDir := filepath.Join(productDir, parts[2])
	if !world.IsRepoDir(repoDir) {
		return ctx, nil
	}
	ctx.Kind = "repo"
	ctx.Repo = parts[2]
	ctx.Path = repoDir

	return ctx, nil
}

// normalize makes both paths absolute and resolves symlinks best effort, so a
// root reached through a link and a physical cwd still share a prefix.
func normalize(root, dir string) (rootAbs, dirAbs string, err error) {
	rootAbs, err = filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("нормализация %s: %w", root, err)
	}
	dirAbs, err = filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("нормализация %s: %w", dir, err)
	}
	if resolved, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = resolved
	}
	if resolved, err := filepath.EvalSymlinks(dirAbs); err == nil {
		dirAbs = resolved
	}
	return rootAbs, dirAbs, nil
}
