package wiki

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

// Bootstrap creates the memory skeleton for a company/product/repo and links
// it into the checkouts. Idempotent: existing files are left alone, only the
// human-readable graph pages (<co>.md, <prod>.md, <repo>.md) are rewritten.
//
// Privacy is structural: a repo's company-knowledge link may only point into
// its own company directory, and SanityCheck verifies exactly that.

// render substitutes the {co}/{prod}/{repo}/{repoPath} placeholders.
func render(template string, pairs ...string) string {
	return strings.NewReplacer(pairs...).Replace(template)
}

// ensureFile writes content only when the file does not exist yet.
func ensureFile(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("запись %s: %w", path, err)
	}
	return nil
}

// writeManagedFile always rewrites: these pages are owned by bootstrap.
func writeManagedFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("запись %s: %w", path, err)
	}
	return nil
}

func ensureDirs(dirs ...string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("создание %s: %w", dir, err)
		}
	}
	return nil
}

// safeLink replaces a symlink but refuses to clobber a real file/directory.
func safeLink(target, link string) error {
	if info, err := os.Lstat(link); err == nil && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s существует и не является симлинком — перенеси вручную", link)
	}
	_ = os.Remove(link)
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("линк %s: %w", link, err)
	}
	return nil
}

func ensureExcludeLine(repoPath, line string) error {
	exclude := filepath.Join(repoPath, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return err
	}
	data, _ := os.ReadFile(exclude)
	for _, existing := range strings.Split(string(data), "\n") {
		if existing == line {
			return nil
		}
	}
	file, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(line + "\n")
	return err
}

// registerInIndex maintains the auto-managed lists: upgrades a legacy
// [[x/index]] link to the readable [[x/x]] form, otherwise appends the entry
// (after the section heading when given, at the end otherwise).
func registerInIndex(indexPath, marker, legacyLine, newLine, sectionHeading string) error {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("чтение %s: %w", indexPath, err)
	}
	content := string(data)

	if strings.Contains(content, legacyLine) {
		return writeManagedFile(indexPath, strings.ReplaceAll(content, legacyLine, newLine))
	}
	if strings.Contains(content, marker) {
		return nil
	}

	if sectionHeading != "" {
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if line == sectionHeading {
				lines = append(lines[:i+1], append([]string{newLine}, lines[i+1:]...)...)
				return writeManagedFile(indexPath, strings.Join(lines, "\n"))
			}
		}
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return writeManagedFile(indexPath, content+newLine+"\n")
}

