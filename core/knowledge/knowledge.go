// Package knowledge exposes owner-controlled, scoped Markdown and imported history.
// Runner processes must not receive this owner API or select their own company scope.
package knowledge

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/research"
	"github.com/Gerc0g/dotfiles/core/world"
)

type Scope struct {
	Kind    string `json:"kind"`
	Company string `json:"company,omitempty"`
}
type Request struct {
	Scope     Scope  `json:"scope"`
	ID        string `json:"id,omitempty"`
	Query     string `json:"query,omitempty"`
	Content   string `json:"content,omitempty"`
	Revision  string `json:"revision,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Cursor    string `json:"cursor,omitempty"`
	Direction string `json:"direction,omitempty"`
}
type Note struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Kind      string   `json:"kind"`
	Revision  string   `json:"revision"`
	UpdatedAt string   `json:"updatedAt"`
	Excerpt   string   `json:"excerpt"`
	Content   string   `json:"content,omitempty"`
	Aliases   []string `json:"aliases"`
	Links     []string `json:"links"`
	Sources   []string `json:"sources"`
	topic     research.Topic
}
type ListResult struct {
	Notes      []Note `json:"notes"`
	Total      int    `json:"total"`
	NextOffset *int   `json:"nextOffset"`
}
type Node struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Kind    string `json:"kind"`
	Missing bool   `json:"missing"`
}
type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}
type Graph struct {
	Nodes      []Node `json:"nodes"`
	Edges      []Edge `json:"edges"`
	TotalNodes int    `json:"totalNodes"`
	Truncated  bool   `json:"truncated"`
}

func Dispatch(operation string, args json.RawMessage) (any, error) {
	var r Request
	if err := json.Unmarshal(args, &r); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if operation == "knowledge.scopes" {
		return listScopes()
	}
	if operation == "knowledge.tick" {
		return runScheduledJobs()
	}
	if operation == "knowledge.context" {
		return researchContext(r)
	}
	if strings.HasPrefix(operation, "history.") {
		return dispatchHistory(operation, r)
	}
	root, err := openScope(r.Scope)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	switch operation {
	case "knowledge.list":
		notes, err := scan(root)
		if err != nil {
			return nil, err
		}
		filtered := []Note{}
		q := strings.ToLower(strings.TrimSpace(r.Query))
		for _, n := range notes {
			if q == "" || strings.Contains(strings.ToLower(n.Title+"\n"+n.Content+"\n"+strings.Join(n.Aliases, " ")), q) {
				n.Content = ""
				filtered = append(filtered, n)
			}
		}
		start, end := pageBounds(len(filtered), r.Offset, r.Limit)
		var next *int
		if end < len(filtered) {
			next = &end
		}
		return ListResult{filtered[start:end], len(filtered), next}, nil
	case "knowledge.resolve":
		return resolveLink(root, r)
	case "knowledge.get":
		return readNote(root, r.ID)
	case "knowledge.save":
		return saveNote(root, r)
	case "knowledge.graph":
		notes, err := scan(root)
		if err != nil {
			return nil, err
		}
		return boundedGraph(scopedGraph(root, notes), 500), nil
	case "knowledge.attachment":
		return readAttachment(root, r.ID, r.Offset)
	case "knowledge.jobs":
		if r.Scope.Kind == "vault" || r.Scope.Kind == "work" {
			return []Job{}, nil
		}
		return listJobs(r.Scope)
	case "knowledge.runJob":
		if r.Scope.Kind == "vault" || r.Scope.Kind == "work" {
			return nil, errors.New("memory jobs require an explicit company or research scope")
		}
		return runJob(root, r)
	default:
		return nil, fmt.Errorf("unknown knowledge operation")
	}
}
func wikiRoot() string {
	if v := os.Getenv("HQ_WIKI_ROOT"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop", "WikiPedik")
}
func dataRoot() string {
	if v := os.Getenv("HQ_DATA_ROOT"); v != "" {
		return v
	}
	return "/srv/hq-data"
}
func openScope(scope Scope) (*os.Root, error) {
	var rel string
	switch scope.Kind {
	case "vault":
		if scope.Company != "" {
			return nil, errors.New("vault scope cannot select a company")
		}
		// Only the authenticated owner transport accepts this scope. Research
		// contexts and runner capabilities are separately prebound and reject it.
		return os.OpenRoot(wikiRoot())
	case "work":
		if scope.Company != "" {
			return nil, errors.New("work scope cannot select a company")
		}
		// Aggregate browsing is owner-only. Company-scoped runners and memory
		// jobs retain their narrower roots; research is a separate vault zone.
		rel = "dev"
	case "research":
		if scope.Company != "" {
			return nil, errors.New("research scope cannot select a company")
		}
		rel = "research"
	case "company":
		if err := world.ValidateSegment(scope.Company); err != nil && scope.Company != "_platform" {
			return nil, errors.New("invalid company scope")
		}
		rel = "dev/20-projects/" + scope.Company
	default:
		return nil, errors.New("select a work, research, vault or company scope")
	}
	canonical, err := world.SafePath(wikiRoot())
	if err != nil {
		return nil, errors.New("Wikipedia vault is unavailable")
	}
	vault, err := os.OpenRoot(canonical)
	if err != nil {
		return nil, errors.New("Wikipedia vault is unavailable")
	}
	defer vault.Close()
	if err := rejectSymlinkComponents(vault, rel); err != nil {
		return nil, err
	}
	root, err := vault.OpenRoot(rel)
	if err != nil {
		return nil, errors.New("Wikipedia scope is unavailable")
	}
	return root, nil
}

// ResearchDirectory exposes the same existing, canonical research root used by
// knowledge.context. Catalog consumers must not guess a research path from names.
func ResearchDirectory() (string, error) {
	root, err := openScope(Scope{Kind: "research"})
	if err != nil {
		return "", err
	}
	defer root.Close()
	return root.Name(), nil
}

func safeID(id string) error {
	if !fs.ValidPath(id) || id == "." || strings.Contains(id, "\\") {
		return errors.New("invalid scoped path")
	}
	for _, part := range strings.Split(id, "/") {
		if strings.HasPrefix(part, ".") {
			return errors.New("hidden files are not exposed")
		}
	}
	return nil
}
func readRegular(root *os.Root, id string) ([]byte, error) {
	if err := rejectSymlinkComponents(root, id); err != nil {
		return nil, err
	}
	f, err := root.Open(id)
	if err != nil {
		return nil, errors.New("file is unavailable in this scope")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("not a regular scoped file")
	}
	return root.ReadFile(id)
}
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func readNote(root *os.Root, id string) (Note, error) {
	if !strings.HasSuffix(id, ".md") {
		return Note{}, errors.New("only Markdown notes are editable")
	}
	data, err := readRegular(root, id)
	if err != nil {
		return Note{}, err
	}
	t, err := research.ParseTopicContent(id, path.Base(path.Dir(id)), string(data))
	if err != nil {
		return Note{}, errors.New("invalid note frontmatter")
	}
	info, err := root.Stat(id)
	if err != nil {
		return Note{}, err
	}
	kind := "note"
	switch {
	case strings.Contains("/"+id, "/topics/"):
		kind = "topic"
	case strings.Contains("/"+id, "/sources/"):
		kind = "source"
	case strings.Contains("/"+id, "/maps/"):
		kind = "map"
	case path.Base(id) == "_inbox.md":
		kind = "inbox"
	}
	excerpt := t.Summary
	if excerpt == "" {
		excerpt = strings.TrimSpace(t.Body)
	}
	runes := []rune(excerpt)
	if len(runes) > 240 {
		excerpt = string(runes[:240]) + "…"
	}
	return Note{ID: id, Title: t.Title, Kind: kind, Revision: digest(data), UpdatedAt: info.ModTime().UTC().Format(time.RFC3339), Excerpt: excerpt, Content: string(data), Aliases: nonNil(t.Aliases), Links: nonNil(t.Links), Sources: nonNil(t.Sources), topic: t}, nil
}
func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
func scan(root *os.Root) ([]Note, error) {
	notes := []Note{}
	err := fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		n, e := readNote(root, p)
		if e != nil {
			return fmt.Errorf("cannot read note %s: %w", p, e)
		}
		notes = append(notes, n)
		return nil
	})
	return notes, err
}
func pageBounds(total, offset, limit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return offset, end
}
func saveNote(root *os.Root, r Request) (Note, error) {
	if r.Revision == "" {
		return Note{}, errors.New("a revision is required")
	}
	if err := safeID(r.ID); err != nil {
		return Note{}, err
	}
	lock := r.ID + ".hq-lock"
	f, err := root.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Note{}, errors.New("note is already being edited; retry")
	}
	f.Close()
	defer root.Remove(lock)
	old, err := readNote(root, r.ID)
	if err != nil {
		return Note{}, err
	}
	if old.Revision != r.Revision {
		return Note{}, errors.New("HQ_KNOWLEDGE_REVISION_CONFLICT")
	}
	if _, err := research.ParseTopicContent(r.ID, "", r.Content); err != nil {
		return Note{}, errors.New("invalid note frontmatter")
	}
	temp := r.ID + ".hq-edit"
	if err := root.WriteFile(temp, []byte(r.Content), 0600); err != nil {
		return Note{}, err
	}
	defer root.Remove(temp)
	if err := root.Rename(temp, r.ID); err != nil {
		return Note{}, err
	}
	return readNote(root, r.ID)
}
func buildGraph(notes []Note) Graph {
	g := Graph{Nodes: []Node{}, Edges: []Edge{}}
	byName := map[string][]string{}
	names := map[string]bool{}
	for _, n := range notes {
		g.Nodes = append(g.Nodes, Node{ID: n.ID, Title: n.Title, Kind: n.Kind})
		names[n.ID] = true
		keys := append([]string{n.ID, strings.TrimSuffix(n.ID, ".md"), strings.TrimSuffix(path.Base(n.ID), ".md"), n.Title, strings.TrimPrefix(strings.TrimSuffix(n.ID, ".md"), "topics/")}, n.Aliases...)
		seen := map[string]bool{}
		for _, key := range keys {
			key = strings.ToLower(key)
			if !seen[key] {
				byName[key] = append(byName[key], n.ID)
				seen[key] = true
			}
		}
	}
	missing := map[string]bool{}
	for _, n := range notes {
		for _, raw := range n.Links {
			target := research.LinkTarget(raw)
			local := path.Clean(path.Join(path.Dir(n.ID), target))
			id := ""
			for _, candidate := range []string{local, local + ".md", target, target + ".md"} {
				if names[candidate] {
					id = candidate
					break
				}
			}
			if id == "" {
				matches := byName[strings.ToLower(target)]
				if len(matches) == 1 {
					id = matches[0]
				}
			}
			if id == "" {
				id = "missing:" + target
				if !missing[id] {
					g.Nodes = append(g.Nodes, Node{ID: id, Title: target, Kind: "missing", Missing: true})
					missing[id] = true
				}
			}
			g.Edges = append(g.Edges, Edge{Source: n.ID, Target: id})
		}
	}
	return g
}

type Attachment struct {
	Name       string `json:"name"`
	MIME       string `json:"mime"`
	Data       string `json:"data"`
	Total      int64  `json:"total"`
	NextOffset *int64 `json:"nextOffset"`
}

// Attachment chunks share the owner RPC transport budget with JSON/base64 overhead.
const attachmentChunkBytes = 256 * 1024

func readAttachment(root *os.Root, id string, offset int) (Attachment, error) {
	if err := rejectSymlinkComponents(root, id); err != nil {
		return Attachment{}, err
	}
	if offset < 0 {
		return Attachment{}, errors.New("invalid attachment offset")
	}
	f, err := root.Open(id)
	if err != nil {
		return Attachment{}, errors.New("attachment unavailable in this scope")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Attachment{}, errors.New("invalid attachment")
	}
	if int64(offset) > info.Size() {
		return Attachment{}, errors.New("invalid attachment offset")
	}
	if _, err = f.Seek(int64(offset), io.SeekStart); err != nil {
		return Attachment{}, err
	}
	data, err := io.ReadAll(io.LimitReader(f, attachmentChunkBytes))
	if err != nil {
		return Attachment{}, err
	}
	typ := mime.TypeByExtension(filepath.Ext(id))
	if typ == "" {
		typ = "application/octet-stream"
	}
	var next *int64
	position := int64(offset) + int64(len(data))
	if position < info.Size() {
		next = &position
	}
	return Attachment{path.Base(id), typ, base64.StdEncoding.EncodeToString(data), info.Size(), next}, nil
}
func resolveLink(root *os.Root, r Request) (any, error) {
	if _, err := readNote(root, r.ID); err != nil {
		return nil, err
	}
	target := research.LinkTarget(r.Query)
	if strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "http://") {
		return map[string]string{"kind": "external", "id": target}, nil
	}
	notes, err := scan(root)
	if err != nil {
		return nil, err
	}
	g := buildGraph(append(notes, Note{ID: "resolve:request", Links: []string{target}}))
	for _, e := range g.Edges {
		if e.Source == "resolve:request" && !strings.HasPrefix(e.Target, "missing:") {
			return map[string]string{"kind": "note", "id": e.Target}, nil
		}
	}
	for _, id := range []string{path.Clean(path.Join(path.Dir(r.ID), target)), target} {
		if safeID(id) != nil {
			continue
		}
		f, err := root.Open(id)
		if err != nil {
			continue
		}
		info, err := f.Stat()
		f.Close()
		if err == nil && info.Mode().IsRegular() {
			kind := "attachment"
			if strings.HasSuffix(id, ".md") {
				kind = "note"
			}
			return map[string]string{"kind": kind, "id": id}, nil
		}
	}
	return nil, errors.New("link target is unavailable in this scope")
}
func sortedNotes(notes []Note) {
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
}

// listScopes is an owner-only chooser; runner access is prebound outside this API.
func listScopes() ([]Scope, error) {
	scopes := []Scope{{Kind: "vault"}, {Kind: "research"}, {Kind: "work"}}
	root, err := os.OpenRoot(wikiRoot())
	if err != nil {
		return nil, errors.New("Wikipedia vault is unavailable")
	}
	defer root.Close()
	dir, err := root.Open("dev/20-projects")
	if os.IsNotExist(err) {
		return scopes, nil
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() && (world.ValidateSegment(entry.Name()) == nil || entry.Name() == "_platform") {
			scopes = append(scopes, Scope{Kind: "company", Company: entry.Name()})
		}
	}
	sort.Slice(scopes[3:], func(i, j int) bool { return scopes[i+3].Company < scopes[j+3].Company })
	return scopes, nil
}
func boundedGraph(g Graph, limit int) Graph {
	g.TotalNodes = len(g.Nodes)
	if len(g.Nodes) <= limit {
		return g
	}
	g.Truncated = true
	g.Nodes = g.Nodes[:limit]
	visible := map[string]bool{}
	for _, node := range g.Nodes {
		visible[node.ID] = true
	}
	edges := []Edge{}
	for _, edge := range g.Edges {
		if visible[edge.Source] && visible[edge.Target] {
			edges = append(edges, edge)
		}
	}
	g.Edges = edges
	return g
}
func researchContext(r Request) (any, error) {
	if r.Scope.Kind != "research" || r.Scope.Company != "" {
		return nil, errors.New("research context cannot include company memory")
	}
	root, err := openScope(r.Scope)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	prompt := "Research the selected topic using only the research memory. Current server instructions and permissions apply."
	if r.ID != "" {
		note, err := readNote(root, r.ID)
		if err != nil {
			return nil, err
		}
		prompt += "\n\nSelected research note: " + note.ID + "\n\n" + note.Content
	}
	return map[string]any{"directory": root.Name(), "prompt": prompt, "preset": "research"}, nil
}

func scopedGraph(root *os.Root, notes []Note) Graph {
	g := buildGraph(notes)
	known := map[string]bool{}
	for _, n := range g.Nodes {
		known[n.ID] = true
	}
	resolved := map[string]bool{}
	for i, e := range g.Edges {
		if !strings.HasPrefix(e.Target, "missing:") {
			continue
		}
		target := strings.TrimPrefix(e.Target, "missing:")
		for _, id := range []string{path.Clean(path.Join(path.Dir(e.Source), target)), target} {
			if safeID(id) != nil {
				continue
			}
			f, err := root.Open(id)
			if err != nil {
				continue
			}
			info, err := f.Stat()
			f.Close()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			g.Edges[i].Target = id
			resolved[e.Target] = true
			if !known[id] {
				g.Nodes = append(g.Nodes, Node{ID: id, Title: path.Base(id), Kind: "attachment"})
				known[id] = true
			}
			break
		}
	}
	used := map[string]bool{}
	for _, e := range g.Edges {
		used[e.Target] = true
	}
	nodes := g.Nodes[:0]
	for _, n := range g.Nodes {
		if n.Missing && resolved[n.ID] && !used[n.ID] {
			continue
		}
		nodes = append(nodes, n)
	}
	g.Nodes = nodes
	return g
}

func rejectSymlinkComponents(root *os.Root, id string) error {
	if err := safeID(id); err != nil {
		return err
	}
	parts := strings.Split(id, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return errors.New("scoped path is unavailable")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("scope aliases cannot cross memory boundaries")
		}
	}
	return nil
}
