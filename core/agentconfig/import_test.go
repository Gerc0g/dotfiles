package agentconfig

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestImportPortablePreferencesAndHTTPSMCPWithoutMachineSecrets(t *testing.T) {
	fixture(t)
	preview, err := PreviewImport("work", `model = "gpt-6-astra"
model_reasoning_effort = "ultra"
service_tier = "default"
approvals_reviewer = "user"
[features]
js_repl = false
[mcp_servers.docs]
url = "https://developers.openai.com/mcp"
[mcp_servers.local]
command = "/Applications/Private.app/tool"
[mcp_servers.secret]
url = "https://example.com/mcp"
bearer_token_env_var = "PRIVATE_TOKEN"
`)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(preview.Preset)
	text := string(raw)
	for _, want := range []string{`"serviceTier":"default"`, `"approvalsReviewer":"user"`, `"jsRepl":false`, `"url":"https://developers.openai.com/mcp"`} {
		if !strings.Contains(text, want) {
			t.Errorf("portable setting missing %s: %s", want, text)
		}
	}
	for _, forbidden := range []string{"Private.app", "PRIVATE_TOKEN", "https://example.com/mcp"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("unsafe MCP retained %s", forbidden)
		}
	}
	if len(preview.Preset.MCP) != 1 {
		t.Fatalf("wrong MCP count: %d", len(preview.Preset.MCP))
	}
	rendered, err := render(preview.Preset)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"service_tier = 'default'", "approvals_reviewer = 'user'", "js_repl = false", "url = 'https://developers.openai.com/mcp'"} {
		if !strings.Contains(string(rendered), want) {
			t.Errorf("not rendered: %s", want)
		}
	}
}

func TestMCPURLPolicyRejectsCredentialsPrivateHostsAndMixedTransport(t *testing.T) {
	fixture(t)
	for _, endpoint := range []string{"http://example.com/mcp", "https://user:secret@example.com/mcp", "https://example.com/mcp?token=secret", "https://127.0.0.1/mcp", "https://internal/mcp", "https://db.local/mcp"} {
		s, _ := Read()
		raw, _ := json.Marshal(map[string]any{"name": "test", "url": endpoint, "enabled": true, "args": []string{}})
		var m MCP
		_ = json.Unmarshal(raw, &m)
		p := s.Presets["work"]
		p.MCP = []MCP{m}
		s.Presets["work"] = p
		if _, err := Save(s, s.Revision); err == nil {
			t.Errorf("unsafe MCP URL accepted: %s", endpoint)
		}
	}
}

func TestMCPRejectsMixedTransportsAndURLArguments(t *testing.T) {
	fixture(t)
	for _, m := range []MCP{{Name: "mixed", Command: "sh", URL: "https://example.com/mcp"}, {Name: "args", URL: "https://example.com/mcp", Args: []string{"secret"}}} {
		s, _ := Read()
		p := s.Presets["work"]
		p.MCP = []MCP{m}
		s.Presets["work"] = p
		if _, err := Save(s, s.Revision); err == nil {
			t.Fatal("mixed MCP transport accepted")
		}
	}
}
