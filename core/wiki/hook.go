package wiki

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/world"
)

// SessionStart hooks inject the repo's hot.md into every agent session. The
// contract is best-effort by design: a hook must never break agent startup,
// so every failure here surfaces as "no output", not as an error.

// memoryCheckpoint is appended after the hot context — the discipline the
// agents follow around the memory loop.
const memoryCheckpoint = "\n\n# WikiPedik memory checkpoint\n\n" +
	"1. The hot context above is an INDEX, not the whole memory. Read `docs/knowledge/<page>.md` (lessons, gotchas, debugging-stories, open-questions) on demand when the task touches those topics; for nontrivial debug/design/migration work start with the `wiki-context-pack` skill.\n" +
	"2. Verify before apply: when acting on a remembered lesson that cites code locations, check the cited code first — if the code has changed and contradicts the lesson, capture a corrected version via `lesson-append` instead of applying the stale one.\n" +
	"3. Before the final answer, decide whether this task produced a durable root cause, production gotcha, failed approach, reusable rule, informal decision, or cross-repo invariant. If yes, use the `lesson-append` skill to append one concise candidate (with citations) to `docs/knowledge/_inbox.md`. If not, write nothing."

// ZoneContext is filled by the research package at init time, so the vault
// hook can carry zone state without wiki importing research (research already
// imports wiki for the vault root).
var ZoneContext = map[string]func() string{}

// SessionStartContext builds the hook JSON for an agent starting in cwd.
// Empty output means "emit nothing" — the silent exit of the hook contract.
func SessionStartContext(agent, cwd string) string {
	vaultRoot, err := VaultRoot()
	if err != nil {
		return ""
	}

	// Sessions inside the vault get vault discipline plus the state of the
	// zone they are in.
	if cwd == vaultRoot || strings.HasPrefix(cwd, vaultRoot+"/") {
		context := vaultCheckpoint(vaultRoot, cwd)
		if zone := zoneOf(vaultRoot, cwd); zone != "" {
			if build, ok := ZoneContext[zone]; ok {
				if extra := build(); extra != "" {
					context += "\n\n" + extra
				}
			}
		}
		return hookJSON(context)
	}

	workRoot, err := world.Root()
	if err != nil {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dotfiles := filepath.Join(home, "dotfiles")

	inProjects := strings.HasPrefix(cwd, workRoot+"/")
	inDotfiles := cwd == dotfiles || strings.HasPrefix(cwd, dotfiles+"/")
	if !inProjects && !inDotfiles {
		return ""
	}

	repoRoot := gitToplevel(cwd)
	if repoRoot == "" {
		return ""
	}

	hotFile := filepath.Join(repoRoot, "docs", "knowledge", "hot.md")
	hot, err := os.ReadFile(hotFile)
	if err != nil || len(strings.TrimSpace(string(hot))) == 0 {
		// Memory not attached. Do not stay silent: the memory loop once died
		// invisibly for weeks because a hook chose not to bother anyone.
		return hookJSON(memoryWarning(workRoot, repoRoot) + memoryCheckpoint)
	}

	var context string
	if reason := staleReason(string(hot)); reason != "" {
		// A stale index is worse than none: the agent trusts it as current.
		// This is not theory — a gotcha marked obsolete on 2026-07-17 was
		// still served as a Critical Gotcha weeks later, and three inbox
		// entries cited it as authority.
		context = "# WikiPedik memory is stale\n\n" +
			"docs/knowledge/hot.md " + reason + ", so it is NOT loaded as current context.\n" +
			"Do not rely on remembered claims until it is rebuilt: `hq wiki hot-refresh <scope>`.\n" +
			"Curated pages under docs/knowledge/ can still be read directly — verify their citations first." +
			memoryCheckpoint
	} else {
		context = "# Recent wiki context (auto-loaded from docs/knowledge/hot.md)\n\n" +
			strings.TrimSpace(string(hot)) + memoryCheckpoint
	}

	if candidates := countInboxCandidates(repoRoot); candidates >= inboxNudgeThreshold {
		context += fmt.Sprintf("\n\nInbox status: %d candidate lessons are pending in docs/knowledge/_inbox.md. Mention to the user that `wiki sync` is overdue.", candidates)
	}
	return hookJSON(context)
}

// inboxNudgeThreshold is the single source for "the inbox needs attention".
// The number used to differ between the hook, the curator skill and the
// cheatsheet, so nobody could say what "overdue" meant.
const inboxNudgeThreshold = 5

// hotMaxAge is how long a rebuilt index stays trustworthy. Memory that has
// not been touched for two months describes a codebase that no longer exists.
const hotMaxAge = 60 * 24 * time.Hour

var hotStampRe = regexp.MustCompile(`<!-- last refreshed: ([^(]+?)\s*(?:\(|-->)`)

// staleReason explains why an index must not be injected, or "" when it is
// fresh enough to trust.
func staleReason(hot string) string {
	match := hotStampRe.FindStringSubmatch(hot)
	if match == nil {
		return "carries no refresh stamp"
	}
	stamp := strings.TrimSpace(match[1])
	if stamp == "never" {
		return "was never rebuilt (it is still the empty template)"
	}
	when, err := time.Parse("2006-01-02 15:04", stamp)
	if err != nil {
		return ""
	}
	if age := time.Since(when); age > hotMaxAge {
		return fmt.Sprintf("was last rebuilt %d days ago (%s)", int(age.Hours()/24), stamp)
	}
	return ""
}

// zoneOf reports which vault zone a directory belongs to.
func zoneOf(vaultRoot, cwd string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(cwd, vaultRoot), "/")
	if rel == "" {
		return ""
	}
	head := strings.SplitN(rel, "/", 2)[0]
	for _, zone := range VaultZones {
		if zone.Dir == head {
			return zone.Name
		}
	}
	return ""
}

