// Package agentconfig owns the desired Codex server presets. Runtime files are
// derived snapshots; changing them never grants more host access to a runner.
package agentconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/sys/unix"
)

type Hooks struct {
	SessionContext bool `json:"sessionContext"`
	InboxCapture   bool `json:"inboxCapture"`
}

type MCP struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Enabled bool     `json:"enabled"`
	URL     string   `json:"url,omitempty"`
}

type Preset struct {
	Model             string   `json:"model"`
	Effort            string   `json:"effort"`
	ServiceTier       string   `json:"serviceTier,omitempty"`
	ApprovalsReviewer string   `json:"approvalsReviewer,omitempty"`
	JSRepl            *bool    `json:"jsRepl,omitempty"`
	WebSearch         string   `json:"webSearch"`
	Network           bool     `json:"network"`
	Instructions      string   `json:"instructions"`
	Skills            []string `json:"skills"`
	Hooks             Hooks    `json:"hooks"`
	MCP               []MCP    `json:"mcp"`
}

type ImportItem struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}
type ImportPreview struct {
	Preset   Preset       `json:"preset"`
	Imported []ImportItem `json:"imported"`
	Replaced []ImportItem `json:"replaced"`
	Skipped  []ImportItem `json:"skipped"`
}
type Settings struct {
	Version      int               `json:"version"`
	Revision     string            `json:"revision"`
	Presets      map[string]Preset `json:"presets"`
	ImportReport *ImportPreview    `json:"importReport,omitempty"`
}
type Effective struct {
	Preset
	Revision       string `json:"revision"`
	Source         string `json:"source"`
	PermissionMode string `json:"permissionMode,omitempty"`
}
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Research    bool   `json:"research"`
}

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
var researchSkills = map[string]bool{"source-ingest": true, "autoresearch": true, "wiki-lint": true, "skill-creator": true}

func DataRoot() string {
	if root := os.Getenv("HQ_DATA_ROOT"); root != "" {
		return root
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "hq")
}
func skillsRoot() string {
	if root := os.Getenv("HQ_SKILLS_ROOT"); root != "" {
		return root
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "dotfiles", "skills")
}
func settingsFile() string { return filepath.Join(DataRoot(), "settings", "agent.json") }
func defaults() Settings {
	work := Preset{Effort: "high", WebSearch: "live", Network: true, Skills: []string{}, MCP: []MCP{}, Hooks: Hooks{true, true}}
	research := work
	research.Instructions = "Research only within the mounted Wikipedia research area. Keep source citations and uncertainty explicit. Company data and credentials are not available."
	return Settings{Version: 1, Presets: map[string]Preset{"work": work, "research": research, "ordinary": ordinaryDefault(work)}}
}
func ordinaryDefault(work Preset) Preset {
	return Preset{
		Model: work.Model, Effort: work.Effort, WebSearch: "live", Network: true,
		Instructions: "Help with general questions and tasks in the isolated ordinary workspace. Company and research memory are not available. Keep any files within this workspace.",
		Skills:       []string{}, MCP: []MCP{},
	}
}

