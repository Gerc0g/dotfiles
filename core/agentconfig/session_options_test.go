package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestSessionOptionsMaterializeOnlyTaskSnapshot(t *testing.T) {
	t.Setenv("HQ_DATA_ROOT", t.TempDir())
	settings, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	preset := settings.Presets["work"]
	preset.Model = "preset-model"
	preset.MCP = []MCP{{Name: "docs", Command: "docs", Args: []string{}, Enabled: true}, {Name: "lookup", Command: "lookup", Args: []string{}, Enabled: false}}
	settings.Presets["work"] = preset
	settings, err = Save(settings, settings.Revision)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(settingsFile())
	if err != nil {
		t.Fatal(err)
	}
	options, err := ParseSessionOptions(`{"v":1,"model":"session-model","effort":"ultra","network":false,"webSearch":"disabled","permissionMode":"yolo","mcpSelection":{"v":1,"managedServersEnabled":false,"forceIncludeServerIds":["lookup"],"forceExcludeServerIds":[]}}`)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "task")
	effective, err := Materialize("work", destination, options)
	if err != nil {
		t.Fatal(err)
	}
	if effective.Model != "session-model" || effective.Effort != "ultra" || effective.Network || effective.WebSearch != "disabled" || effective.MCP[0].Enabled || !effective.MCP[1].Enabled {
		t.Fatalf("overlay not applied: %+v", effective)
	}
	after, err := os.ReadFile(settingsFile())
	if err != nil || string(after) != string(before) {
		t.Fatal("shared preset changed")
	}
	raw, err := os.ReadFile(filepath.Join(destination, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err = toml.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if config["model"] != "session-model" || config["approval_policy"] != "never" || config["sandbox_mode"] != "danger-full-access" {
		t.Fatalf("task preferences not rendered: %s", raw)
	}
	restored, err := Materialize("work", filepath.Join(t.TempDir(), "second-task"))
	if err != nil || restored.Model != "preset-model" || !restored.MCP[0].Enabled || restored.MCP[1].Enabled {
		t.Fatalf("snapshot leaked into next task: %+v %v", restored, err)
	}
}

func TestSessionOptionsRejectSecretsUnknownMCPAndInvalidPreferences(t *testing.T) {
	t.Setenv("HQ_DATA_ROOT", t.TempDir())
	for _, raw := range []string{`{"v":1,"auth":{}}`, `{"v":1,"mcpSelection":{"v":1,"managedServersEnabled":true,"forceIncludeServerIds":["foreign"],"forceExcludeServerIds":[]}}`, `{"v":1,"model":"../host"}`, `{"v":1,"permissionMode":"unchecked"}`, `{"v":2}`, `{"v":1} {"v":1}`} {
		t.Run(raw, func(t *testing.T) {
			options, err := ParseSessionOptions(raw)
			if err == nil {
				_, err = Materialize("work", filepath.Join(t.TempDir(), "task"), options)
			}
			if err == nil {
				t.Fatal("invalid session options accepted")
			}
		})
	}
	// An explicit empty model means use the Codex default; absence inherits preset.
	options, err := ParseSessionOptions(`{"v":1,"model":""}`)
	if err != nil || options.Model == nil || *options.Model != "" {
		t.Fatalf("empty model lost: %+v %v", options, err)
	}
	var encoded map[string]any
	raw, _ := json.Marshal(options)
	json.Unmarshal(raw, &encoded)
	if encoded["model"] != "" {
		t.Fatal("empty override omitted")
	}
}
