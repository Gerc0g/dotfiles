package knowledge

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/research"
	"github.com/Gerc0g/dotfiles/core/wiki"
	_ "modernc.org/sqlite"
)

type Job struct {
	ID         string `json:"id"`
	Scope      Scope  `json:"scope"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Error      string `json:"error,omitempty"`
	Result     any    `json:"result,omitempty"`
}

func scopeKey(s Scope) string { return s.Kind + ":" + s.Company }
func openJournal() (*sql.DB, error) {
	if err := os.MkdirAll(dataRoot(), 0700); err != nil {
		return nil, err
	}
	journal := filepath.Join(dataRoot(), "knowledge.sqlite")
	f, err := os.OpenFile(journal, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	err = f.Chmod(0600)
	f.Close()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", journal)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS jobs (id TEXT PRIMARY KEY, scope TEXT NOT NULL, kind TEXT NOT NULL, record TEXT NOT NULL); CREATE INDEX IF NOT EXISTS jobs_scope ON jobs(scope);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func listJobs(s Scope) ([]Job, error) {
	db, err := openJournal()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query("SELECT record FROM jobs WHERE scope=? ORDER BY rowid DESC LIMIT 100", scopeKey(s))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []Job{}
	for rows.Next() {
		var text string
		var j Job
		if err := rows.Scan(&text); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(text), &j); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range jobs {
		if jobs[i].Status != "running" {
			continue
		}
		release, err := acquireScopeLock(s)
		if err == nil {
			// The job may have finished between the query and lock acquisition.
			var latest string
			if err = db.QueryRow("SELECT record FROM jobs WHERE id=?", jobs[i].ID).Scan(&latest); err != nil {
				release()
				return nil, err
			}
			if err = json.Unmarshal([]byte(latest), &jobs[i]); err != nil {
				release()
				return nil, err
			}
			if jobs[i].Status != "running" {
				release()
				continue
			}
			jobs[i].Status = "failed"
			jobs[i].Error = "Memory job interrupted before completion"
			jobs[i].FinishedAt = time.Now().UTC().Format(time.RFC3339)
			err = saveJob(db, jobs[i])
			release()
			if err != nil {
				return nil, err
			}
		}
	}
	return jobs, nil
}
func saveJob(db *sql.DB, j Job) error {
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO jobs (id,scope,kind,record) VALUES (?,?,?,?) ON CONFLICT(id) DO UPDATE SET record=excluded.record", j.ID, scopeKey(j.Scope), j.Kind, string(b))
	return err
}
func runJob(root *os.Root, r Request) (Job, error) {
	switch r.Kind {
	case "lint", "index", "review", "hot", "synthesize":
	default:
		return Job{}, errors.New("unsupported memory job")
	}
	db, err := openJournal()
	if err != nil {
		return Job{}, err
	}
	defer db.Close()
	release, err := acquireScopeLock(r.Scope)
	if err != nil {
		return Job{}, err
	}
	defer release()
	// Recover attempts that lost their process while this scope was unlocked.
	rows, err := db.Query("SELECT record FROM jobs WHERE scope=? AND json_extract(record,'$.status')='running'", scopeKey(r.Scope))
	if err != nil {
		return Job{}, err
	}
	var interrupted []Job
	for rows.Next() {
		var data string
		var previous Job
		if e := rows.Scan(&data); e != nil {
			rows.Close()
			return Job{}, e
		}
		if e := json.Unmarshal([]byte(data), &previous); e != nil {
			rows.Close()
			return Job{}, e
		}
		if previous.Status == "running" {
			previous.Status = "failed"
			previous.Error = "Memory job interrupted before completion"
			previous.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			interrupted = append(interrupted, previous)
		}
	}
	rows.Close()
	for _, previous := range interrupted {
		if err := saveJob(db, previous); err != nil {
			return Job{}, err
		}
	}
	b := make([]byte, 16)
	if _, err = rand.Read(b); err != nil {
		return Job{}, err
	}
	j := Job{ID: hex.EncodeToString(b), Scope: r.Scope, Kind: r.Kind, Status: "running", StartedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := saveJob(db, j); err != nil {
		return Job{}, err
	}
	notes, err := scan(root)
	if err == nil {
		switch r.Kind {
		case "review":
			candidates := []string{}
			count := 0
			for _, n := range notes {
				if n.Kind == "inbox" {
					c := wiki.CountStatusText(n.Content, "candidate")
					if c > 0 {
						count += c
						candidates = append(candidates, n.ID)
					}
				}
			}
			j.Result = map[string]any{"candidates": count, "inboxes": candidates}
			if count > 0 {
				j.Status = "needs_review"
			}
		case "synthesize":
			sources := []string{}
			for _, n := range notes {
				base := path.Base(n.ID)
				if base == "_synthesis-candidates.md" || (base == "lessons.md" || base == "gotchas.md" || base == "debugging-stories.md") && len(n.topic.Sections) > 0 {
					sources = append(sources, n.ID)
				}
			}
			j.Result = map[string]any{"files": sources}
			if len(sources) > 0 {
				j.Status = "needs_review"
			}
		case "lint":
			g := scopedGraph(root, notes)
			issues := []map[string]string{}
			for _, e := range g.Edges {
				if strings.HasPrefix(e.Target, "missing:") {
					issues = append(issues, map[string]string{"note": e.Source, "rule": "broken-link", "target": strings.TrimPrefix(e.Target, "missing:")})
				}
			}
			if r.Scope.Kind == "research" {
				for _, f := range research.Lint(zoneFromNotes(notes)) {
					issues = append(issues, map[string]string{"note": f.Ref, "rule": f.Rule, "message": f.Message})
				}
			}
			j.Result = issues
		case "hot":
			if r.Scope.Kind != "company" {
				err = errors.New("hot context belongs to company memory")
				break
			}
			byDir := map[string]map[string]string{}
			for _, n := range notes {
				parts := strings.Split(n.ID, "/")
				idx := -1
				for i, part := range parts {
					if part == "repos" && i+2 < len(parts) {
						idx = i
						break
					}
				}
				if idx < 0 {
					continue
				}
				dir := strings.Join(parts[:idx+2], "/")
				if byDir[dir] == nil {
					byDir[dir] = map[string]string{}
				}
				byDir[dir][strings.Join(parts[idx+2:], "/")] = n.Content
			}
			for dir, pages := range byDir {
				product := strings.Split(dir, "/repos/")[0]
				for _, n := range notes {
					prefix := product + "/shared/rules/"
					if strings.HasPrefix(n.ID, prefix) {
						pages["product-rules/"+strings.TrimPrefix(n.ID, prefix)] = n.Content
					}
				}
			}
			refreshed := []string{}
			for dir, pages := range byDir {
				content := wiki.BuildHotFromPages(path.Base(dir), pages)
				id := path.Join(dir, "hot.md")
				if e := root.WriteFile(id+".hq-edit", []byte(content), 0600); e != nil {
					err = e
					break
				}
				if e := root.Rename(id+".hq-edit", id); e != nil {
					err = e
					break
				}
				refreshed = append(refreshed, id)
			}
			j.Result = map[string]any{"files": refreshed}
		case "index":
			content := ""
			if r.Scope.Kind == "research" {
				content = research.BuildIndex(zoneFromNotes(notes))
			} else {
				var out strings.Builder
				out.WriteString("# Memory index\n\n")
				for _, n := range notes {
					if n.ID != "index.md" {
						fmt.Fprintf(&out, "- [[%s|%s]]\n", strings.TrimSuffix(n.ID, ".md"), n.Title)
					}
				}
				content = out.String()
			}
			err = root.WriteFile("index.md.hq-edit", []byte(content), 0600)
			if err == nil {
				err = root.Rename("index.md.hq-edit", "index.md")
			}
			j.Result = map[string]any{"notes": len(notes), "file": "index.md"}
		}
	}
	if err != nil {
		j.Status = "failed"
		j.Error = err.Error()
	} else if j.Status == "running" {
		j.Status = "succeeded"
	}
	j.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveJob(db, j); err != nil {
		return Job{}, err
	}
	return j, nil
}
func zoneFromNotes(notes []Note) *research.Zone {
	z := &research.Zone{}
	for _, n := range notes {
		switch n.Kind {
		case "topic":
			z.Topics = append(z.Topics, n.topic)
		case "map":
			z.Maps = append(z.Maps, research.Doc{Path: n.ID, Slug: strings.TrimSuffix(path.Base(n.ID), ".md"), Title: n.Title})
		case "source":
			z.Sources = append(z.Sources, research.Doc{Path: n.ID, Slug: strings.TrimSuffix(path.Base(n.ID), ".md"), Title: n.Title})
		}
	}
	return z
}
