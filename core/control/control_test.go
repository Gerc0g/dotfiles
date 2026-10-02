package control

import (
	"bytes"
	"encoding/json"
	"github.com/Gerc0g/dotfiles/core/agentconfig"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnerBoundaryRejectsUnknownVersionAndDoesNotLeakArguments(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HQ_DATA_ROOT", root)
	for _, raw := range []string{`{"v":2,"operation":"agent.get","args":{}}`, `{"v":1,"operation":"shell","args":{"secret":"PRIVATE_TEST_TOKEN"}}`, `{"v":1,"operation":"system.status","args":{},"command":"id"}`} {
		var out bytes.Buffer
		if err := Serve(strings.NewReader(raw), &out); err != nil {
			t.Fatal(err)
		}
		var response Response
		if err := json.Unmarshal(out.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.OK || strings.Contains(out.String(), "PRIVATE_TEST_TOKEN") {
			t.Fatalf("unsafe response %s", out.String())
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("PRIVATE_TEST_TOKEN")) {
		t.Fatal("secret copied into audit")
	}
}

func TestSystemStatusUsesVersionedResponse(t *testing.T) {
	t.Setenv("HQ_DATA_ROOT", t.TempDir())
	r := Dispatch(Request{1, "system.status", json.RawMessage(`{}`)})
	if !r.OK || r.Version != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestUnavailableAuditPreventsMutation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HQ_DATA_ROOT", root)
	if err := os.Mkdir(filepath.Join(root, "control.sqlite"), 0700); err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.Read()
	if err != nil {
		t.Fatal(err)
	}
	work := settings.Presets["work"]
	work.Model = "gpt-5.6-sol"
	settings.Presets["work"] = work
	args, err := json.Marshal(map[string]any{"settings": settings, "revision": settings.Revision})
	if err != nil {
		t.Fatal(err)
	}
	result := Dispatch(Request{Version: 1, Operation: "agent.save", Args: args})
	if result.OK || result.Error == nil || result.Error.Code != "HQ_AUDIT_UNAVAILABLE" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err = os.Stat(filepath.Join(root, "settings", "agent.json")); !os.IsNotExist(err) {
		t.Fatal("mutation executed before audit availability was established")
	}
}