func revision(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func Read() (Settings, error) {
	raw, err := os.ReadFile(settingsFile())
	if os.IsNotExist(err) {
		s := defaults()
		data, _ := json.Marshal(s)
		s.Revision = revision(data)
		return s, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err = json.Unmarshal(raw, &s); err != nil {
		return s, errors.New("HQ_AGENT_CONFIG_INVALID")
	}
	if s.Version != 1 {
		return s, errors.New("HQ_AGENT_CONFIG_VERSION")
	}
	// Extend legacy settings in memory only. The revision still fingerprints
	// persisted bytes, so a pre-upgrade client revision remains valid.
	if _, ok := s.Presets["ordinary"]; !ok && len(s.Presets) == 2 {
		if work, ok := s.Presets["work"]; ok {
			if _, ok := s.Presets["research"]; ok {
				s.Presets["ordinary"] = ordinaryDefault(work)
			}
		}
	}
	s.Revision = revision(raw)
	return s, nil
}
func validate(s Settings) error {
	if s.Version != 1 || (len(s.Presets) != 2 && len(s.Presets) != 3) {
		return errors.New("HQ_AGENT_CONFIG_VERSION")
	}
	names := []string{"work", "research"}
	if len(s.Presets) == 3 {
		names = append(names, "ordinary")
	}
	for _, name := range names {
		p, ok := s.Presets[name]
		if !ok {
			return errors.New("HQ_AGENT_PRESET_INVALID")
		}
		if p.Model != "" && !namePattern.MatchString(p.Model) {
			return errors.New("HQ_AGENT_MODEL_INVALID")
		}
		// Models advertise additional reasoning levels over time; accept identifiers,
		// and let the installed Codex model API validate model-specific combinations.
		if !namePattern.MatchString(p.Effort) {
			return errors.New("HQ_AGENT_EFFORT_INVALID")
		}
		if p.ServiceTier != "" && !namePattern.MatchString(p.ServiceTier) {
			return errors.New("HQ_AGENT_SERVICE_TIER_INVALID")
		}
		if p.ApprovalsReviewer != "" && p.ApprovalsReviewer != "user" && p.ApprovalsReviewer != "auto_review" {
			return errors.New("HQ_AGENT_REVIEWER_INVALID")
		}
		if p.WebSearch != "live" && p.WebSearch != "cached" && p.WebSearch != "indexed" && p.WebSearch != "disabled" {
			return errors.New("HQ_AGENT_SEARCH_INVALID")
		}
		seen := map[string]bool{}
		for _, skill := range p.Skills {
			if seen[skill] {
				return errors.New("HQ_AGENT_SKILL_DUPLICATE")
			}
			seen[skill] = true
			if !namePattern.MatchString(skill) {
				return errors.New("HQ_AGENT_SKILL_INVALID")
			}
			if name == "research" && !researchSkills[skill] {
				return errors.New("HQ_AGENT_SKILL_SCOPE")
			}
			if _, err := readSkill(skill); err != nil {
				return err
			}
		}
		seen = map[string]bool{}
		for _, m := range p.MCP {
			if !namePattern.MatchString(m.Name) || seen[m.Name] {
				return errors.New("HQ_AGENT_MCP_NAME_INVALID")
			}
			seen[m.Name] = true
			// Commands execute only inside the task image. Host paths and secret env are
			// deliberately not a part of the editable MCP contract.
			if m.URL != "" {
				if m.Command != "" || len(m.Args) > 0 || !validMCPURL(m.URL) {
					return errors.New("HQ_AGENT_MCP_URL_INVALID")
				}
			} else if !namePattern.MatchString(m.Command) {
				return errors.New("HQ_AGENT_MCP_COMMAND_INVALID")
			}
			for _, arg := range m.Args {
				if strings.ContainsRune(arg, 0) {
					return errors.New("HQ_AGENT_MCP_ARGUMENT_INVALID")
				}
			}
		}
	}
	return nil
}

// HTTPS MCP traffic still traverses the task proxy, which resolves DNS and
// blocks private addresses and company environments. No auth/header fields
// exist in this preset contract, and local transports run only in the task.
func validMCPURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	return (u.Port() == "" || u.Port() == "443") && strings.Contains(host, ".") && net.ParseIP(host) == nil && !strings.HasSuffix(host, ".localhost") && !strings.HasSuffix(host, ".local") && !strings.HasSuffix(host, ".internal")
}
func Save(next Settings, expected string) (Settings, error) {
	if err := validate(next); err != nil {
		return Settings{}, err
	}
	dir := filepath.Dir(settingsFile())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return Settings{}, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "agent.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return Settings{}, err
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return Settings{}, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	old, err := Read()
	if err != nil {
		return old, err
	}
	if expected == "" || expected != old.Revision {
		return old, errors.New("HQ_AGENT_REVISION_CONFLICT")
	}
	// Older clients send only Work/Research. Preserve current Ordinary settings
	// under the revision lock without mutating the caller's draft map.
	if _, ok := next.Presets["ordinary"]; !ok {
		presets := make(map[string]Preset, 3)
		for name, preset := range next.Presets {
			presets[name] = preset
		}
		presets["ordinary"] = old.Presets["ordinary"]
		next.Presets = presets
		if err := validate(next); err != nil {
			return Settings{}, err
		}
	}
	next.Revision = ""
	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return Settings{}, err
	}
	if err = atomicWrite(settingsFile(), raw); err != nil {
		return Settings{}, err
	}
	return Read()
}
func atomicWrite(name string, raw []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), ".hq-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), name)
}
func EffectiveFor(preset string) (Effective, error) {
	s, err := Read()
	if err != nil {
		return Effective{}, err
	}
	p, ok := s.Presets[preset]
	if !ok {
		return Effective{}, errors.New("HQ_AGENT_PRESET_INVALID")
	}
	if err = validate(s); err != nil {
		return Effective{}, err
	}
	return Effective{Preset: p, Revision: s.Revision, Source: "hq-preset"}, nil
}
func readSkill(name string) ([]byte, error) {
	root, err := os.OpenRoot(skillsRoot())
	if err != nil {
		return nil, err
	}
	defer root.Close()
	raw, err := root.ReadFile(filepath.Join(name, "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("HQ_AGENT_SKILL_UNAVAILABLE: %s", name)
	}
	return raw, nil
}
func Skills() ([]Skill, error) {
	entries, err := os.ReadDir(skillsRoot())
	if os.IsNotExist(err) {
		return []Skill{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := []Skill{}
	for _, entry := range entries {
		if !namePattern.MatchString(entry.Name()) {
			continue
		}
		raw, err := readSkill(entry.Name())
		if err != nil {
			continue
		}
		description := ""
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "description:") {
				description = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), "\"'")
				break
			}
		}
		result = append(result, Skill{Name: entry.Name(), Description: description, Research: researchSkills[entry.Name()]})
	}
	return result, nil
}
func PreviewImport(preset, source string) (ImportPreview, error) {
	effective, err := EffectiveFor(preset)
	if err != nil {
		return ImportPreview{}, err
	}
	result := ImportPreview{Preset: effective.Preset, Imported: []ImportItem{}, Replaced: []ImportItem{}, Skipped: []ImportItem{}}
	var data map[string]any
	if err = toml.Unmarshal([]byte(source), &data); err != nil {
		return result, errors.New("HQ_AGENT_IMPORT_TOML_INVALID")
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, isString := data[key].(string)
		switch key {
		case "model", "model_reasoning_effort", "web_search", "service_tier", "approvals_reviewer", "developer_instructions":
			if !isString {
				result.Skipped = append(result.Skipped, ImportItem{key, "unsupported value type"})
				continue
			}
			if key == "model" {
				result.Preset.Model = value
			}
			if key == "model_reasoning_effort" {
				result.Preset.Effort = value
			}
			if key == "web_search" {
				result.Preset.WebSearch = value
			}
			if key == "service_tier" {
				result.Preset.ServiceTier = value
			}
			if key == "approvals_reviewer" {
				result.Preset.ApprovalsReviewer = value
			}
			if key == "developer_instructions" {
				result.Preset.Instructions = value
			}
			result.Imported = append(result.Imported, ImportItem{key, "portable Codex preference"})
		case "features":
			features, ok := data[key].(map[string]any)
			if !ok {
				result.Skipped = append(result.Skipped, ImportItem{key, "unsupported value type"})
				continue
			}
			for _, name := range sortedKeys(features) {
				if enabled, ok := features[name].(bool); name == "js_repl" && ok {
					result.Preset.JSRepl = &enabled
					result.Imported = append(result.Imported, ImportItem{"features.js_repl", "portable Codex preference"})
				} else {
					result.Skipped = append(result.Skipped, ImportItem{"features." + name, "feature not in the server allowlist"})
				}
			}
		case "mcp_servers":
			servers, ok := data[key].(map[string]any)
			if !ok {
				result.Skipped = append(result.Skipped, ImportItem{key, "unsupported value type"})
				continue
			}
			for _, name := range sortedKeys(servers) {
				server, ok := servers[name].(map[string]any)
				endpoint, _ := server["url"].(string)
				enabled := true
				if v, present := server["enabled"]; present {
					enabled, ok = v.(bool)
				}
				for k := range server {
					if k != "url" && k != "enabled" {
						ok = false
					}
				}
				if !ok || !namePattern.MatchString(name) || !validMCPURL(endpoint) {
					result.Skipped = append(result.Skipped, ImportItem{"mcp_servers." + name, "only public HTTPS endpoints without credentials, headers or local commands are portable"})
					continue
				}
				m := MCP{Name: name, URL: endpoint, Args: []string{}, Enabled: enabled}
				found := false
				for i := range result.Preset.MCP {
					if result.Preset.MCP[i].Name == name {
						result.Preset.MCP[i] = m
						found = true
					}
				}
				if !found {
					result.Preset.MCP = append(result.Preset.MCP, m)
				}
				result.Imported = append(result.Imported, ImportItem{"mcp_servers." + name, "public HTTPS MCP; access remains restricted by the task network policy"})
			}
		case "sandbox_mode", "approval_policy", "sandbox_workspace_write", "hooks":
			result.Replaced = append(result.Replaced, ImportItem{key, "HQ task isolation and managed hooks"})
		default:
			result.Skipped = append(result.Skipped, ImportItem{key, "machine-specific or not in the server allowlist; configure explicitly"})
		}
	}
	s, _ := Read()
	s.Presets[preset] = result.Preset
	if err = validate(s); err != nil {
		return result, err
	}
	return result, nil
}
func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func render(p Preset) ([]byte, error) {
	c := map[string]any{"model_reasoning_effort": p.Effort, "web_search": p.WebSearch, "approval_policy": "on-request", "sandbox_mode": "workspace-write", "sandbox_workspace_write": map[string]any{"network_access": p.Network}}
	if p.Model != "" {
		c["model"] = p.Model
	}
	if p.ServiceTier != "" {
		c["service_tier"] = p.ServiceTier
	}
	if p.ApprovalsReviewer != "" {
		c["approvals_reviewer"] = p.ApprovalsReviewer
	}
	if p.JSRepl != nil {
		c["features"] = map[string]any{"js_repl": *p.JSRepl}
	}
	if p.Instructions != "" {
		c["developer_instructions"] = p.Instructions
	}
	servers := map[string]any{}
	for _, m := range p.MCP {
		if m.URL != "" {
			servers[m.Name] = map[string]any{"url": m.URL, "enabled": m.Enabled}
		} else {
			servers[m.Name] = map[string]any{"command": m.Command, "args": m.Args, "enabled": m.Enabled}
		}
	}
	if len(servers) > 0 {
		c["mcp_servers"] = servers
	}
	return toml.Marshal(c)
}

