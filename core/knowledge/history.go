package knowledge

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type Thread struct {
	ID          string   `json:"id"`
	File        string   `json:"-"`
	OriginalCwd string   `json:"originalCwd"`
	MappedCwd   string   `json:"mappedCwd"`
	Timestamp   string   `json:"timestamp"`
	Title       string   `json:"title"`
	ParentID    string   `json:"parentId,omitempty"`
	Children    []Thread `json:"children"`
	Scope       Scope    `json:"scope"`
}
type archiveRecord struct {
	ID          string `json:"id"`
	File        string `json:"file"`
	OriginalCwd string `json:"originalCwd"`
	MappedCwd   string `json:"mappedCwd"`
	Timestamp   string `json:"timestamp"`
}
type HistoryList struct {
	Threads    []Thread `json:"threads"`
	Total      int      `json:"total"`
	NextOffset *int     `json:"nextOffset"`
}
type ArchiveDescriptor struct {
	Root         string `json:"root"`
	RelativeFile string `json:"relativeFile"`
}
type HistoryDescriptor struct {
	Thread    Thread            `json:"thread"`
	Archive   ArchiveDescriptor `json:"archive"`
	Cursor    string            `json:"cursor,omitempty"`
	Direction string            `json:"direction"`
}

func archiveRoot() string {
	if v := os.Getenv("HQ_ARCHIVE_ROOT"); v != "" {
		return v
	}
	return filepath.Join(dataRoot(), "imports", "mac-20260927")
}
func archiveScope(r archiveRecord) Scope {
	if strings.HasPrefix(r.File, "codex-research/") {
		return Scope{Kind: "research"}
	}
	work := os.Getenv("PROKECTFILES_ROOT")
	if work == "" {
		work = "/home/agent/Desktop/Prokectfiles"
	}
	rel, err := filepath.Rel(work, r.MappedCwd)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, "../") && !filepath.IsAbs(rel) {
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) > 0 && parts[0] == ".worktrees" {
			parts = parts[1:]
		}
		if len(parts) >= 3 && parts[0] != "" {
			return Scope{Kind: "company", Company: parts[0]}
		}
	}
	rel, err = filepath.Rel(filepath.Join(wikiRoot(), "dev", "20-projects"), r.MappedCwd)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, "../") && !filepath.IsAbs(rel) {
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) > 0 && parts[0] != "." {
			return Scope{Kind: "company", Company: parts[0]}
		}
	}
	if r.MappedCwd == filepath.Join(wikiRoot(), "research") || strings.HasPrefix(r.MappedCwd, filepath.Join(wikiRoot(), "research")+string(filepath.Separator)) {
		return Scope{Kind: "research"}
	}
	return Scope{Kind: "personal"}
}
func archiveThreads(root *os.Root, s Scope) ([]Thread, error) {
	if s.Kind != "research" && s.Kind != "company" && s.Kind != "personal" {
		return nil, errors.New("invalid archive scope")
	}
	if s.Kind == "company" && s.Company == "" || s.Kind != "company" && s.Company != "" {
		return nil, errors.New("invalid archive company")
	}
	data, err := readRegular(root, "catalog.json")
	if err != nil {
		return nil, err
	}
	var entries []archiveRecord
	if err = json.Unmarshal(data, &entries); err != nil {
		return nil, errors.New("archive catalog is invalid")
	}
	titles := archiveTitles(root)
	threads := []Thread{}
	for _, r := range entries {
		scope := archiveScope(r)
		if scope != s {
			continue
		}
		if err := safeID(r.File); err != nil {
			continue
		}
		if !strings.HasPrefix(r.File, "codex/sessions/") && !strings.HasPrefix(r.File, "codex-research/sessions/") {
			continue
		}
		t := Thread{ID: r.ID, File: r.File, OriginalCwd: r.OriginalCwd, MappedCwd: r.MappedCwd, Timestamp: r.Timestamp, Title: r.ID, Scope: scope, Children: []Thread{}}
		if title := titles[t.ID]; title != "" {
			t.Title = title
		}
		if err := readThreadMetadata(root, &t); err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	sort.Slice(threads, func(i, j int) bool { return threads[i].Timestamp > threads[j].Timestamp })
	return threads, nil
}
func readThreadMetadata(root *os.Root, t *Thread) error {
	if err := rejectSymlinkComponents(root, t.File); err != nil {
		return err
	}
	f, err := root.Open(t.File)
	if err != nil {
		return errors.New("archive rollout is unavailable")
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	line, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return err
	}
	var envelope struct {
		Type    string `json:"type"`
		Payload struct {
			ID           string          `json:"id"`
			ForkedFromID string          `json:"forked_from_id"`
			Source       json.RawMessage `json:"source"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &envelope) != nil || envelope.Type != "session_meta" || envelope.Payload.ID != t.ID {
		return errors.New("archive thread identity does not match catalog")
	}
	t.ParentID = envelope.Payload.ForkedFromID
	var source struct {
		Subagent struct {
			ThreadSpawn struct {
				ParentThreadID string `json:"parent_thread_id"`
			} `json:"thread_spawn"`
		} `json:"subagent"`
	}
	if json.Unmarshal(envelope.Payload.Source, &source) == nil && source.Subagent.ThreadSpawn.ParentThreadID != "" {
		t.ParentID = source.Subagent.ThreadSpawn.ParentThreadID
	}
	return nil
}
func dispatchHistory(op string, r Request) (any, error) {
	root, err := os.OpenRoot(archiveRoot())
	if err != nil {
		return nil, errors.New("Mac import archive is unavailable")
	}
	defer root.Close()
	threads, err := archiveThreads(root, r.Scope)
	if err != nil {
		return nil, err
	}
	switch op {
	case "history.list":
		byID := map[string]Thread{}
		for _, t := range threads {
			byID[t.ID] = t
		}
		roots := []Thread{}
		var attach func(Thread, map[string]bool) Thread
		attach = func(t Thread, seen map[string]bool) Thread {
			seen[t.ID] = true
			for _, child := range threads {
				if child.ParentID == t.ID && !seen[child.ID] {
					t.Children = append(t.Children, attach(child, seen))
				}
			}
			return t
		}
		q := strings.ToLower(r.Query)
		for _, t := range threads {
			_, hasParent := byID[t.ParentID]
			if hasParent && t.ParentID != t.ID {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(t.ID+" "+t.OriginalCwd+" "+t.Title), q) {
				continue
			}
			roots = append(roots, attach(t, map[string]bool{}))
		}
		start, end := pageBounds(len(roots), r.Offset, r.Limit)
		var next *int
		if end < len(roots) {
			next = &end
		}
		return HistoryList{roots[start:end], len(roots), next}, nil
	case "history.get", "history.continuation":
		for _, t := range threads {
			if t.ID == r.ID {
				f, err := root.Open(t.File)
				if err != nil {
					return nil, errors.New("archive rollout is unavailable")
				}
				info, err := f.Stat()
				f.Close()
				if err != nil || !info.Mode().IsRegular() {
					return nil, errors.New("archive rollout is invalid")
				}
				direction := r.Direction
				if direction == "" {
					direction = "older"
				}
				if direction != "older" && direction != "newer" {
					return nil, errors.New("invalid transcript direction")
				}
				return HistoryDescriptor{Thread: t, Archive: ArchiveDescriptor{Root: root.Name(), RelativeFile: t.File}, Cursor: r.Cursor, Direction: direction}, nil
			}
		}
		return nil, errors.New("thread is unavailable in this scope")
	case "history.attachment": // IDs are archive-relative attachment paths, never arbitrary old absolute paths.
		if err := safeID(r.ID); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(r.ID, "codex/attachments/") && !strings.HasPrefix(r.ID, "codex/generated_images/") {
			return nil, errors.New("unsupported archive attachment")
		}
		referenced := false
		for _, t := range threads {
			f, e := root.Open(t.File)
			if e != nil {
				continue
			}
			reader := bufio.NewReader(f)
			for {
				line, e := reader.ReadString('\n')
				if referencesAttachment(line, strings.TrimPrefix(r.ID, "codex/")) {
					referenced = true
					break
				}
				if e != nil {
					break
				}
			}
			f.Close()
			if referenced {
				break
			}
		}
		if !referenced {
			return nil, errors.New("attachment is not referenced in this scope")
		}
		return readAttachment(root, path.Clean(r.ID), r.Offset)
	default:
		return nil, errors.New("unsupported archive operation")
	}
}

func referencesAttachment(line, target string) bool {
	if !strings.Contains(line, target) {
		return false
	}
	var record any
	if json.Unmarshal([]byte(line), &record) != nil {
		return false
	}
	var visit func(any) bool
	visit = func(value any) bool {
		switch value := value.(type) {
		case string:
			for start := 0; start < len(value); {
				i := strings.Index(value[start:], target)
				if i < 0 {
					return false
				}
				i += start
				end := i + len(target)
				before := i == 0 || strings.ContainsRune("/ \t\r\n\"'`([", rune(value[i-1]))
				after := end == len(value) || strings.ContainsRune(" \t\r\n\"'`)]>?#", rune(value[end]))
				if before && after {
					return true
				}
				start = end
			}
		case []any:
			for _, child := range value {
				if visit(child) {
					return true
				}
			}
		case map[string]any:
			for _, child := range value {
				if visit(child) {
					return true
				}
			}
		}
		return false
	}
	return visit(record)
}

func archiveTitles(root *os.Root) map[string]string {
	titles := map[string]string{}
	for _, name := range []string{"codex/session_index.jsonl", "codex-research/session_index.jsonl"} {
		f, err := root.Open(name)
		if err != nil {
			continue
		}
		reader := bufio.NewReader(f)
		for {
			line, e := reader.ReadBytes('\n')
			var item struct {
				ID         string `json:"id"`
				ThreadName string `json:"thread_name"`
			}
			if json.Unmarshal(line, &item) == nil && item.ID != "" && item.ThreadName != "" {
				titles[item.ID] = item.ThreadName
			}
			if e != nil {
				break
			}
		}
		f.Close()
	}
	return titles
}
