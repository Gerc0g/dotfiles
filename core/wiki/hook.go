package wiki

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

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
		return hookJSON(memoryWarning(workRoot, repoRoot))
	}

	context := "# Recent wiki context (auto-loaded from docs/knowledge/hot.md)\n\n" +
		strings.TrimSpace(string(hot)) + memoryCheckpoint

	if candidates := countInboxCandidates(repoRoot); candidates >= 5 {
		context += fmt.Sprintf("\n\nInbox status: %d candidate lessons are pending in docs/knowledge/_inbox.md. Mention to the user that `wiki sync` is overdue.", candidates)
	}
	return hookJSON(context)
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
- Изменил вольт — заверши это коммитом: `+"`hq wikipedik sync \"<кратко по-русски>\"`"+` (коммитит весь вольт и пушит). Не накапливай изменения и не жди просьбы.
- Current Git boundary: `+"`%s`"+` is the vault Git repository. Project memory lives in `+"`%s/dev`"+`, research in `+"`%s/research`"+`, and Personal Brand in `+"`%s/Personal Brand`"+`.
- Сырой `+"`git commit`"+`/`+"`git push`"+` не используй: коммит вольта идёт только через `+"`hq wikipedik sync`"+` — там секрет-скан диффа и pull --rebase перед пушем.
- Commit messages: English Conventional Commit type/scope, Russian description/body, for example `+"`docs(brand): обновить трекер личного бренда`"+`.
`, scope, vaultRoot, vaultRoot, vaultRoot, vaultRoot)
}
