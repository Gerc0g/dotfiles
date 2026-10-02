package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// This is a policy on the existing Codex 0.157.1 v2 JSONL protocol, not a
// replacement transport. Unrelated requests and approval replies pass intact.
type rpcPolicy struct {
	Model       string `json:"model"`
	Effort      string `json:"effort"`
	ServiceTier string `json:"serviceTier"`
	WebSearch   string `json:"webSearch"`
}

func (p rpcPolicy) apply(line []byte) ([]byte, error) {
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return line, nil
	}
	var method string
	if err := json.Unmarshal(msg["method"], &method); err != nil {
		return line, nil
	}
	thread := method == "thread/start" || method == "thread/resume" || method == "thread/fork"
	if !thread && method != "turn/start" {
		return line, nil
	}
	if p.Model == "" {
		return nil, fmt.Errorf("HQ default model has not been resolved")
	}
	var params map[string]json.RawMessage
	if len(msg["params"]) > 0 && !bytes.Equal(msg["params"], []byte("null")) {
		if err := json.Unmarshal(msg["params"], &params); err != nil {
			return nil, fmt.Errorf("invalid Codex thread/turn parameters")
		}
	}
	if params == nil {
		params = map[string]json.RawMessage{}
	}
	put := func(target map[string]json.RawMessage, key string, value any) { target[key], _ = json.Marshal(value) }
	if p.Model != "" {
		put(params, "model", p.Model)
	} else {
		delete(params, "model")
	}
	tier := p.ServiceTier
	if tier == "" {
		tier = "default"
	}
	put(params, "serviceTier", tier)
	if thread {
		put(params, "modelProvider", "openai")
		var config map[string]json.RawMessage
		if raw := params["config"]; len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
			if err := json.Unmarshal(raw, &config); err != nil {
				return nil, fmt.Errorf("invalid Codex thread configuration")
			}
		}
		if config == nil {
			config = map[string]json.RawMessage{}
		}
		if p.Model != "" {
			put(config, "model", p.Model)
		} else {
			delete(config, "model")
		}
		put(config, "model_provider", "openai")
		if p.Effort != "" {
			put(config, "model_reasoning_effort", p.Effort)
		} else {
			delete(config, "model_reasoning_effort")
		}
		put(config, "service_tier", tier)
		put(config, "web_search", p.WebSearch)
		put(params, "config", config)
	} else {
		if p.Effort != "" {
			put(params, "effort", p.Effort)
		} else {
			delete(params, "effort")
		}
		put(params, "serviceTierForTurn", tier)
		// TurnStart has no web-search override; it retains the managed thread
		// configuration. Config writes cannot alter the read-only preset file.
	}
	if raw := params["collaborationMode"]; len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
		var mode map[string]json.RawMessage
		if err := json.Unmarshal(raw, &mode); err != nil {
			return nil, fmt.Errorf("invalid collaboration mode")
		}
		var settings map[string]json.RawMessage
		if raw := mode["settings"]; len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
			if err := json.Unmarshal(raw, &settings); err != nil {
				return nil, fmt.Errorf("invalid collaboration settings")
			}
		}
		if settings == nil {
			settings = map[string]json.RawMessage{}
		}
		if p.Model != "" {
			put(settings, "model", p.Model)
		} else {
			delete(settings, "model")
		}
		if p.Effort != "" {
			put(settings, "reasoning_effort", p.Effort)
		} else {
			put(settings, "reasoning_effort", nil)
		}
		put(mode, "settings", settings)
		put(params, "collaborationMode", mode)
	}
	put(msg, "params", params)
	return json.Marshal(msg)
}

func (p rpcPolicy) resolveDefault(ctx context.Context) (rpcPolicy, error) {
	if p.Model != "" {
		return p, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--listen", "stdio://")
	input, err := cmd.StdinPipe()
	if err != nil {
		return p, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return p, err
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		return p, err
	}
	defer func() { input.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	encoder := json.NewEncoder(input)
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	request := func(id int, method string, params any) (json.RawMessage, error) {
		if err := encoder.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for scanner.Scan() {
			var msg struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if json.Unmarshal(scanner.Bytes(), &msg) != nil || msg.ID != id {
				continue
			}
			if len(msg.Error) > 0 && string(msg.Error) != "null" {
				return nil, fmt.Errorf("HQ Codex default model lookup failed; select an explicit model")
			}
			return msg.Result, nil
		}
		return nil, fmt.Errorf("HQ Codex default model unavailable; select an explicit model")
	}
	if _, err = request(1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "hq-preset-default", "version": "1"}}); err != nil {
		return p, err
	}
	if err = encoder.Encode(map[string]any{"method": "initialized"}); err != nil {
		return p, err
	}
	cursor := ""
	for id := 2; id < 12; id++ {
		params := map[string]any{"includeHidden": true, "limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := request(id, "model/list", params)
		if err != nil {
			return p, err
		}
		var page struct {
			Data []struct {
				ID        string `json:"id"`
				IsDefault bool   `json:"isDefault"`
				Effort    string `json:"defaultReasoningEffort"`
			} `json:"data"`
			NextCursor string `json:"nextCursor"`
		}
		if err = json.Unmarshal(raw, &page); err != nil {
			return p, fmt.Errorf("HQ Codex model catalog is invalid")
		}
		for _, model := range page.Data {
			if model.IsDefault && model.ID != "" {
				p.Model = model.ID
				if p.Effort == "" {
					p.Effort = model.Effort
				}
				return p, nil
			}
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	return p, fmt.Errorf("HQ Codex default model unavailable; select an explicit model")
}

func (p rpcPolicy) forward(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), 96<<20)
	for scanner.Scan() {
		line, err := p.apply(scanner.Bytes())
		if err != nil {
			return err
		}
		if _, err = output.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return scanner.Err()
}