func hookJSON(context string) string {
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "SessionStart",
			"additionalContext": context,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

// PromptHookJSON wraps context for the UserPromptSubmit event.
func PromptHookJSON(context string) string {
	payload := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "UserPromptSubmit",
			"additionalContext": context,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func gitToplevel(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func memoryWarning(workRoot, repoRoot string) string {
	rel := strings.TrimPrefix(repoRoot, workRoot+"/")
	rel = strings.TrimPrefix(rel, ".worktrees/")
	parts := strings.Split(rel, "/")
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	scope := strings.TrimSpace(strings.Join(parts[:3], " "))

	return "# WikiPedik memory warning\n\n" +
		"This repo has no WikiPedik project memory attached: docs/knowledge/hot.md is missing or empty.\n" +
		"Lessons will not be captured and no prior context is available.\n" +
		fmt.Sprintf("Early in this session, tell the user that project memory is disconnected and suggest running: `wiki bootstrap %s`.", scope)
}

var candidateLineRe = regexp.MustCompile(`(?m)^Status: candidate$`)

func countInboxCandidates(repoRoot string) int {
	data, err := os.ReadFile(filepath.Join(repoRoot, "docs", "knowledge", "_inbox.md"))
	if err != nil {
		return 0
	}
	return len(candidateLineRe.FindAllString(string(data), -1))
}

// vaultCheckpoint is the discipline reminder for sessions inside the vault
// itself — not project memory, session rules for Obsidian work.
func vaultCheckpoint(vaultRoot, cwd string) string {
	scope := "root"
	switch {
	case strings.HasPrefix(cwd, filepath.Join(vaultRoot, "dev")):
		scope = "dev"
	case strings.HasPrefix(cwd, filepath.Join(vaultRoot, "research")):
		scope = "research"
	case strings.HasPrefix(cwd, filepath.Join(vaultRoot, "Personal Brand")):
		scope = "Personal Brand"
	}

	return fmt.Sprintf(`# WikiPedik session checkpoint

Current WikiPedik scope: %s

- Conversation and vault-writing default: Russian.
- Code identifiers, commands, URLs, and stable product names stay as written.
- Personal Brand content is Russian by default. Translate or write English only on explicit user request.
- English Personal Brand output should preserve the Russian source and usually be a separate copy with filename suffix `+"` - EN.md`"+`.
- Закончил логическое изменение — сразу коммить сам: `+"`hq wikipedik sync \"<что сделал по смыслу>\"`"+` (коммитит вольт и пушит). Не жди конца сессии, не пиши «не забудь закоммитить» — коммит твой.
- Current Git boundary: `+"`%s`"+` is the vault Git repository. Project memory lives in `+"`%s/dev`"+`, research in `+"`%s/research`"+`, and Personal Brand in `+"`%s/Personal Brand`"+`.
- Сырой `+"`git commit`"+`/`+"`git push`"+` не используй: коммит вольта идёт через `+"`hq wikipedik`"+` — там секрет-скан диффа и pull --rebase перед пушем.
- Commit messages: English Conventional Commit type/scope, Russian description/body, for example `+"`docs(brand): обновить трекер личного бренда`"+`.
`, scope, vaultRoot, vaultRoot, vaultRoot, vaultRoot)
}
