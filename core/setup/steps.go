package setup

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Gerc0g/dotfiles/core/internal/ui"
)

// agentProfiles are the runtime config directories the platform manages.
//
// These are the tools' own default paths, on purpose. Selecting a profile with
// CODEX_HOME / CLAUDE_CONFIG_DIR only works where those variables are set — a
// terminal — so a GUI-launched VS Code fell back to the defaults and picked up
// a different, unmanaged setup. Owning the defaults makes the managed profile
// the one every caller sees: CLI, editor extension, launchd, anything later.
var agentProfiles = []string{
	".claude",
	".codex",
}

// baselineTargets maps each profile to the filename its agent reads. Codex
// reads AGENTS.md, Claude reads CLAUDE.md, and both point at the same file —
// that single source is why an instruction written once reaches both agents.
var baselineTargets = map[string]string{
	".claude": "CLAUDE.md",
	".codex":  "AGENTS.md",
}

const loaderNeedle = "dotfiles/shell/_loader.zsh"

const loaderBlock = "\n# === Personal Platform ===\n" +
	"[ -f ~/dotfiles/shell/_loader.zsh ] && source ~/dotfiles/shell/_loader.zsh\n"

// Plan returns the declared machine state, in apply order.
func Plan() []Step {
	return []Step{
		workspaceRootStep(),
		vaultStep(),
		agentProfilesStep(),
		baselineProfilesStep(),
		configLinksStep(),
		shellLoaderStep(),
		transcriptScrubStep(),
		coreBinaryStep(),
		skillLinksStep(),
		wikiHooksStep(),
		curatorSkillsStep(),
		codexZoneProfilesStep(),
		claudeSettingsStep(),
		codexConfigStep(),
		worktreeMemoryStep(),
		hookPulseStep(),
	}
}

func workspaceRootStep() Step {
	return Step{
		Name:  "workspace-root",
		About: "корень компаний, продуктов и репозиториев существует",
		check: func(env Env) Result { return checkDir(env.Workspace) },
		apply: func(env Env) error { return ensureDir(env.Workspace) },
	}
}

func agentProfilesStep() Step {
	return Step{
		Name:  "agent-profiles",
		About: "каталоги конфигов Codex и Claude на месте",
		check: func(env Env) Result {
			var results []Result
			for _, profile := range agentProfiles {
				results = append(results, checkDir(filepath.Join(env.Home, profile)))
			}
			return combine(results, ui.Plural(len(results), "каталог профиля", "каталога профилей", "каталогов профилей"))
		},
		apply: func(env Env) error {
			for _, profile := range agentProfiles {
				if err := ensureDir(filepath.Join(env.Home, profile)); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func baselineProfilesStep() Step {
	return Step{
		Name:  "baseline-profile",
		About: "все профили агентов читают agent-profiles/BASELINE.md",
		check: func(env Env) Result {
			src := baselinePath(env)
			var results []Result
			for _, profile := range sortedProfiles() {
				dst := filepath.Join(env.Home, profile, baselineTargets[profile])
				results = append(results, checkSymlink(src, dst))
			}
			return combine(results, ui.Plural(len(results), "профиль читает", "профиля читают", "профилей читают")+" BASELINE.md")
		},
		apply: func(env Env) error {
			src := baselinePath(env)
			for _, profile := range sortedProfiles() {
				dst := filepath.Join(env.Home, profile, baselineTargets[profile])
				if err := ensureSymlink(src, dst); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func baselinePath(env Env) string {
	return filepath.Join(env.Dotfiles, "agent-profiles", "BASELINE.md")
}

// configLinks are terminal and multiplexer configs that must live at fixed
// paths outside the repo.
func configLinks(env Env) [][2]string {
	return [][2]string{
		{
			filepath.Join(env.Dotfiles, "ghostty", "config"),
			filepath.Join(env.Home, ".config", "ghostty", "config"),
		},
		{
			filepath.Join(env.Dotfiles, "tmux", "tmux.conf"),
			filepath.Join(env.Home, ".tmux.conf"),
		},
	}
}

func configLinksStep() Step {
	return Step{
		Name:  "config-links",
		About: "конфиги ghostty и tmux ведут в репозиторий",
		check: func(env Env) Result {
			var results []Result
			for _, pair := range configLinks(env) {
				results = append(results, checkSymlink(pair[0], pair[1]))
			}
			return combine(results, ui.Plural(len(results), "конфиг связан", "конфига связаны", "конфигов связаны"))
		},
		apply: func(env Env) error {
			for _, pair := range configLinks(env) {
				if err := ensureSymlink(pair[0], pair[1]); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func shellLoaderStep() Step {
	return Step{
		Name:  "shell-loader",
		About: "~/.zshrc подключает модули оболочки",
		check: func(env Env) Result {
			return checkLineInFile(filepath.Join(env.Home, ".zshrc"), loaderNeedle)
		},
		apply: func(env Env) error {
			return appendBlock(filepath.Join(env.Home, ".zshrc"), loaderBlock)
		},
	}
}

// coreBinaryStep is check-only on purpose: hq cannot sensibly rebuild the
// binary it is currently running from.
func coreBinaryStep() Step {
	return Step{
		Name:  "core-binary",
		About: "bin/hq собран (сборка: make build)",
		check: func(env Env) Result {
			path := filepath.Join(env.Dotfiles, "bin", "hq")
			if _, err := os.Stat(path); err != nil {
				return missing("%s — соберите: make build", short(path))
			}
			return ok("%s", short(path))
		},
	}
}

// skillLinksStep catches the failure the old shell doctor could not see: a
// symlink in a profile whose source was deleted from the repo. It only reports
// — removing links this tool did not create is not its call.
func skillLinksStep() Step {
	return Step{
		Name:  "skill-links",
		About: "в профилях агентов нет висячих ссылок на скиллы",
		check: func(env Env) Result {
			var dangling []string

			for _, profile := range sortedProfiles() {
				dir := filepath.Join(env.Home, profile, "skills")
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}

				for _, entry := range entries {
					path := filepath.Join(dir, entry.Name())
					info, err := os.Lstat(path)
					if err != nil || info.Mode()&os.ModeSymlink == 0 {
						continue
					}
					if _, err := os.Stat(path); err != nil {
						dangling = append(dangling, short(path))
					}
				}
			}

			if len(dangling) > 0 {
				return drifted("ведут в никуда: %s", strings.Join(dangling, ", "))
			}
			return ok("висячих ссылок нет")
		},
	}
}

func sortedProfiles() []string {
	profiles := append([]string(nil), agentProfiles...)
	sort.Strings(profiles)
	return profiles
}

// combine folds per-target results into one. The worst status wins, and the
// detail of every failing target is kept so output stays actionable.
func combine(results []Result, okText string) Result {
	var failures []string
	worst := StatusOK

	for _, result := range results {
		if result.Status == StatusOK {
			continue
		}
		failures = append(failures, result.Detail)
		if worst == StatusOK || result.Status == StatusDrifted {
			worst = result.Status
		}
	}

	if len(failures) == 0 {
		return Result{Status: StatusOK, Detail: okText}
	}
	return Result{Status: worst, Detail: strings.Join(failures, "; ")}
}
