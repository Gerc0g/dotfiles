package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWorkScopeRetainsLegacyScopeDiscovery(t *testing.T) {
	root := fixture(t)
	writeFixture(t, root, "dev/20-projects/other/p/repos/r/index.md", "# Other")
	scopes := call(t, "knowledge.scopes", map[string]any{}).([]Scope)
	want := map[Scope]bool{
		{Kind: "work"}:                      true,
		{Kind: "research"}:                  true,
		{Kind: "vault"}:                     true,
		{Kind: "company", Company: "acme"}:  true,
		{Kind: "company", Company: "other"}: true,
	}
	got := make(map[Scope]bool)
	for _, scope := range scopes {
		got[scope] = true
	}
	if len(scopes) != len(want) || !reflect.DeepEqual(got, want) {
		t.Fatalf("scopes = %+v", scopes)
	}
}

func TestWorkScopeReadsEditsAndGraphsOnlyDev(t *testing.T) {
	root := fixture(t)
	writeFixture(t, root, "dev/10-personal/guide.md", "# Work guide\n[[20-projects/acme/p/repos/r/lessons]] [[Research only]]")
	writeFixture(t, root, "research/topics/ml/private.md", "# Research only")
	writeFixture(t, root, "personal/private.md", "# Unrelated zone")
	scope := Scope{Kind: "work"}
	listed := call(t, "knowledge.list", Request{Scope: scope}).(ListResult)
	if listed.Total != 2 {
		t.Fatalf("work list = %+v", listed)
	}
	for _, note := range listed.Notes {
		if strings.HasPrefix(note.ID, "dev/") || strings.Contains(note.Title, "Research") || strings.Contains(note.Title, "Unrelated") {
			t.Fatalf("wrong work-relative note: %+v", note)
		}
	}
	graph := call(t, "knowledge.graph", Request{Scope: scope}).(Graph)
	foundWorkEdge := false
	for _, edge := range graph.Edges {
		if edge.Source == "10-personal/guide.md" && edge.Target == "20-projects/acme/p/repos/r/lessons.md" {
			foundWorkEdge = true
		}
	}
	if !foundWorkEdge {
		t.Fatalf("work graph failed to resolve local link: %+v", graph)
	}
	for _, node := range graph.Nodes {
		if node.Title == "Research only" && !node.Missing {
			t.Fatal("work graph resolved a research note")
		}
	}
	note := call(t, "knowledge.get", Request{Scope: scope, ID: "10-personal/guide.md"}).(Note)
	saved := call(t, "knowledge.save", Request{Scope: scope, ID: note.ID, Revision: note.Revision, Content: "# Updated work guide"}).(Note)
	if saved.Content != "# Updated work guide" || saved.Revision == note.Revision {
		t.Fatalf("save = %+v", saved)
	}
	request, _ := json.Marshal(Request{Scope: scope, ID: note.ID, Revision: note.Revision, Content: "# Stale"})
	if _, err := Dispatch("knowledge.save", request); err == nil {
		t.Fatal("work save accepted a stale revision")
	}
	research := call(t, "knowledge.list", Request{Scope: Scope{Kind: "research"}}).(ListResult)
	if research.Total != 3 {
		t.Fatalf("research scope changed: %+v", research)
	}
}

func TestWorkScopeRejectsResearchTraversalAndAliases(t *testing.T) {
	root := fixture(t)
	if err := os.Symlink("../research", filepath.Join(root, "dev", "research-alias")); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../research/topics/ml/one.md", "research-alias/topics/ml/one.md", "/research/topics/ml/one.md"} {
		for _, operation := range []string{"knowledge.get", "knowledge.save", "knowledge.attachment"} {
			request, _ := json.Marshal(Request{Scope: Scope{Kind: "work"}, ID: id, Revision: "any", Content: "# Replaced"})
			if _, err := Dispatch(operation, request); err == nil {
				t.Fatalf("%s accepted %s", operation, id)
			}
		}
	}
	listed := call(t, "knowledge.list", Request{Scope: Scope{Kind: "work"}}).(ListResult)
	if listed.Total != 1 {
		t.Fatalf("work scan followed research symlink: %+v", listed)
	}
	request, _ := json.Marshal(Request{Scope: Scope{Kind: "work", Company: "acme"}})
	if _, err := Dispatch("knowledge.list", request); err == nil {
		t.Fatal("aggregate work scope accepted a company selector")
	}
	if err := os.Rename(filepath.Join(root, "dev"), filepath.Join(root, "original-dev")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("research", filepath.Join(root, "dev")); err != nil {
		t.Fatal(err)
	}
	request, _ = json.Marshal(Request{Scope: Scope{Kind: "work"}})
	if _, err := Dispatch("knowledge.list", request); err == nil {
		t.Fatal("work root followed a research alias")
	}
}

func TestWorkScopeCannotRunAggregateMaintenanceOrResearchContext(t *testing.T) {
	root := fixture(t)
	scope := Scope{Kind: "work"}
	jobs := call(t, "knowledge.jobs", Request{Scope: scope}).([]Job)
	if len(jobs) != 0 {
		t.Fatalf("aggregate work jobs = %+v", jobs)
	}
	for _, operation := range []string{"knowledge.runJob", "knowledge.context"} {
		request, _ := json.Marshal(Request{Scope: scope, Kind: "index"})
		if _, err := Dispatch(operation, request); err == nil {
			t.Fatalf("work allowed %s", operation)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "dev", "index.md")); !os.IsNotExist(err) {
		t.Fatalf("aggregate work index was created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataRoot(), "knowledge.sqlite")); !os.IsNotExist(err) {
		t.Fatalf("aggregate work operations created a job journal: %v", err)
	}
}
