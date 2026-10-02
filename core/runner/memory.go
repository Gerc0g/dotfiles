package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/knowledge"
	"github.com/Gerc0g/dotfiles/core/world"
	"golang.org/x/sys/unix"
)

func captureMemory(b Binding, args json.RawMessage) (any, error) {
	var input struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("invalid memory capture JSON")
	}
	if len(input.Title) == 0 || len(input.Title) > 160 || strings.ContainsAny(input.Title, "\r\n") || len(input.Body) == 0 || len(input.Body) > 16000 {
		return nil, fmt.Errorf("memory capture requires a short title and body (max 16 KiB)")
	}
	parts := strings.Split(b.RepoID, "/")
	if b.Preset != "work" || len(parts) != 3 || parts[0] != b.CompanyID {
		return nil, fmt.Errorf("company memory is unavailable in research")
	}
	safe, err := world.SafePath(wikiRoot(), "dev", "20-projects", parts[0], parts[1], "repos", parts[2])
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(safe)
	if err != nil {
		return nil, fmt.Errorf("scoped project memory unavailable: %w", err)
	}
	defer root.Close()
	file, err := root.OpenFile("_inbox.md", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err = unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return nil, err
	}
	defer unix.Flock(int(file.Fd()), unix.LOCK_UN)
	entry := fmt.Sprintf("\n## [%s] capture | %s\n\nScope: %s\nStatus: candidate\nSource: HQ isolated task\n\n%s\n", time.Now().UTC().Format("2006-01-02 15:04"), input.Title, b.RepoID, input.Body)
	if _, err = file.WriteString(entry); err != nil {
		return nil, err
	}
	return map[string]string{"status": "candidate", "repoId": b.RepoID}, nil
}
func reviewMemory(b Binding) error {
	raw, _ := json.Marshal(map[string]any{"scope": map[string]string{"kind": "company", "company": b.CompanyID}, "kind": "review"})
	_, err := knowledge.Dispatch("knowledge.runJob", raw)
	return err
}
func appendMemoryInstructions(home string) error {
	file, err := os.OpenFile(filepath.Join(home, "AGENTS.md"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString("\n\n## Memory capture\n\nProject memory is read-only. To preserve a durable lesson, use `hq runner broker memory.capture '{\"title\":\"Short title\",\"body\":\"Problem, evidence, root cause, reusable rule and citations\"}'`. This appends only to this repository's inbox. Entries remain candidates for human review. Do not include secrets. Git and environment operations use `hq runner broker` and the bound company policy; no company credentials are available in this task.\n")
	return err
}
