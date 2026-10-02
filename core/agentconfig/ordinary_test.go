package agentconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacySettingsGainIndependentOrdinaryWithoutWriting(t *testing.T) {
	fixture(t)
	legacy := defaults()
	delete(legacy.Presets, "ordinary")
	work := legacy.Presets["work"]
	work.Model, work.Effort, work.Instructions = "gpt-work", "ultra", "PRIVATE COMPANY INSTRUCTIONS"
	work.Skills = []string{"source-ingest"}
	legacy.Presets["work"] = work
	raw, _ := json.Marshal(legacy)
	os.MkdirAll(filepath.Dir(settingsFile()), 0700)
	os.WriteFile(settingsFile(), raw, 0600)
	got, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	ordinary, ok := got.Presets["ordinary"]
	if !ok {
		t.Fatal("ordinary missing from legacy settings")
	}
	if ordinary.Model != work.Model || ordinary.Effort != work.Effort || ordinary.Instructions == work.Instructions || ordinary.Instructions == "" || len(ordinary.Skills) != 0 || len(ordinary.MCP) != 0 || ordinary.Hooks != (Hooks{}) {
		t.Fatalf("unsafe ordinary default: %+v", ordinary)
	}
	if !reflect.DeepEqual(got.Presets["work"], work) || got.Revision != revision(raw) {
		t.Fatal("read changed Work or persisted revision")
	}
	after, _ := os.ReadFile(settingsFile())
	if !bytes.Equal(raw, after) {
		t.Fatal("read wrote settings")
	}
	effective, err := Materialize("ordinary", filepath.Join(t.TempDir(), "profile"))
	if err != nil || effective.Model != work.Model || effective.Hooks != (Hooks{}) {
		t.Fatalf("ordinary cannot materialize: %+v %v", effective, err)
	}
	// A client opened before deployment can still save its original two-preset
	// payload using the revision of the old file.
	saved, err := Save(legacy, revision(raw))
	if err != nil || !reflect.DeepEqual(saved.Presets["ordinary"], ordinary) {
		t.Fatalf("pre-upgrade client could not save: %+v %v", saved, err)
	}
}

func TestOldClientSavePreservesOrdinaryAndRevisionCheck(t *testing.T) {
	fixture(t)
	current, _ := Read()
	ordinary := current.Presets["ordinary"]
	ordinary.Model, ordinary.Effort, ordinary.Instructions = "gpt-ordinary", "low", "Independent personal instructions"
	current.Presets["ordinary"] = ordinary
	saved, err := Save(current, current.Revision)
	if err != nil {
		t.Fatal(err)
	}
	legacy := saved
	legacy.Presets = map[string]Preset{"work": saved.Presets["work"], "research": saved.Presets["research"]}
	work := legacy.Presets["work"]
	work.Model = "gpt-work-new"
	legacy.Presets["work"] = work
	next, err := Save(legacy, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(next.Presets["ordinary"], ordinary) || next.Presets["work"].Model != work.Model {
		t.Fatal("old client lost ordinary settings or update")
	}
	if _, err := Save(legacy, saved.Revision); err == nil {
		t.Fatal("stale legacy save accepted")
	}
	if _, exists := legacy.Presets["ordinary"]; exists {
		t.Fatal("save mutated caller draft")
	}
}
