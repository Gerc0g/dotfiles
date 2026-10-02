package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HQ_WIKI_ROOT", root)
	t.Setenv("HQ_DATA_ROOT", t.TempDir())
	for p, s := range map[string]string{"research/topics/ml/one.md": "---\naliases: [First]\n---\n# One\n[[two]] [[Missing]]", "research/topics/ml/two.md": "# Two\n[[First]]", "dev/20-projects/acme/p/repos/r/lessons.md": "# Confidential"} {
		writeFixture(t, root, p, s)
	}
	return root
}
func writeFixture(t *testing.T, root, p, s string) {
	t.Helper()
	f := filepath.Join(root, p)
	if err := os.MkdirAll(filepath.Dir(f), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte(s), 0600); err != nil {
		t.Fatal(err)
	}
}
func call(t *testing.T, op string, args any) any {
	t.Helper()
	b, _ := json.Marshal(args)
	r, e := Dispatch(op, b)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestScopeAndGraph(t *testing.T) {
	fixture(t)
	s := Scope{Kind: "research"}
	result := call(t, "knowledge.list", map[string]any{"scope": s}).(ListResult)
	if len(result.Notes) != 2 {
		t.Fatal(result)
	}
	g := call(t, "knowledge.graph", map[string]any{"scope": s}).(Graph)
	if len(g.Nodes) != 3 || len(g.Edges) != 3 {
		t.Fatal(g)
	}
	for _, n := range g.Nodes {
		if strings.Contains(n.Title, "Confidential") {
			t.Fatal("company memory leaked")
		}
	}
}
func TestOwnerVaultIncludesOriginalFoldersWithoutResearchExpansion(t *testing.T) {
	root := fixture(t)
	writeFixture(t, root, "dev/10-personal/note.md", "# Original personal note")
	writeFixture(t, root, "dev/20-projects/_templates/repo.md", "# Original template")
	all := call(t, "knowledge.list", map[string]any{"scope": Scope{Kind: "vault"}}).(ListResult)
	if all.Total != 5 {
		t.Fatalf("owner vault omitted imported notes: %d", all.Total)
	}
	research := call(t, "knowledge.list", map[string]any{"scope": Scope{Kind: "research"}}).(ListResult)
	if research.Total != 2 {
		t.Fatal("research scope expanded")
	}
	for _, op := range []string{"knowledge.context", "knowledge.runJob"} {
		b, _ := json.Marshal(Request{Scope: Scope{Kind: "vault"}, Kind: "index"})
		if _, err := Dispatch(op, b); err == nil {
			t.Fatalf("vault allowed automatic scoped operation %s", op)
		}
	}
}
func TestTraversalSymlinkAndCAS(t *testing.T) {
	root := fixture(t)
	s := Scope{Kind: "research"}
	outside := t.TempDir()
	writeFixture(t, outside, "secret.md", "secret")
	os.Symlink(outside, filepath.Join(root, "research", "escape"))
	for _, id := range []string{"../dev/20-projects/acme/p/repos/r/lessons.md", "escape/secret.md", "/etc/passwd"} {
		b, _ := json.Marshal(map[string]any{"scope": s, "id": id})
		if _, e := Dispatch("knowledge.get", b); e == nil {
			t.Fatalf("accepted %s", id)
		}
	}
	n := call(t, "knowledge.get", map[string]any{"scope": s, "id": "topics/ml/one.md"}).(Note)
	saved := call(t, "knowledge.save", map[string]any{"scope": s, "id": n.ID, "revision": n.Revision, "content": "# Edited"}).(Note)
	if saved.Content != "# Edited" || saved.Revision == n.Revision {
		t.Fatal(saved)
	}
	b, _ := json.Marshal(map[string]any{"scope": s, "id": n.ID, "revision": n.Revision, "content": "lost update"})
	if _, e := Dispatch("knowledge.save", b); e == nil {
		t.Fatal("stale revision accepted")
	}
}
func TestReviewPreservesInbox(t *testing.T) {
	root := fixture(t)
	p := "dev/20-projects/acme/p/repos/r/_inbox.md"
	body := "# Inbox\nStatus: candidate\n"
	writeFixture(t, root, p, body)
	s := Scope{Kind: "company", Company: "acme"}
	j := call(t, "knowledge.runJob", map[string]any{"scope": s, "kind": "review"}).(Job)
	if j.Status != "needs_review" {
		t.Fatal(j)
	}
	data, _ := os.ReadFile(filepath.Join(root, p))
	if string(data) != body {
		t.Fatal("review mutated inbox")
	}
	jobs := call(t, "knowledge.jobs", map[string]any{"scope": s}).([]Job)
	if len(jobs) != 1 || jobs[0].ID != j.ID {
		t.Fatal(jobs)
	}
}

func TestHotJobKeepsScopedRulesAndReview(t *testing.T) {
	root := fixture(t)
	writeFixture(t, root, "dev/20-projects/acme/p/repos/r/rules/test.md", "---\npaths: ['src/**']\n---\n# Repo rule\n")
	writeFixture(t, root, "dev/20-projects/acme/p/shared/rules/test.md", "---\npaths: ['src/**']\n---\n# Product rule\n")
	j := call(t, "knowledge.runJob", map[string]any{"scope": Scope{Kind: "company", Company: "acme"}, "kind": "hot"}).(Job)
	if j.Status != "succeeded" {
		t.Fatal(j)
	}
	b, e := os.ReadFile(filepath.Join(root, "dev/20-projects/acme/p/repos/r/hot.md"))
	if e != nil || !strings.Contains(string(b), "Repo rule") || !strings.Contains(string(b), "Product rule") {
		t.Fatalf("rules lost: %s %v", b, e)
	}
	if _, e := os.Stat(filepath.Join(root, "dev/20-projects/acme/p/repos/r/rules/hot.md")); !os.IsNotExist(e) {
		t.Fatal("hot created under rules")
	}
}

func TestResearchRootCannotAliasCompany(t *testing.T) {
	root := fixture(t)
	if err := os.Rename(filepath.Join(root, "research"), filepath.Join(root, "original-research")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("dev/20-projects/acme", filepath.Join(root, "research")); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{"scope": Scope{Kind: "research"}})
	if _, err := Dispatch("knowledge.list", b); err == nil {
		t.Fatal("research root followed company alias")
	}
}

func TestJobsRunningConcurrencyAndInterruptedRecovery(t *testing.T) {
	fixture(t)
	s := Scope{Kind: "research"}
	release, e := acquireScopeLock(s)
	if e != nil {
		t.Fatal(e)
	}
	db, e := openJournal()
	if e != nil {
		t.Fatal(e)
	}
	if e = saveJob(db, Job{ID: "interrupted", Scope: s, Kind: "lint", Status: "running", StartedAt: "2026-01-01T00:00:00Z"}); e != nil {
		t.Fatal(e)
	}
	db.Close()
	jobs := call(t, "knowledge.jobs", map[string]any{"scope": s}).([]Job)
	if jobs[0].Status != "running" {
		t.Fatal(jobs)
	}
	b, _ := json.Marshal(map[string]any{"scope": s, "kind": "lint"})
	if _, e := Dispatch("knowledge.runJob", b); e == nil {
		t.Fatal("concurrent job accepted")
	}
	release()
	jobs = call(t, "knowledge.jobs", map[string]any{"scope": s}).([]Job)
	if jobs[0].Status != "failed" || jobs[0].Error == "" {
		t.Fatal(jobs)
	}
}
func TestScheduledJobsStayScopedAndPreserveReview(t *testing.T) {
	root := fixture(t)
	writeFixture(t, root, "dev/20-projects/acme/p/repos/r/_inbox.md", "# Inbox\nStatus: candidate\n")
	report := call(t, "knowledge.tick", map[string]any{}).(ScheduledReport)
	if report.Failed != 0 || len(report.Jobs) != 7 {
		t.Fatal(report)
	}
	for _, job := range report.Jobs {
		if job.Scope.Kind == "vault" {
			t.Fatal("scheduler traversed the whole vault")
		}
	}
	if _, err := os.Stat(filepath.Join(root, "index.md")); !os.IsNotExist(err) {
		t.Fatal("scheduler wrote vault-wide index")
	}
	data, _ := os.ReadFile(filepath.Join(root, "dev/20-projects/acme/p/repos/r/_inbox.md"))
	if string(data) != "# Inbox\nStatus: candidate\n" {
		t.Fatal("scheduler promoted candidate")
	}
	db, err := openJournal()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var index string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='jobs_scope'").Scan(&index); err != nil {
		t.Fatal("scope journal index missing", err)
	}
}
func TestGraphResponseDisclosesLimit(t *testing.T) {
	nodes := make([]Node, 501)
	for i := range nodes {
		nodes[i] = Node{ID: fmt.Sprint(i)}
	}
	g := boundedGraph(Graph{Nodes: nodes, Edges: []Edge{{Source: "0", Target: "500"}, {Source: "0", Target: "1"}}}, 500)
	if len(g.Nodes) != 500 || g.TotalNodes != 501 || !g.Truncated || len(g.Edges) != 1 {
		t.Fatal(g)
	}
}
