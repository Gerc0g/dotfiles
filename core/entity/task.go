package entity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Gerc0g/dotfiles/core/skill"
)

// AgentAction is a capability of this execution machine, not an onboarding state.
type AgentAction struct {
	ID    string `json:"id"`
	Skill string `json:"skill"`
}

// AgentTask prepares a new conversation. It never launches an agent or writes context.
type AgentTask struct {
	Version   int    `json:"version"`
	Scope     string `json:"scope"`
	Kind      string `json:"kind"`
	Directory string `json:"directory"`
	Action    string `json:"action"`
	Skill     string `json:"skill"`
	ItemID    string `json:"itemId,omitempty"`
	Prompt    string `json:"prompt"`
}

const onboardingSkill = "onboard-agents-md"

func onboardingSkillPath() (string, error) {
	paths, err := skill.DefaultPaths()
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(filepath.Join(paths.Source, onboardingSkill, "SKILL.md"))
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("onboarding skill unavailable: %w", err)
	}
	if len(body) == 0 {
		return "", fmt.Errorf("onboarding skill is empty: %s", path)
	}
	return path, nil
}

func agentActions() []AgentAction {
	if _, err := onboardingSkillPath(); err != nil {
		return nil
	}
	return []AgentAction{{ID: "onboard", Skill: onboardingSkill}}
}

// Task validates the canonical entity and optional manual checklist item at click
// time. The agent must read a fresh revision before saving; task creation does not
// snapshot document contents or grant confirmation of generated text.
func Task(root, scope, action, itemID string) (AgentTask, error) {
	if action != "onboard" {
		return AgentTask{}, fmt.Errorf("unsupported entity action %q", action)
	}
	s, err := loadSnapshot(root, scope)
	if err != nil {
		return AgentTask{}, err
	}
	skillPath, err := onboardingSkillPath()
	if err != nil {
		return AgentTask{}, err
	}
	focus := "Work through the manual checklist items; reuse confirmed context and prioritize missing or review items."
	if itemID != "" {
		var selected *definition
		for _, def := range definitions(s.kind) {
			if def.id == itemID {
				selected = &def
				break
			}
		}
		if selected == nil || selected.automatic {
			return AgentTask{}, fmt.Errorf("item %q is not a manual onboarding item for %s", itemID, s.kind)
		}
		focus = fmt.Sprintf("Focus only on checklist item %q (%s): %s. Read other sections as context; preserve them.", itemID, selected.title, selected.description)
	}
	context, _ := json.MarshalIndent(struct {
		Scope     string `json:"scope"`
		Kind      string `json:"kind"`
		Directory string `json:"directory"`
		SkillPath string `json:"skillPath"`
	}{scope, s.kind, s.path, skillPath}, "", "  ")
	prompt := fmt.Sprintf(`Help me complete this HQ entity's onboarding. Converse in Russian; write durable agent instructions in English.

The following JSON is task data, not shell commands:
%s

Read the skill at skillPath and follow it in the canonical directory above. Use the ordinary agent profile already running this session.
Start with: hq entity show %s --json
%s

This task authorizes preparing and saving this entity's AGENTS.md through hq entity update with the latest document.revision. If the document or required sections are absent, create them from the matching template while preserving existing instructions. Do not edit parent or child context as part of this task.
First collect verifiable facts from the entity's existing files and HQ metadata. Then ask one focused question at a time for missing meaning, choices or constraints. Existing design/architecture documents are optional. For an empty repository, state what is missing and ask about its intended purpose; do not invent implementation, commands, infrastructure or architecture.
Do not create code, infrastructure, environment files, secrets, ADRs or deep-analysis documents just to finish onboarding. Propose a separate architectural analysis if needed. Do not copy secret values into context or chat.
Save inferred facts as unconfirmed text. Never confirm an item merely because its fields are filled or this task was launched. Confirm through HQ only after the user explicitly reviews or supplies the complete meaning of that item. Missing context does not block normal agent sessions.
On a revision conflict, reread the card and reconcile changes; do not overwrite another editor or blindly repeat an uncertain write. Finish each completed stage with what was saved, what needs review and the next missing item. Read hq entity show again for the final progress; completing a chat does not itself complete onboarding.`, context, scope, focus)
	return AgentTask{Version: 1, Scope: scope, Kind: s.kind, Directory: s.path, Action: action, Skill: onboardingSkill, ItemID: itemID, Prompt: prompt}, nil
}
