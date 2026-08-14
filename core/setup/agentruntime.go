package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The agent runtime: SessionStart hooks, curator skills and profile settings.
// This used to live in install-wiki-runtime.sh and an inline python block of
// bootstrap.sh — machine state applied by scripts that no checker knew about.

// curatorSkills are the wiki curator skills, linked into the codex profile.
var curatorSkills = []string{
	"source-ingest", "agent-history-ingest", "inbox-drain",
	"wiki-synthesize", "wiki-lint", "wiki-status", "autoresearch",
}

// hookSources maps each profile to its SessionStart hook source.
var hookSources = map[string]string{
	".claude": "auto-load-claude.sh",
	".codex":  "auto-load-codex.sh",
}

func wikiHooksStep() Step {
	return Step{
		Name:  "session-hooks",
		About: "SessionStart-хуки обоих профилей ведут на хуки из репо",
		check: func(env Env) Result {
			var results []Result
			for profile, source := range hookSources {
				src := filepath.Join(env.Dotfiles, "skills-stash", "wiki", "hooks", source)
				dst := filepath.Join(env.Home, profile, "hooks", "SessionStart.sh")
				results = append(results, checkSymlink(src, dst))
			}
			return combine(results, "хуки на месте")
		},
		apply: func(env Env) error {
			for profile, source := range hookSources {
				src := filepath.Join(env.Dotfiles, "skills-stash", "wiki", "hooks", source)
				dst := filepath.Join(env.Home, profile, "hooks", "SessionStart.sh")
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

// codexZoneProfiles are per-zone codex config layers: `codex -p <zone>` loads
// ~/.codex/<zone>.config.toml on top of the base config. They exist to keep
// tools a zone never uses out of its prompt.
var codexZoneProfiles = map[string]string{
	"research": "codex-research.config.toml",
}

func codexProfilesStep() Step {
	return Step{
		Name:  "codex-profiles",
		About: "профили зон codex прилинкованы (codex -p <зона>)",
		check: func(env Env) Result {
			var results []Result
			for zone, source := range codexZoneProfiles {
				src := filepath.Join(env.Dotfiles, "agent-profiles", source)
				dst := filepath.Join(env.Home, ".codex", zone+".config.toml")
				results = append(results, checkSymlink(src, dst))
			}
			return combine(results, "профили зон на месте")
		},
		apply: func(env Env) error {
			for zone, source := range codexZoneProfiles {
				src := filepath.Join(env.Dotfiles, "agent-profiles", source)
				dst := filepath.Join(env.Home, ".codex", zone+".config.toml")
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

func claudeHookPath(env Env) string {
	return filepath.Join(env.Home, ".claude", "hooks", "SessionStart.sh")
}

func claudeSettingsStep() Step {
	return Step{
		Name:  "claude-settings",
		About: "settings.json: плагины, маркетплейсы, SessionStart-хук, без co-authored-by",
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
			if !hasHookEntry(data, claudeHookPath(env)) {
				missingBits = append(missingBits, "SessionStart-хук")
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

			hookPath := claudeHookPath(env)
			if !hasHookEntry(data, hookPath) {
				hooks := ensureMap(data, "hooks")
				items, _ := hooks["SessionStart"].([]any)
				items = append(items, map[string]any{
					"matcher": "*",
					"hooks":   []any{map[string]any{"type": "command", "command": hookPath}},
				})
				hooks["SessionStart"] = items
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

func hasHookEntry(data map[string]any, command string) bool {
	hooks, _ := data["hooks"].(map[string]any)
	items, _ := hooks["SessionStart"].([]any)
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

func codexConfigPath(env Env) string {
	return filepath.Join(env.Home, ".codex", "config.toml")
}

func codexHookPath(env Env) string {
	return filepath.Join(env.Home, ".codex", "hooks", "SessionStart.sh")
}

// codexConfigStep keeps the codex hook and vault trust entries in
// config.toml. The file is managed textually — codex owns its format, this
// step only appends what is missing and refuses ambiguous merges.
func codexConfigStep() Step {
	return Step{
		Name:  "codex-config",
		About: "config.toml: SessionStart-хук и trust для зон вольта",
		check: func(env Env) Result {
			data, err := os.ReadFile(codexConfigPath(env))
			if err != nil {
				return missing("нет %s", short(codexConfigPath(env)))
			}
			content := string(data)

			var missingBits []string
			if !strings.Contains(content, codexHookPath(env)) {
				missingBits = append(missingBits, "SessionStart-хук")
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

			if !strings.Contains(content, codexHookPath(env)) {
				if strings.Contains(content, "[hooks]") {
					return fmt.Errorf("%s уже содержит [hooks] без нашего хука — добавь вручную:\n"+
						"SessionStart = [{ matcher = \"startup|resume\", hooks = [{ type = \"command\", command = %q }] }]",
						path, codexHookPath(env))
				}
				content += fmt.Sprintf("\n[hooks]\nSessionStart = [\n"+
					"  { matcher = \"startup|resume\", hooks = [{ type = \"command\", command = %q }] },\n]\n",
					codexHookPath(env))
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
