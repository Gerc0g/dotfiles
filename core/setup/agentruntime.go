package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/wiki"
	"github.com/Gerc0g/dotfiles/core/workspace"
)

// The agent runtime: SessionStart hooks, curator skills and profile settings.
// This used to live in install-wiki-runtime.sh and an inline python block of
// bootstrap.sh — machine state applied by scripts that no checker knew about.

// curatorSkills are the wiki curator skills, linked into the codex profile.
var curatorSkills = []string{
	"source-ingest", "agent-history-ingest", "inbox-drain",
	"wiki-synthesize", "wiki-lint", "wiki-status", "autoresearch",
}

// profileHooks maps each agent profile to the hooks it runs: SessionStart
// injects memory, SessionEnd hands the inbox to the curator. Draining at the
// end of a session is what keeps captured lessons from piling up unread —
// 268 of them had accumulated before this existed.
var profileHooks = map[string]map[string]string{
	".claude": {
		"SessionStart.sh": "auto-load-claude.sh",
		"SessionEnd.sh":   "memory-drain-claude.sh",
	},
	".codex": {
		"SessionStart.sh": "auto-load-codex.sh",
		"SessionEnd.sh":   "memory-drain-codex.sh",
	},
}

func wikiHooksStep() Step {
	return Step{
		Name:  "session-hooks",
		About: "хуки памяти обоих профилей ведут на хуки из репо",
		check: func(env Env) Result {
			var results []Result
			for profile, hooks := range profileHooks {
				for hook, source := range hooks {
					src := filepath.Join(env.Dotfiles, "skills-stash", "wiki", "hooks", source)
					dst := filepath.Join(env.Home, profile, "hooks", hook)
					results = append(results, checkSymlink(src, dst))
				}
			}
			return combine(results, "хуки на месте")
		},
		apply: func(env Env) error {
			for profile, hooks := range profileHooks {
				for hook, source := range hooks {
					src := filepath.Join(env.Dotfiles, "skills-stash", "wiki", "hooks", source)
					dst := filepath.Join(env.Home, profile, "hooks", hook)
					if err := ensureDir(filepath.Dir(dst)); err != nil {
						return err
					}
					if err := ensureSymlink(src, dst); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
}

func curatorSkillsStep() Step {
	return Step{
		Name:  "curator-skills",
		About: "кураторские wiki-скиллы прилинкованы в профиль codex",
		check: func(env Env) Result {
			var results []Result
			for _, skill := range curatorSkills {
				src := filepath.Join(env.Dotfiles, "skills-stash", "wiki", "curator", skill)
				dst := filepath.Join(env.Home, ".codex", "skills", skill)
				results = append(results, checkSymlink(src, dst))
			}
			return combine(results, fmt.Sprintf("%d скиллов прилинковано", len(curatorSkills)))
		},
		apply: func(env Env) error {
			for _, skill := range curatorSkills {
				src := filepath.Join(env.Dotfiles, "skills-stash", "wiki", "curator", skill)
				dst := filepath.Join(env.Home, ".codex", "skills", skill)
				if err := ensureDir(filepath.Dir(dst)); err != nil {
					return err
				}
				if err := ensureSymlink(src, dst); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// codexZones are the codex profiles isolated by the kind of work they serve.
// The default ~/.codex owns code and project memory; a zone profile owns one
// semantic area and only carries its own skills, hooks and history.
//
// Isolation has to be a separate CODEX_HOME: the profile layer (`codex -p`)
// overrides settings but not the skills directory, so a zone would still load
// every skill of the default profile into every prompt.
var codexZones = []string{"research"}

// codexShared are the entries a zone profile symlinks back to the default
// one: one login for both, and no second copy of the plugin cache.
var codexShared = []string{"auth.json", "plugins"}

// codexZoneHooks maps a zone profile's hook file to its source in the repo.
// Stop commits the vault after every answer — the mechanical duplicate of the
// contract rule the model skips.
// Committing is the agent's job — only it knows what a change means, and a
// per-edit hook would always beat it to the commit, leaving every message
// machine-made. The hooks stay as the safety net at the end of a turn and a
// session, so a forgotten commit costs a worse message, never lost work.
var codexZoneHooks = map[string]string{
	"SessionStart.sh": "auto-load-codex.sh",
	"Stop.sh":         "vault-autosync.sh",
	"SessionEnd.sh":   "vault-autosync.sh",
}

func codexHome(env Env, zone string) string {
	return filepath.Join(env.Home, ".codex-"+zone)
}

func codexZoneProfilesStep() Step {
	return Step{
		Name:  "codex-zones",
		About: "изолированные профили codex по зонам (свои скиллы, общий логин)",
		check: func(env Env) Result {
			var results []Result
			for _, zone := range codexZones {
				home := codexHome(env, zone)

				results = append(results, checkDir(filepath.Join(home, "skills")))
				for _, name := range codexShared {
					src := filepath.Join(env.Home, ".codex", name)
					// auth.json appears only after `codex login`, which is a
					// manual step: report, do not fail the machine.
					if _, err := os.Stat(src); err != nil {
						results = append(results, skipped("нет %s — сначала codex login", short(src)))
						continue
					}
					results = append(results, checkSymlink(src, filepath.Join(home, name)))
				}
				for hook, source := range codexZoneHooks {
					results = append(results, checkSymlink(
						filepath.Join(env.Dotfiles, "skills-stash", "wiki", "hooks", source),
						filepath.Join(home, "hooks", hook)))
				}

				if _, err := os.Stat(filepath.Join(home, "config.toml")); err != nil {
					results = append(results, missing("нет %s", short(filepath.Join(home, "config.toml"))))
				}
			}
			return combine(results, "профили зон на месте")
		},
		apply: func(env Env) error {
			for _, zone := range codexZones {
				home := codexHome(env, zone)
				if err := ensureDir(filepath.Join(home, "skills")); err != nil {
					return err
				}
				for _, name := range codexShared {
					src := filepath.Join(env.Home, ".codex", name)
					if _, err := os.Stat(src); err != nil {
						continue
					}
					if err := ensureSymlink(src, filepath.Join(home, name)); err != nil {
						return err
					}
				}
				for hook, source := range codexZoneHooks {
					if err := ensureSymlink(
						filepath.Join(env.Dotfiles, "skills-stash", "wiki", "hooks", source),
						filepath.Join(home, "hooks", hook)); err != nil {
						return err
					}
				}
				if err := seedZoneConfig(env, zone, home); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// seedZoneConfig copies the profile template once. It is never rewritten:
// codex itself appends trust levels and hook hashes to this file, so an
// overwriting step would silently discard the user's own trust decisions.
func seedZoneConfig(env Env, zone, home string) error {
	dst := filepath.Join(home, "config.toml")
	if _, err := os.Stat(dst); err == nil {
		return nil
	}

	src := filepath.Join(env.Dotfiles, "agent-profiles", "codex-"+zone, "config.toml")
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("чтение шаблона %s: %w", src, err)
	}
	rendered := strings.ReplaceAll(string(data), "{{HOME}}", env.Home)
	if err := os.WriteFile(dst, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("запись %s: %w", dst, err)
	}
	return nil
}

// claudePlugins is the intentionally small plugin set of the Claude profile.
var claudePlugins = []string{
	"pyright-lsp@claude-plugins-official",
	"vtsls@claude-code-lsps",
	"yaml-language-server@claude-code-lsps",
}

var claudeMarketplaces = map[string]string{
	"claude-plugins-official": "anthropics/claude-plugins-official",
	"claude-code-lsps":        "boostvolt/claude-code-lsps",
}

func claudeSettingsPath(env Env) string {
	return filepath.Join(env.Home, ".claude", "settings.json")
}

// sessionEvents maps each hook event to the script it runs; both profiles
// install the same pair.
var sessionEvents = map[string]string{
	"SessionStart": "SessionStart.sh",
	"SessionEnd":   "SessionEnd.sh",
}

func claudeHookPath(env Env, script string) string {
	return filepath.Join(env.Home, ".claude", "hooks", script)
}

func claudeSettingsStep() Step {
	return Step{
		Name:  "claude-settings",
		About: "settings.json: плагины, маркетплейсы, хуки сессии, без co-authored-by",
		check: func(env Env) Result {
			data, err := readSettings(claudeSettingsPath(env))
			if err != nil {
				return drifted("%v", err)
			}
			var missingBits []string
			if v, ok := data["includeCoAuthoredBy"].(bool); !ok || v {
				missingBits = append(missingBits, "includeCoAuthoredBy=false")
			}
			enabled, _ := data["enabledPlugins"].(map[string]any)
			for _, plugin := range claudePlugins {
				if v, ok := enabled[plugin].(bool); !ok || !v {
					missingBits = append(missingBits, plugin)
				}
			}
			for event, script := range sessionEvents {
				if !hasHookEntry(data, event, claudeHookPath(env, script)) {
					missingBits = append(missingBits, event+"-хук")
				}
			}
			if len(missingBits) > 0 {
				return missing("нет: %s", strings.Join(missingBits, ", "))
			}
			return ok("настройки на месте")
		},
		apply: func(env Env) error {
			path := claudeSettingsPath(env)
			data, err := readSettings(path)
			if err != nil {
				return err
			}

			data["includeCoAuthoredBy"] = false

			enabled := ensureMap(data, "enabledPlugins")
			for _, plugin := range claudePlugins {
				enabled[plugin] = true
			}
			marketplaces := ensureMap(data, "extraKnownMarketplaces")
			for name, repo := range claudeMarketplaces {
				marketplaces[name] = map[string]any{
					"source": map[string]any{"source": "github", "repo": repo},
				}
			}

			for event, script := range sessionEvents {
				hookPath := claudeHookPath(env, script)
				if !hasHookEntry(data, event, hookPath) {
					addHookEntry(data, event, hookPath)
				}
			}

			if err := ensureDir(filepath.Dir(path)); err != nil {
				return err
			}
			encoded, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				return err
			}
			return os.WriteFile(path, append(encoded, '\n'), 0o644)
		},
	}
}

// readSettings loads settings.json as a generic map; a missing file is an
// empty config, invalid JSON is an error so apply never destroys user edits.
func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("невалидный JSON в %s — не трогаю: %w", path, err)
	}
	return out, nil
}

func ensureMap(data map[string]any, key string) map[string]any {
	if m, ok := data[key].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	data[key] = m
	return m
}

func hasHookEntry(data map[string]any, event, command string) bool {
	hooks, _ := data["hooks"].(map[string]any)
	items, _ := hooks[event].([]any)
	for _, item := range items {
		entry, _ := item.(map[string]any)
		nested, _ := entry["hooks"].([]any)
		for _, h := range nested {
			hook, _ := h.(map[string]any)
			if hook["command"] == command {
				return true
			}
		}
	}
	return false
}

// addHookEntry registers command under event, leaving any hooks the user
// added by hand untouched.
func addHookEntry(data map[string]any, event, command string) {
	hooks := ensureMap(data, "hooks")
	items, _ := hooks[event].([]any)
	items = append(items, map[string]any{
		"matcher": "*",
		"hooks":   []any{map[string]any{"type": "command", "command": command}},
	})
	hooks[event] = items
}

func codexConfigPath(env Env) string {
	return filepath.Join(env.Home, ".codex", "config.toml")
}

func codexHookPath(env Env, script string) string {
	return filepath.Join(env.Home, ".codex", "hooks", script)
}

// insertCodexHook adds one event line to config.toml. The file belongs to
// codex and already carries hooks from other tools, so the edit is textual and
// minimal: a new key right under the [hooks] header (TOML keys after a header
// belong to that table), or a fresh [hooks] table when there is none.
func insertCodexHook(content, event, command string) string {
	line := fmt.Sprintf("%s = [\n  { matcher = \"*\", hooks = [{ type = \"command\", command = %q }] },\n]\n",
		event, command)

	header := "[hooks]\n"
	idx := strings.Index(content, header)
	if idx < 0 {
		if !strings.HasSuffix(content, "\n") && content != "" {
			content += "\n"
		}
		return content + "\n[hooks]\n" + line
	}
	at := idx + len(header)
	return content[:at] + line + content[at:]
}

// worktreeMemoryStep keeps project memory reachable from worktrees.
//
// The links are untracked, so a worktree only has them if something puts them
// there. Start does that now, but the pool predates it: work happens in
// worktrees, and in most of them memory was invisible in both directions.
func worktreeMemoryStep() Step {
	return Step{
		Name:  "worktree-memory",
		About: "воркспейсы видят память своего репо",
		check: func(env Env) Result {
			gaps, err := memoryGaps(env)
			if err != nil {
				return skipped("%v", err)
			}
			if len(gaps) > 0 {
				return missing("%d воркспейсов без памяти", len(gaps))
			}
			return ok("все воркспейсы связаны")
		},
		apply: func(env Env) error {
			manager, err := workspace.New(env.Workspace)
			if err != nil {
				return err
			}
			_, err = manager.RelinkMemory()
			return err
		},
	}
}

func memoryGaps(env Env) ([]workspace.MemoryGap, error) {
	if _, err := os.Stat(env.Workspace); err != nil {
		return nil, fmt.Errorf("нет %s", short(env.Workspace))
	}
	manager, err := workspace.New(env.Workspace)
	if err != nil {
		return nil, err
	}
	return manager.MemoryGaps()
}

// cronPulseStep reports whether the scheduled drain is actually running.
//
// The job needs Full Disk Access to reach a vault under ~/Desktop, and that
// grant is keyed to the binary — `make build` rewrites it, so macOS can revoke
// the permission without saying anything. The run records its own outcome
// precisely so that revocation shows up here instead of as months of silence.
func cronPulseStep() Step {
	return Step{
		Name:  "cron-pulse",
		About: "плановый разбор доходит до вольта",
		check: func(env Env) Result {
			if runtime.GOOS != "darwin" {
				return skipped("launchd есть только в macOS")
			}
			last, outcome := wiki.LastCron(env.Home)
			switch {
			case last.IsZero():
				return drifted("не запускался ещё ни разу")
			case outcome == wiki.CronDenied:
				return drifted("macOS не пустил к вольту — выдай Full Disk Access для %s",
					short(filepath.Join(env.Dotfiles, "bin", "hq")))
			case time.Since(last) > wiki.CronStale:
				return drifted("молчит %d ч. — задача в launchd жива?",
					int(time.Since(last).Hours()))
			}
			return ok("последний прогон %s", last.Format("02.01 15:04"))
		},
	}
}

// hookPulseStep reports whether the session hooks are actually firing.
//
// Nothing else notices when they stop. Codex revokes hook trust whenever the
// script changes and says nothing; the config keeps its trust entry with a
// stale hash, so checking the config reports health that is not there. Only
// the firing itself is evidence, and this step has no apply — a human has to
// re-trust the hook in the agent.
func hookPulseStep() Step {
	return Step{
		Name:  "hook-pulse",
		About: "хуки сессии реально срабатывали за последние дни",
		check: func(env Env) Result {
			var cold []string
			for profile := range profileHooks {
				agent := strings.TrimPrefix(profile, ".")
				for _, event := range []string{"session-start", "session-end"} {
					last := wiki.LastHook(env.Home, agent, event)
					switch {
					case last.IsZero():
						cold = append(cold, fmt.Sprintf("%s/%s ни разу", agent, event))
					case time.Since(last) > wiki.HookStale:
						cold = append(cold, fmt.Sprintf("%s/%s молчит %d дн.",
							agent, event, int(time.Since(last).Hours()/24)))
					}
				}
			}
			if len(cold) > 0 {
				sort.Strings(cold)
				return drifted("%s — проверь доверие к хукам в агенте", strings.Join(cold, ", "))
			}
			return ok("все хуки срабатывали")
		},
	}
}

// codexConfigStep keeps the codex hook and vault trust entries in
// config.toml. The file is managed textually — codex owns its format, this
// step only appends what is missing and refuses ambiguous merges.
func codexConfigStep() Step {
	return Step{
		Name:  "codex-config",
		About: "config.toml: хуки сессии и trust для зон вольта",
		check: func(env Env) Result {
			data, err := os.ReadFile(codexConfigPath(env))
			if err != nil {
				return missing("нет %s", short(codexConfigPath(env)))
			}
			content := string(data)

			var missingBits []string
			for event, script := range sessionEvents {
				if !strings.Contains(content, codexHookPath(env, script)) {
					missingBits = append(missingBits, event+"-хук")
				}
			}
			for _, zone := range vaultTrustZones(env) {
				if !strings.Contains(content, fmt.Sprintf("[projects.%q]", zone)) {
					missingBits = append(missingBits, "trust "+filepath.Base(zone))
				}
			}
			if len(missingBits) > 0 {
				return missing("нет: %s", strings.Join(missingBits, ", "))
			}
			return ok("конфиг на месте")
		},
		apply: func(env Env) error {
			path := codexConfigPath(env)
			data, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				data = nil
			} else if err != nil {
				return err
			}
			content := string(data)

			for event, script := range sessionEvents {
				hookPath := codexHookPath(env, script)
				if strings.Contains(content, hookPath) {
					continue
				}
				if strings.Contains(content, event+" = [") {
					return fmt.Errorf("%s уже содержит %s без нашего хука — добавь вручную:\n"+
						"{ matcher = \"*\", hooks = [{ type = \"command\", command = %q }] }",
						path, event, hookPath)
				}
				content = insertCodexHook(content, event, hookPath)
			}

			for _, zone := range vaultTrustZones(env) {
				header := fmt.Sprintf("[projects.%q]", zone)
				if !strings.Contains(content, header) {
					content += fmt.Sprintf("\n%s\ntrust_level = \"trusted\"\n", header)
				}
			}

			if err := ensureDir(filepath.Dir(path)); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(content), 0o644)
		},
	}
}

// vaultTrustZones are the vault directories codex sessions must trust.
func vaultTrustZones(env Env) []string {
	return []string{
		filepath.Join(env.Vault, "dev"),
		filepath.Join(env.Vault, "research"),
		filepath.Join(env.Vault, "Personal Brand"),
		env.Vault,
	}
}
