// Package skill manages repo-owned agent skills.
//
// Skills live in ~/dotfiles/skills/<name>/SKILL.md and are symlinked into
// both runtime profiles — the tools' own default paths. The earlier
// setup/daily/wiki split existed to keep five profiles apart; with two it was
// only a way to forget a skill somewhere, so every skill goes everywhere.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Paths locates the skill source and the profile targets. The environment
// overrides mirror the bash tool this replaces, and keep tests hermetic.
type Paths struct {
	Source  string
	Targets []string
}

// DefaultPaths resolves the standard layout, honouring DOTFILES,
// CLAUDE_SKILLS and CODEX_SKILLS overrides.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}

	dotfiles := os.Getenv("DOTFILES")
	if dotfiles == "" {
		dotfiles = filepath.Join(home, "dotfiles")
	}
	claude := os.Getenv("CLAUDE_SKILLS")
	if claude == "" {
		claude = filepath.Join(home, ".claude", "skills")
	}
	codex := os.Getenv("CODEX_SKILLS")
	if codex == "" {
		codex = filepath.Join(home, ".codex", "skills")
	}

	return Paths{
		Source:  filepath.Join(dotfiles, "skills"),
		Targets: []string{claude, codex},
	}, nil
}

// LinkStatus is the state of one skill link in one profile.
type LinkStatus string

const (
	StatusLinked       LinkStatus = "linked"
	StatusLinkedOther  LinkStatus = "linked-other"
	StatusExistsNotLnk LinkStatus = "exists-not-link"
	StatusMissing      LinkStatus = "missing"
)

// Info is one repo-owned skill and its install state per target.
type Info struct {
	Name   string
	Dir    string
	Status map[string]LinkStatus
}

// Dirs lists the repo-owned skill directories, sorted by name.
func Dirs(p Paths) ([]string, error) {
	entries, err := os.ReadDir(p.Source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("чтение %s: %w", p.Source, err)
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(p.Source, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// List reports every skill with its per-profile link status.
func List(p Paths) ([]Info, error) {
	dirs, err := Dirs(p)
	if err != nil {
		return nil, err
	}

	var out []Info
	for _, dir := range dirs {
		name := filepath.Base(dir)
		info := Info{Name: name, Dir: dir, Status: map[string]LinkStatus{}}
		for _, target := range p.Targets {
			info.Status[target] = linkStatus(filepath.Join(target, name), dir)
		}
		out = append(out, info)
	}
	return out, nil
}

func linkStatus(link, want string) LinkStatus {
	info, err := os.Lstat(link)
	if err != nil {
		return StatusMissing
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return StatusExistsNotLnk
	}
	actual, err := os.Readlink(link)
	if err != nil || actual != want {
		return StatusLinkedOther
	}
	return StatusLinked
}

var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// New creates the SKILL.md template for a new skill and returns its path.
func New(p Paths, name string) (string, error) {
	if !nameRe.MatchString(name) {
		return "", fmt.Errorf("имя скилла — lowercase kebab-case: [a-z0-9-]")
	}

	dir := filepath.Join(p.Source, name)
	file := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(file); err == nil {
		return "", fmt.Errorf("скилл уже существует: %s", file)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("создание %s: %w", dir, err)
	}

	template := fmt.Sprintf(`---
name: %s
description: "Use when the task involves <specific trigger words and workflow scope>."
---

# %s

Use this skill when <when to use>.

## Workflow

1. <step>
2. <step>
3. <step>
`, name, name)

	if err := os.WriteFile(file, []byte(template), 0o644); err != nil {
		return "", fmt.Errorf("запись %s: %w", file, err)
	}
	return file, nil
}

// Install symlinks every skill into every target and returns the linked names.
func Install(p Paths) ([]string, error) {
	dirs, err := Dirs(p)
	if err != nil {
		return nil, err
	}

	for _, target := range p.Targets {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return nil, fmt.Errorf("создание %s: %w", target, err)
		}
	}

	var linked []string
	for _, dir := range dirs {
		name := filepath.Base(dir)
		for _, target := range p.Targets {
			link := filepath.Join(target, name)
			// ln -sfn: replace whatever link is there; refuse to clobber a
			// real directory.
			if info, err := os.Lstat(link); err == nil {
				if info.Mode()&os.ModeSymlink == 0 {
					return nil, fmt.Errorf("не ссылка, не трогаю: %s", link)
				}
				if err := os.Remove(link); err != nil {
					return nil, fmt.Errorf("замена %s: %w", link, err)
				}
			}
			if err := os.Symlink(dir, link); err != nil {
				return nil, fmt.Errorf("линк %s: %w", link, err)
			}
		}
		linked = append(linked, name)
	}
	return linked, nil
}

// descriptionRe requires a quoted description, so a skill never ships with an
// unquoted YAML scalar that breaks on the first colon.
var descriptionRe = regexp.MustCompile(`(?m)^description:[ \t]+".+"[ \t]*$`)

// Doctor validates every skill and every profile link. It returns the list of
// problems; an empty list is a clean bill.
func Doctor(p Paths) ([]string, error) {
	infos, err := List(p)
	if err != nil {
		return nil, err
	}

	var problems []string
	for _, info := range infos {
		if err := validate(filepath.Join(info.Dir, "SKILL.md"), info.Name); err != nil {
			problems = append(problems, err.Error())
		}
		for _, target := range p.Targets {
			if info.Status[target] != StatusLinked {
				problems = append(problems,
					fmt.Sprintf("не прилинкован в %s: %s (%s)", target, info.Name, info.Status[target]))
			}
		}
	}

	problems = append(problems, orphans(p)...)
	return problems, nil
}

// validate checks the SKILL.md frontmatter contract.
func validate(file, name string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("чтение %s: %w", file, err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return fmt.Errorf("%s: нет открывающего frontmatter", file)
	}

	closing := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			closing = i
			break
		}
	}
	if closing == -1 {
		return fmt.Errorf("%s: нет закрывающего frontmatter", file)
	}

	frontmatter := strings.Join(lines[1:closing], "\n")
	nameLine := regexp.MustCompile(`(?m)^name:[ \t]*` + regexp.QuoteMeta(name) + `$`)
	if !nameLine.MatchString(frontmatter) {
		return fmt.Errorf("%s: name должен совпадать с каталогом (%s)", file, name)
	}
	if !descriptionRe.MatchString(frontmatter) {
		return fmt.Errorf("%s: description должен быть в кавычках", file)
	}
	return nil
}

// orphans finds links in the profiles whose source is gone — the failure a
// repo-only scan can never see, which is how a dangling skill link once
// survived for weeks while doctor reported ok.
func orphans(p Paths) []string {
	var problems []string
	for _, target := range p.Targets {
		entries, err := os.ReadDir(target)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			link := filepath.Join(target, entry.Name())
			info, err := os.Lstat(link)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			if _, err := os.Stat(link); err != nil {
				dest, _ := os.Readlink(link)
				problems = append(problems, fmt.Sprintf("висячая ссылка: %s -> %s", link, dest))
			}
		}
	}
	return problems
}