// BootstrapRoot ensures the vault-level index, README and templates exist.
func BootstrapRoot() error {
	projects, err := ProjectsRoot()
	if err != nil {
		return err
	}
	if err := ensureDirs(projects, filepath.Join(projects, "_templates")); err != nil {
		return err
	}

	files := map[string]string{
		filepath.Join(projects, "index.md"):                         tplRootIndex,
		filepath.Join(projects, "README.md"):                        tplRootReadme,
		filepath.Join(projects, "_templates", "lesson-entry.md"):    tplLessonEntry,
		filepath.Join(projects, "_templates", "debugging-story.md"): tplDebuggingStory,
		filepath.Join(projects, "_templates", "open-question.md"):   tplOpenQuestion,
	}
	for path, content := range files {
		if err := ensureFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

// BootstrapCompany creates the company skeleton and registers it in the root
// index.
func BootstrapCompany(co string, out io.Writer) error {
	projects, err := ProjectsRoot()
	if err != nil {
		return err
	}
	coDir := filepath.Join(projects, co)
	if err := ensureDirs(coDir, filepath.Join(coDir, "shared"), filepath.Join(coDir, "shared", "playbooks")); err != nil {
		return err
	}

	r := func(tpl string) string { return render(tpl, "{co}", co) }
	ensured := map[string]string{
		filepath.Join(coDir, "index.md"):                            r(tplCompanyIndex),
		filepath.Join(coDir, "log.md"):                              r(tplCompanyLog),
		filepath.Join(coDir, "health.md"):                           r(tplCompanyHealth),
		filepath.Join(coDir, "privacy.md"):                          r(tplCompanyPrivacy),
		filepath.Join(coDir, "shared", "patterns.md"):               r(tplCompanyPatterns),
		filepath.Join(coDir, "shared", "gotchas.md"):                r(tplCompanyGotchas),
		filepath.Join(coDir, "shared", "glossary.md"):               r(tplCompanyGlossary),
		filepath.Join(coDir, "shared", "cross-product-lessons.md"):  r(tplCompanyCrossLessons),
		filepath.Join(coDir, "shared", "decisions-not-adr.md"):      r(tplCompanyDecisions),
		filepath.Join(coDir, "shared", "playbooks", "debugging.md"): r(tplCompanyPlaybookDebug),
		filepath.Join(coDir, "shared", "playbooks", "release.md"):   r(tplCompanyPlaybookRelease),
		filepath.Join(coDir, "shared", "playbooks", "migration.md"): r(tplCompanyPlaybookMigration),
	}
	for path, content := range ensured {
		if err := ensureFile(path, content); err != nil {
			return err
		}
	}
	if err := writeManagedFile(filepath.Join(coDir, co+".md"), r(tplCompanyPage)); err != nil {
		return err
	}

	if err := registerInIndex(
		filepath.Join(projects, "index.md"),
		"- "+co,
		fmt.Sprintf("- %s: [[%s/index]]", co, co),
		fmt.Sprintf("- %s: [[%s/%s]]", co, co, co),
		"",
	); err != nil {
		return err
	}

	fmt.Fprintf(out, "  ✓ company skeleton: %s\n", coDir)
	return nil
}

// BootstrapProduct creates the product skeleton and registers it in the
// company index.
func BootstrapProduct(co, prod string, out io.Writer) error {
	projects, err := ProjectsRoot()
	if err != nil {
		return err
	}
	coDir := filepath.Join(projects, co)
	prodDir := filepath.Join(coDir, prod)
	if err := ensureDirs(prodDir, filepath.Join(prodDir, "shared"), filepath.Join(prodDir, "repos")); err != nil {
		return err
	}

	r := func(tpl string) string { return render(tpl, "{co}", co, "{prod}", prod) }
	ensured := map[string]string{
		filepath.Join(prodDir, "index.md"):                        r(tplProductIndex),
		filepath.Join(prodDir, "log.md"):                          r(tplProductLog),
		filepath.Join(prodDir, "health.md"):                       r(tplProductHealth),
		filepath.Join(prodDir, "open-questions.md"):               r(tplProductOpenQuestions),
		filepath.Join(prodDir, "shared", "patterns.md"):           r(tplProductPatterns),
		filepath.Join(prodDir, "shared", "gotchas.md"):            r(tplProductGotchas),
		filepath.Join(prodDir, "shared", "interfaces.md"):         r(tplProductInterfaces),
		filepath.Join(prodDir, "shared", "integration-points.md"): r(tplProductIntegrations),
		filepath.Join(prodDir, "shared", "decisions-not-adr.md"):  r(tplProductDecisions),
		filepath.Join(prodDir, "shared", "debugging-playbook.md"): r(tplProductPlaybook),
		filepath.Join(prodDir, "_synthesis-candidates.md"):        r(tplProductSynthesis),
	}
	for path, content := range ensured {
		if err := ensureFile(path, content); err != nil {
			return err
		}
	}
	if err := writeManagedFile(filepath.Join(prodDir, prod+".md"), r(tplProductPage)); err != nil {
		return err
	}

	if err := registerInIndex(
		filepath.Join(coDir, "index.md"),
		"- "+prod,
		fmt.Sprintf("- %s: [[%s/index]]", prod, prod),
		fmt.Sprintf("- %s: [[%s/%s]]", prod, prod, prod),
		"## Products",
	); err != nil {
		return err
	}

	fmt.Fprintf(out, "  ✓ product skeleton: %s\n", prodDir)
	return nil
}

// BootstrapRepo creates the repo memory pages and the three knowledge
// symlinks inside the checkout.
func BootstrapRepo(co, prod, repo string, out io.Writer) error {
	projects, err := ProjectsRoot()
	if err != nil {
		return err
	}
	workRoot, err := world.Root()
	if err != nil {
		return err
	}

	repoPath := filepath.Join(workRoot, co, prod, repo)
	coDir := filepath.Join(projects, co)
	prodDir := filepath.Join(coDir, prod)
	repoMem := filepath.Join(prodDir, "repos", repo)

	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		return fmt.Errorf("%s не git-репозиторий — пропущен", repoPath)
	}
	if err := ensureDirs(repoMem); err != nil {
		return err
	}

	r := func(tpl string) string {
		return render(tpl, "{co}", co, "{prod}", prod, "{repo}", repo, "{repoPath}", repoPath)
	}
	ensured := map[string]string{
		filepath.Join(repoMem, "index.md"):             r(tplRepoIndex),
		filepath.Join(repoMem, "_inbox.md"):            r(tplRepoInbox),
		filepath.Join(repoMem, "hot.md"):               r(tplRepoHot),
		filepath.Join(repoMem, "lessons.md"):           r(tplRepoLessons),
		filepath.Join(repoMem, "debugging-stories.md"): r(tplRepoDebugging),
		filepath.Join(repoMem, "gotchas.md"):           r(tplRepoGotchas),
		filepath.Join(repoMem, "decisions-not-adr.md"): r(tplRepoDecisions),
		filepath.Join(repoMem, "open-questions.md"):    r(tplRepoOpenQuestions),
		filepath.Join(repoMem, "links.md"):             r(tplRepoLinks),
	}
	for path, content := range ensured {
		if err := ensureFile(path, content); err != nil {
			return err
		}
	}
	if err := writeManagedFile(filepath.Join(repoMem, repo+".md"), r(tplRepoPage)); err != nil {
		return err
	}

	if err := ensureDirs(filepath.Join(repoPath, "docs")); err != nil {
		return err
	}
	links := map[string]string{
		filepath.Join(repoPath, "docs", "knowledge"):         repoMem,
		filepath.Join(repoPath, "docs", "product-knowledge"): prodDir,
		filepath.Join(repoPath, "docs", "company-knowledge"): coDir,
	}
	for link, target := range links {
		if err := safeLink(target, link); err != nil {
			return err
		}
	}
	for _, line := range []string{"docs/knowledge", "docs/product-knowledge", "docs/company-knowledge"} {
		if err := ensureExcludeLine(repoPath, line); err != nil {
			return err
		}
	}

	if err := registerInIndex(
		filepath.Join(prodDir, "index.md"),
		fmt.Sprintf("- %s:", repo),
		fmt.Sprintf("- %s: [[repos/%s/index]]", repo, repo),
		fmt.Sprintf("- %s: [[repos/%s/%s]]", repo, repo, repo),
		"## Repos",
	); err != nil {
		return err
	}

	fmt.Fprintf(out, "  ✓ repo: %s (%s)\n", repo, repoPath)
	return nil
}

// SanityCheck verifies the symlinks, the privacy boundary and the git
// exclusion of one bootstrapped repo.
func SanityCheck(co, prod, repo string, out io.Writer) {
	projects, err := ProjectsRoot()
	if err != nil {
		return
	}
	workRoot, err := world.Root()
	if err != nil {
		return
	}
	repoPath := filepath.Join(workRoot, co, prod, repo)
	coDir := filepath.Join(projects, co)

	fmt.Fprintf(out, "\n─── Sanity check: %s ───\n", repo)
	for _, link := range []string{"docs/knowledge", "docs/product-knowledge", "docs/company-knowledge"} {
		dest, err := os.Readlink(filepath.Join(repoPath, link))
		if err != nil {
			fmt.Fprintf(out, "  ✗ %s отсутствует или не симлинк\n", link)
			continue
		}
		fmt.Fprintf(out, "  ✓ %s → %s\n", link, dest)
	}

	companyTarget, _ := os.Readlink(filepath.Join(repoPath, "docs", "company-knowledge"))
	if companyTarget == coDir {
		fmt.Fprintf(out, "  ✓ privacy: остаётся в неймспейсе %s\n", co)
	} else {
		fmt.Fprintf(out, "  ✗ privacy: company-knowledge указывает вне ожидаемой компании\n")
	}

	status, err := gitOut(repoPath, "status", "--short")
	leaked := false
	if err == nil {
		for _, needle := range []string{"docs/knowledge", "docs/product-knowledge", "docs/company-knowledge"} {
			if strings.Contains(status, needle) {
				leaked = true
			}
		}
	}
	if leaked {
		fmt.Fprintln(out, "  ✗ симлинки НЕ исключены из git")
	} else {
		fmt.Fprintln(out, "  ✓ симлинки исключены из git")
	}
}
