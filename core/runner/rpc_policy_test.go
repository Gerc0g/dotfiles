package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedPresetWinsOverThreadAndTurnOverrides(t *testing.T) {
	p := rpcPolicy{Model: "preset-model", Effort: "ultra", ServiceTier: "default", WebSearch: "disabled"}
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork", "turn/start"} {
		for _, overrides := range []string{`{}`, `{"model":"foreign","effort":"low","serviceTier":"fast","config":{"model_reasoning_effort":"low","web_search":"live","unrelated":true},"collaborationMode":{"mode":"plan","settings":{"model":"foreign","reasoning_effort":"low","developer_instructions":"keep"}}}`} {
			input := []byte(`{"id":"opaque-id","method":"` + method + `","params":` + overrides + `}`)
			out, err := p.apply(input)
			if err != nil {
				t.Fatal(err)
			}
			var msg struct {
				ID     string         `json:"id"`
				Params map[string]any `json:"params"`
			}
			if err = json.Unmarshal(out, &msg); err != nil {
				t.Fatal(err)
			}
			params := msg.Params
			if msg.ID != "opaque-id" || params["model"] != p.Model || params["serviceTier"] != p.ServiceTier {
				t.Fatalf("%s %s", method, out)
			}
			if method == "turn/start" {
				if params["effort"] != p.Effort || params["serviceTierForTurn"] != p.ServiceTier {
					t.Fatalf("%s", out)
				}
			} else {
				config := params["config"].(map[string]any)
				if config["web_search"] != p.WebSearch || config["model_reasoning_effort"] != p.Effort {
					t.Fatalf("%s", out)
				}
			}
			if mode, ok := params["collaborationMode"].(map[string]any); ok {
				settings := mode["settings"].(map[string]any)
				if settings["model"] != p.Model || settings["reasoning_effort"] != p.Effort || settings["developer_instructions"] != "keep" {
					t.Fatalf("%s", out)
				}
			}
		}
	}
}

func TestAutomaticPresetResolvesCodexDefaultBeforeAcceptingClientModel(t *testing.T) {
	bin := t.TempDir()
	script := `#!/bin/sh
while IFS= read -r line; do
 case "$line" in
  *'"method":"initialize"'*) printf '%s\n' '{"id":1,"result":{}}' ;;
  *'"method":"model/list"'*) printf '%s\n' '{"id":2,"result":{"data":[{"id":"foreign-first","isDefault":false},{"id":"codex-default","isDefault":true,"defaultReasoningEffort":"high"}],"nextCursor":null}}' ;;
 esac
done
`
	os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	p, err := (rpcPolicy{WebSearch: "cached"}).resolveDefault(context.Background())
	if err != nil || p.Model != "codex-default" || p.Effort != "high" {
		t.Fatalf("%+v %v", p, err)
	}
	out, err := p.apply([]byte(`{"method":"turn/start","params":{"model":"foreign","collaborationMode":{"mode":"plan","settings":{"model":"foreign"}}}}`))
	if err != nil || bytes.Contains(out, []byte("foreign")) || !bytes.Contains(out, []byte("codex-default")) {
		t.Fatalf("%s %v", out, err)
	}
}

func TestRPCPolicyForwardsApprovalRepliesAndManagedTurnAsJSONL(t *testing.T) {
	p := rpcPolicy{Model: "model", Effort: "high", WebSearch: "cached"}
	input := `{"id":"approval","result":{"decision":"accept"}}` + "\n" + `{"id":3,"method":"turn/start","params":{"threadId":"t","input":[{"type":"text","text":"body"}],"serviceTierForTurn":"fast"}}` + "\n"
	var output bytes.Buffer
	if err := p.forward(strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 || lines[0] != strings.Split(input, "\n")[0] {
		t.Fatalf("%s", output.String())
	}
	var turn map[string]any
	json.Unmarshal([]byte(lines[1]), &turn)
	params := turn["params"].(map[string]any)
	if params["serviceTierForTurn"] != "default" || params["threadId"] != "t" {
		t.Fatalf("%s", lines[1])
	}
	if params["input"].([]any)[0].(map[string]any)["text"] != "body" {
		t.Fatal("user input changed")
	}
}

func TestRPCPolicyPreservesUnrelatedMessagesByteForByte(t *testing.T) {
	p := rpcPolicy{Model: "preset-model", Effort: "high", WebSearch: "cached"}
	for _, input := range []string{`{ "id":42,"method":"initialize","params":{"clientInfo":{"name":"x","version":"1"}}}`, `{"id":999999999999999999,"result":{"decision":"accept"}}`, `{"method":"turn/steer","params":{"threadId":"t","input":[{"type":"text","text":"keep me"}]}}`} {
		out, err := p.apply([]byte(input))
		if err != nil || string(out) != input {
			t.Fatalf("%s %v", out, err)
		}
	}
}