// SessionEffectiveFor validates an overlay without mutating persisted presets.
func SessionEffectiveFor(preset string, options ...SessionOptions) (Effective, error) {
	effective, err := EffectiveFor(preset)
	if err != nil {
		return effective, err
	}
	if len(options) > 1 {
		return effective, errors.New("HQ_SESSION_OPTIONS_INVALID")
	}
	if len(options) == 1 {
		return options[0].apply(preset, effective)
	}
	return effective, nil
}

// Materialize must receive a new runtime directory, never an existing login or
// session home. No auth, history, host plugins, or executable local hooks cross it.
func Materialize(preset, destination string, options ...SessionOptions) (Effective, error) {
	effective, err := SessionEffectiveFor(preset, options...)
	if err != nil {
		return effective, err
	}
	return materialize(effective, destination)
}

func materialize(effective Effective, destination string) (Effective, error) {
	if err := os.Mkdir(destination, 0700); err != nil {
		return effective, fmt.Errorf("HQ_AGENT_NEW_HOME_REQUIRED: %w", err)
	}
	raw, err := renderEffective(effective)
	if err != nil {
		return effective, err
	}
	if err = os.WriteFile(filepath.Join(destination, "config.toml"), raw, 0600); err != nil {
		return effective, err
	}
	if err = os.WriteFile(filepath.Join(destination, "AGENTS.md"), []byte(effective.Instructions+"\n"), 0600); err != nil {
		return effective, err
	}
	root, err := os.OpenRoot(skillsRoot())
	if err != nil && len(effective.Skills) > 0 {
		return effective, err
	}
	if root != nil {
		defer root.Close()
	}
	for _, skill := range effective.Skills {
		err = fs.WalkDir(root.FS(), skill, func(name string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.Type()&os.ModeSymlink != 0 {
				return errors.New("HQ_AGENT_SKILL_SYMLINK")
			}
			target := filepath.Join(destination, "skills", name)
			if d.IsDir() {
				return os.MkdirAll(target, 0700)
			}
			if !d.Type().IsRegular() {
				return errors.New("HQ_AGENT_SKILL_FILE_TYPE")
			}
			data, e := root.ReadFile(name)
			if e != nil {
				return e
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			mode := os.FileMode(0600)
			if info.Mode()&0111 != 0 {
				mode = 0700
			}
			return os.WriteFile(target, data, mode)
		})
		if err != nil {
			return effective, err
		}
	}
	return effective, nil
}

func Dispatch(operation string, args json.RawMessage) (any, error) {
	switch operation {
	case "agent.inspect":
		var input struct {
			Preset   string    `json:"preset"`
			Settings *Settings `json:"settings,omitempty"`
		}
		if err := decode(args, &input); err != nil {
			return nil, err
		}
		return inspect(input.Preset, input.Settings)
	case "agent.get":
		s, err := Read()
		if err != nil {
			return nil, err
		}
		skills, err := Skills()
		return map[string]any{"settings": s, "skills": skills, "provider": "codex", "applyTo": "new-sessions"}, err
	case "agent.save":
		var input struct {
			Settings Settings `json:"settings"`
			Revision string   `json:"revision"`
		}
		if err := decode(args, &input); err != nil {
			return nil, err
		}
		return Save(input.Settings, input.Revision)
	case "agent.previewImport":
		var input struct {
			Preset string `json:"preset"`
			TOML   string `json:"toml"`
		}
		if err := decode(args, &input); err != nil {
			return nil, err
		}
		return PreviewImport(input.Preset, input.TOML)
	case "agent.preview":
		var input struct {
			Settings Settings `json:"settings"`
			Preset   string   `json:"preset"`
		}
		if err := decode(args, &input); err != nil {
			return nil, err
		}
		if err := validate(input.Settings); err != nil {
			return nil, err
		}
		p, ok := input.Settings.Presets[input.Preset]
		if !ok {
			return nil, errors.New("HQ_AGENT_PRESET_INVALID")
		}
		raw, err := render(p)
		return map[string]any{"config": string(raw), "source": "hq-preset", "applyTo": "new-sessions"}, err
	default:
		return nil, errors.New("HQ_AGENT_OPERATION_UNKNOWN")
	}
}
func decode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("HQ_AGENT_REQUEST_INVALID")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("HQ_AGENT_REQUEST_INVALID")
	}
	return nil
}
