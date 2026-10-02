package agentconfig

import (
	"encoding/json"
	"errors"

	"github.com/pelletier/go-toml/v2"
)

// SessionOptions contains only portable, non-secret preferences. MCP selection
// refers to the preset's registered names, never client-supplied executables.
type SessionOptions struct {
	Version        int           `json:"v"`
	Model          *string       `json:"model,omitempty"`
	Effort         *string       `json:"effort,omitempty"`
	ServiceTier    *string       `json:"serviceTier,omitempty"`
	WebSearch      *string       `json:"webSearch,omitempty"`
	Network        *bool         `json:"network,omitempty"`
	PermissionMode *string       `json:"permissionMode,omitempty"`
	MCPSelection   *MCPSelection `json:"mcpSelection,omitempty"`
}

type MCPSelection struct {
	Version               int      `json:"v"`
	ManagedServersEnabled bool     `json:"managedServersEnabled"`
	ForceIncludeServerIDs []string `json:"forceIncludeServerIds"`
	ForceExcludeServerIDs []string `json:"forceExcludeServerIds"`
}

func ParseSessionOptions(raw string) (SessionOptions, error) {
	var options SessionOptions
	if err := decode([]byte(raw), &options); err != nil {
		return options, errors.New("HQ_SESSION_OPTIONS_INVALID")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil || fields == nil {
		return options, errors.New("HQ_SESSION_OPTIONS_INVALID")
	}
	for _, value := range fields {
		if string(value) == "null" {
			return options, errors.New("HQ_SESSION_OPTIONS_INVALID")
		}
	}
	if selection, ok := fields["mcpSelection"]; ok {
		var values map[string]json.RawMessage
		if err := json.Unmarshal(selection, &values); err != nil {
			return options, errors.New("HQ_SESSION_OPTIONS_INVALID")
		}
		for _, key := range []string{"v", "managedServersEnabled", "forceIncludeServerIds", "forceExcludeServerIds"} {
			if value, ok := values[key]; !ok || string(value) == "null" {
				return options, errors.New("HQ_SESSION_OPTIONS_INVALID")
			}
		}
	}
	return options, options.validate()
}

func (options SessionOptions) validate() error {
	if options.Version != 1 {
		return errors.New("HQ_SESSION_OPTIONS_VERSION")
	}
	if options.PermissionMode != nil {
		switch *options.PermissionMode {
		case "default", "acceptEdits", "bypassPermissions", "plan", "read-only", "safe-yolo", "yolo":
		default:
			return errors.New("HQ_SESSION_PERMISSION_INVALID")
		}
	}
	if options.MCPSelection != nil {
		if options.MCPSelection.Version != 1 {
			return errors.New("HQ_SESSION_MCP_VERSION")
		}
		for _, names := range [][]string{options.MCPSelection.ForceIncludeServerIDs, options.MCPSelection.ForceExcludeServerIDs} {
			for _, name := range names {
				if !namePattern.MatchString(name) {
					return errors.New("HQ_SESSION_MCP_NAME_INVALID")
				}
			}
		}
	}
	return nil
}

func (options SessionOptions) apply(preset string, effective Effective) (Effective, error) {
	if err := options.validate(); err != nil {
		return effective, err
	}
	if options.Model != nil {
		effective.Model = *options.Model
	}
	if options.Effort != nil {
		effective.Effort = *options.Effort
	}
	if options.ServiceTier != nil {
		effective.ServiceTier = *options.ServiceTier
	}
	if options.WebSearch != nil {
		effective.WebSearch = *options.WebSearch
	}
	if options.Network != nil {
		effective.Network = *options.Network
	}
	if options.PermissionMode != nil {
		effective.PermissionMode = *options.PermissionMode
	}
	if selection := options.MCPSelection; selection != nil {
		known := map[string]bool{}
		for _, server := range effective.MCP {
			known[server.Name] = true
		}
		included, excluded := map[string]bool{}, map[string]bool{}
		for _, name := range selection.ForceIncludeServerIDs {
			if !known[name] {
				return effective, errors.New("HQ_SESSION_MCP_UNKNOWN")
			}
			included[name] = true
		}
		for _, name := range selection.ForceExcludeServerIDs {
			if !known[name] {
				return effective, errors.New("HQ_SESSION_MCP_UNKNOWN")
			}
			excluded[name] = true
		}
		effective.MCP = append([]MCP(nil), effective.MCP...)
		for i, server := range effective.MCP {
			effective.MCP[i].Enabled = (server.Enabled && selection.ManagedServersEnabled || included[server.Name]) && !excluded[server.Name]
		}
	}
	settings, err := Read()
	if err != nil {
		return effective, err
	}
	settings.Presets[preset] = effective.Preset
	if err = validate(settings); err != nil {
		return effective, err
	}
	effective.Source = "hq-session"
	return effective, nil
}

func renderEffective(effective Effective) ([]byte, error) {
	raw, err := render(effective.Preset)
	if err != nil || effective.PermissionMode == "" || effective.PermissionMode == "default" {
		return raw, err
	}
	var config map[string]any
	if err = toml.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	switch effective.PermissionMode {
	case "yolo", "bypassPermissions":
		config["approval_policy"] = "never"
		config["sandbox_mode"] = "danger-full-access"
	case "read-only":
		config["approval_policy"] = "never"
		config["sandbox_mode"] = "read-only"
	case "safe-yolo":
		config["approval_policy"] = "on-request"
		config["approvals_reviewer"] = "auto_review"
	case "acceptEdits", "plan":
		config["approval_policy"] = "on-request"
		config["approvals_reviewer"] = "user"
	}
	return toml.Marshal(config)
}
