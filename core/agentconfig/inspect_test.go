package agentconfig

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// This child implements only the diagnostic wire protocol. It observes the
// real process environment, materialized home, and auth copy used by Inspect.
func TestInspectProcess(t *testing.T) {
	marker := -1
	for i, a := range os.Args {
		if a == "hq-inspect-test-helper" {
			marker = i
			break
		}
	}
	if marker < 0 {
		return
	}
	logPath, mode := os.Args[marker+1], os.Args[marker+2]
	f, _ := os.Create(logPath)
	defer f.Close()
	scanner := bufio.NewScanner(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	log := json.NewEncoder(f)
	raw, _ := os.ReadFile(filepath.Join(os.Getenv("CODEX_HOME"), "config.toml"))
	var config map[string]any
	_ = toml.Unmarshal(raw, &config)
	_ = log.Encode(map[string]any{"home": os.Getenv("CODEX_HOME"), "inheritedSecret": os.Getenv("HQ_TEST_PARENT_SECRET"), "config": config})
	auth, _ := os.ReadFile(filepath.Join(os.Getenv("CODEX_HOME"), "auth.json"))
	_ = log.Encode(map[string]any{"authCopied": string(auth) == "server-private-auth"})
	_ = os.WriteFile(filepath.Join(os.Getenv("CODEX_HOME"), "auth.json"), []byte("changed-copy"), 0600)
	for scanner.Scan() {
		var req map[string]any
		_ = json.Unmarshal(scanner.Bytes(), &req)
		_ = log.Encode(req)
		if _, ok := req["id"]; !ok {
			continue
		}
		var result any
		switch req["method"] {
		case "initialize":
			result = map[string]any{"userAgent": "test"}
		case "config/read":
			if mode != "malformed" {
				result = map[string]any{"config": config}
			}
		case "account/read":
			result = map[string]any{"account": map[string]any{"email": "private-email", "token": string(auth)}}
		case "model/list":
			result = map[string]any{"data": []any{map[string]any{"id": "gpt-test", "displayName": "Test", "supportedReasoningEfforts": []any{map[string]any{"reasoningEffort": "high", "private": "private-model-data"}}}}}
		default:
			os.Exit(3)
		}
		_ = out.Encode(map[string]any{"id": req["id"], "result": result})
	}
	os.Exit(0)
}

func inspectFixture(t *testing.T, mode string) (string, string) {
	t.Helper()
	fixture(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "codex")
	logPath := filepath.Join(dir, "requests.jsonl")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	command := "#!/bin/sh\nexec " + quote(exe) + " -test.run=^TestInspectProcess$ -- hq-inspect-test-helper " + quote(logPath) + " " + quote(mode) + "\n"
	if err = os.WriteFile(script, []byte(command), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HQ_CODEX_BINARY", script)
	t.Setenv("HQ_TEST_PARENT_SECRET", "parent-private-data")
	authDir := t.TempDir()
	authFile := filepath.Join(authDir, "server-auth.json")
	if err = os.WriteFile(authFile, []byte("server-private-auth"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HQ_CODEX_AUTH_FILE", authFile)
	t.Setenv("HQ_CODEX_AUTH_HOME", t.TempDir())
	s, _ := Read()
	p := s.Presets["work"]
	p.Model = "gpt-test"
	p.MCP = []MCP{{Name: "test", Command: "sh", Args: []string{"-c", "forbidden host command"}, Enabled: true}}
	s.Presets["work"] = p
	if _, err = Save(s, s.Revision); err != nil {
		t.Fatal(err)
	}
	return logPath, authFile
}

func TestInspectUsesTemporaryAuthAndDisablesHostMCP(t *testing.T) {
	logPath, authFile := inspectFixture(t, "valid")
	result, err := Inspect("work")
	if err != nil {
		t.Fatal(err)
	}
	got := result.(map[string]any)
	if got["selectedModelAvailable"] != true || got["selectedEffortAvailable"] != true {
		t.Fatalf("model/effort not verified: %+v", got)
	}
	raw, _ := json.Marshal(result)
	for _, forbidden := range []string{"private-email", "server-private-auth", "private-model-data", "parent-private-data"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("private data returned: %s", forbidden)
		}
	}
	observed, _ := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(observed)), "\n")
	var first struct {
		Home      string         `json:"home"`
		Inherited string         `json:"inheritedSecret"`
		Config    map[string]any `json:"config"`
	}
	_ = json.Unmarshal([]byte(lines[0]), &first)
	if first.Inherited != "" {
		t.Fatal("inherited service environment")
	}
	servers := first.Config["mcp_servers"].(map[string]any)
	if servers["test"].(map[string]any)["enabled"] != false {
		t.Error("MCP could execute in host inspection")
	}
	if _, err = os.Stat(first.Home); !os.IsNotExist(err) {
		t.Error("diagnostic credential copy retained")
	}
	auth, _ := os.ReadFile(authFile)
	if string(auth) != "server-private-auth" {
		t.Error("active auth modified")
	}
	var methods []string
	var copied struct {
		AuthCopied bool `json:"authCopied"`
	}
	_ = json.Unmarshal([]byte(lines[1]), &copied)
	if !copied.AuthCopied {
		t.Fatal("configured server auth was not copied")
	}
	for _, line := range lines[2:] {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal([]byte(line), &req)
		methods = append(methods, req.Method)
		if req.Method == "account/read" && req.Params["refreshToken"] != false {
			t.Error("refresh requested")
		}
	}
	if strings.Join(methods, ",") != "initialize,initialized,config/read,account/read,model/list" {
		t.Fatalf("unexpected protocol: %v", methods)
	}
}
func TestInspectRejectsMissingConfigResult(t *testing.T) {
	inspectFixture(t, "malformed")
	if _, err := Inspect("work"); err == nil {
		t.Fatal("malformed config was reported accepted")
	}
}

func TestInspectDraftUsesUnsavedSettingsWithoutWriting(t *testing.T) {
	inspectFixture(t, "valid")
	before, _ := Read()
	draft := before
	draft.Presets = map[string]Preset{"work": before.Presets["work"], "research": before.Presets["research"]}
	p := draft.Presets["work"]
	p.Model = "draft-model"
	draft.Presets["work"] = p
	args, _ := json.Marshal(map[string]any{"preset": "work", "settings": draft})
	result, err := Dispatch("agent.inspect", args)
	if err != nil {
		t.Fatal(err)
	}
	got := result.(map[string]any)
	if got["source"] != "hq-draft" || got["effective"].(map[string]any)["model"] != "draft-model" {
		t.Fatalf("did not inspect desired draft: %+v", got)
	}
	after, _ := Read()
	if after.Revision != before.Revision || after.Presets["work"].Model != "gpt-test" {
		t.Fatal("inspection persisted the draft")
	}
}
