package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveScopesAndChildren(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HQ_ARCHIVE_ROOT", root)
	t.Setenv("PROKECTFILES_ROOT", "/srv/projects")
	entries := []archiveRecord{{ID: "parent", File: "codex/sessions/parent.jsonl", MappedCwd: "/srv/projects/acme/p/r"}, {ID: "child", File: "codex/sessions/child.jsonl", MappedCwd: "/srv/projects/acme/p/r"}, {ID: "other", File: "codex/sessions/other.jsonl", MappedCwd: "/srv/projects/other/p/r"}, {ID: "research", File: "codex-research/sessions/research.jsonl", MappedCwd: "/old/research"}}
	data, _ := json.Marshal(entries)
	writeFixture(t, root, "catalog.json", string(data))
	for _, r := range entries {
		meta := map[string]any{"id": r.ID}
		if r.ID == "child" {
			meta["source"] = map[string]any{"subagent": map[string]any{"thread_spawn": map[string]any{"parent_thread_id": "parent"}}}
		}
		line, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": meta})
		writeFixture(t, root, r.File, string(line)+"\n")
	}
	s := Scope{Kind: "company", Company: "acme"}
	list := call(t, "history.list", map[string]any{"scope": s}).(HistoryList)
	if len(list.Threads) != 1 || len(list.Threads[0].Children) != 1 || list.Threads[0].Children[0].ID != "child" {
		t.Fatal(list)
	}
	b, _ := json.Marshal(map[string]any{"scope": Scope{Kind: "research"}, "id": "parent"})
	if _, e := Dispatch("history.get", b); e == nil {
		t.Fatal("company archive leaked to research")
	}
	outside := t.TempDir()
	writeFixture(t, outside, "outside.jsonl", "secret")
	os.Remove(filepath.Join(root, entries[0].File))
	os.Symlink(filepath.Join(outside, "outside.jsonl"), filepath.Join(root, entries[0].File))
	b, _ = json.Marshal(map[string]any{"scope": s, "id": "parent"})
	if _, e := Dispatch("history.get", b); e == nil {
		t.Fatal("symlink escape accepted")
	}
}
func TestArchiveAttachmentMustBeExactReferencedRegularFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HQ_ARCHIVE_ROOT", root)
	entry := archiveRecord{ID: "personal", File: "codex/sessions/personal.jsonl", MappedCwd: "/unmapped"}
	data, _ := json.Marshal([]archiveRecord{entry})
	writeFixture(t, root, "catalog.json", string(data))
	writeFixture(t, root, entry.File, "{\"type\":\"session_meta\",\"payload\":{\"id\":\"personal\"}}\n{\"text\":\"/old/.codex/attachments/image.png.other\"}\n")
	writeFixture(t, root, "codex/attachments/image.png", "image fixture")
	args, _ := json.Marshal(Request{Scope: Scope{Kind: "personal"}, ID: "codex/attachments/image.png"})
	if _, err := Dispatch("history.attachment", args); err == nil {
		t.Fatal("partial filename reference authorized attachment")
	}
	writeFixture(t, root, entry.File, "{\"type\":\"session_meta\",\"payload\":{\"id\":\"personal\"}}\n{\"text\":\"/old/.codex/attachments/image.png\"}\n")
	if _, err := Dispatch("history.attachment", args); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "codex/auth.json", "private fixture")
	os.Remove(filepath.Join(root, "codex/attachments/image.png"))
	os.Symlink("../auth.json", filepath.Join(root, "codex/attachments/image.png"))
	if _, err := Dispatch("history.attachment", args); err == nil {
		t.Fatal("attachment alias reached non-attachment archive file")
	}
}
