package agentconfig

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Inspect asks the installed Codex app server to read a freshly materialized
// preset. It never starts a thread, sends a prompt, executes tools, or mutates
// the active server login. Only selected non-secret fields leave this adapter.
func Inspect(preset string) (any, error) {
	return inspect(preset, nil)
}

func inspect(preset string, draft *Settings) (any, error) {
	temp, err := os.MkdirTemp("", "hq-codex-check-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	home := filepath.Join(temp, "home")
	var effective Effective
	if draft == nil {
		effective, err = Materialize(preset, home)
	} else {
		if err = validate(*draft); err != nil {
			return nil, err
		}
		p, ok := draft.Presets[preset]
		if !ok {
			return nil, errors.New("HQ_AGENT_PRESET_INVALID")
		}
		fingerprint := *draft
		fingerprint.Revision = ""
		raw, e := json.Marshal(fingerprint)
		if e != nil {
			return nil, e
		}
		effective, err = materialize(Effective{Preset: p, Revision: revision(raw), Source: "hq-draft"}, home)
	}
	if err != nil {
		return nil, err
	}
	// Inspection runs on the owner host, so preset MCP processes and HTTP
	// endpoints must stay disabled. Their real connectivity is checked only
	// inside a task governed by the runner's network and filesystem boundary.
	diagnostic := effective.Preset
	diagnostic.MCP = append([]MCP(nil), effective.MCP...)
	for i := range diagnostic.MCP {
		diagnostic.MCP[i].Enabled = false
	}
	rawConfig, err := render(diagnostic)
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(home, "config.toml"), rawConfig, 0600); err != nil {
		return nil, err
	}
	userHome, _ := os.UserHomeDir()
	authHome := os.Getenv("HQ_CODEX_AUTH_HOME")
	if authHome == "" {
		authHome = filepath.Join(userHome, ".codex")
	}
	authFile := os.Getenv("HQ_CODEX_AUTH_FILE")
	if authFile == "" {
		authFile = filepath.Join(authHome, "auth.json")
	}
	if raw, e := os.ReadFile(authFile); e == nil {
		if e = os.WriteFile(filepath.Join(home, "auth.json"), raw, 0600); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, errors.New("HQ_CODEX_AUTH_UNAVAILABLE")
	}
	binary := os.Getenv("HQ_CODEX_BINARY")
	if binary == "" {
		binary = filepath.Join(userHome, ".happier/tools/providers/codex/current/bin/codex")
	}
	if !filepath.IsAbs(binary) {
		return nil, errors.New("HQ_CODEX_EXECUTABLE_INVALID")
	}
	// This is a bounded diagnostic, not a session lease. The machine transport's
	// normal request deadline remains the outer owner; no model turn runs here.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "app-server", "--listen", "stdio://")
	cmd.Dir = temp
	cmd.Env = []string{"HOME=" + temp, "CODEX_HOME=" + home, "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8"}
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		return nil, errors.New("HQ_CODEX_UNAVAILABLE")
	}
	defer func() { input.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	encoder := json.NewEncoder(input)
	request := func(id int, method string, params any) (map[string]any, error) {
		if err := encoder.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for scanner.Scan() {
			var message struct {
				ID     int             `json:"id"`
				Result map[string]any  `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if json.Unmarshal(scanner.Bytes(), &message) != nil || message.ID != id {
				continue
			}
			if len(message.Error) > 0 && string(message.Error) != "null" {
				return nil, errors.New("HQ_CODEX_CONFIG_REJECTED")
			}
			return message.Result, nil
		}
		return nil, errors.New("HQ_CODEX_CHECK_FAILED")
	}
	_, err = request(1, "initialize", map[string]any{"clientInfo": map[string]any{"name": "hq", "version": "1"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err != nil {
		return nil, err
	}
	if err = encoder.Encode(map[string]any{"method": "initialized"}); err != nil {
		return nil, err
	}
	config, err := request(2, "config/read", map[string]any{"includeLayers": false})
	if err != nil {
		return nil, err
	}
	configValues, ok := config["config"].(map[string]any)
	if !ok {
		return nil, errors.New("HQ_CODEX_CHECK_FAILED")
	}
	account, err := request(3, "account/read", map[string]any{"refreshToken": false})
	if err != nil {
		return nil, err
	}
	models, modelErr := request(4, "model/list", map[string]any{"includeHidden": true, "limit": 100})
	modelList := []any{}
	var modelAvailable, effortAvailable any
	catalogAvailable := false
	catalogComplete := false
	if modelErr == nil {
		if values, ok := models["data"].([]any); ok {
			catalogAvailable = true
			cursor, _ := models["nextCursor"].(string)
			catalogComplete = cursor == ""
			if effective.Model != "" && catalogComplete {
				modelAvailable = false
			}
			for _, raw := range values {
				if m, ok := raw.(map[string]any); ok {
					id, idOK := m["id"].(string)
					if !idOK {
						continue
					}
					if id == effective.Model {
						modelAvailable = true
						effortAvailable = false
					}
					efforts := []any{}
					if values, ok := m["supportedReasoningEfforts"].([]any); ok {
						for _, value := range values {
							if v, ok := value.(map[string]any); ok {
								if effort, ok := v["reasoningEffort"].(string); ok {
									efforts = append(efforts, map[string]any{"reasoningEffort": effort})
									if id == effective.Model && effort == effective.Effort {
										effortAvailable = true
									}
								}
							}
						}
					}
					name, _ := m["displayName"].(string)
					modelList = append(modelList, map[string]any{"id": id, "displayName": name, "supportedReasoningEfforts": efforts})
				}
			}
		}
	}
	effectiveConfig := map[string]any{}
	for _, k := range []string{"model", "model_reasoning_effort", "service_tier", "approvals_reviewer", "web_search", "sandbox_mode", "approval_policy"} {
		if value, ok := configValues[k].(string); ok {
			effectiveConfig[k] = value
		} else {
			effectiveConfig[k] = nil
		}
	}
	return map[string]any{"preset": preset, "source": effective.Source, "revision": effective.Revision, "configAccepted": true, "authenticated": account["account"] != nil, "modelCatalogAvailable": catalogAvailable, "modelCatalogComplete": catalogComplete, "selectedModelAvailable": modelAvailable, "selectedEffortAvailable": effortAvailable, "models": modelList, "effective": effectiveConfig, "mcpExecutionChecked": false, "appliesTo": "new-sessions"}, nil
}
