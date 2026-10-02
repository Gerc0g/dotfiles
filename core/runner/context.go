package runner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/entity"
	"github.com/Gerc0g/dotfiles/core/world"
)

func decodeBound(raw json.RawMessage, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("invalid bound request")
	}
	return nil
}

func contextOperation(b Binding, operation string, raw json.RawMessage) (any, error) {
	if b.Preset != "work" || b.EntityRef == "" {
		return nil, fmt.Errorf("no company context in this task")
	}
	if operation == "context.get" {
		var empty struct{}
		if err := decodeBound(raw, &empty); err != nil {
			return nil, err
		}
		return b, nil
	}
	var req struct {
		Scope   string                `json:"scope"`
		Request *entity.UpdateRequest `json:"request,omitempty"`
	}
	if err := decodeBound(raw, &req); err != nil {
		return nil, err
	}
	parts := strings.Split(req.Scope, "/")
	if len(parts) > 3 || parts[0] != b.CompanyID {
		return nil, fmt.Errorf("entity scope is outside this task")
	}
	for _, part := range parts {
		if err := world.ValidateSegment(part); err != nil {
			return nil, err
		}
	}
	related := req.Scope == b.EntityRef || strings.HasPrefix(b.EntityRef, req.Scope+"/") || strings.HasPrefix(req.Scope, b.EntityRef+"/")
	if !related {
		return nil, fmt.Errorf("entity scope is outside this task")
	}
	root, err := world.Root()
	if err != nil {
		return nil, err
	}
	switch operation {
	case "entity.show":
		if req.Request != nil {
			return nil, fmt.Errorf("show does not accept an edit")
		}
		return entity.Show(root, req.Scope)
	case "entity.update":
		if req.Scope != b.EntityRef || req.Request == nil || req.Request.Content == nil || req.Request.Confirm != nil || req.Request.Settings != nil {
			return nil, fmt.Errorf("task may update only its bound entity document; human confirmation and policy settings require the owner UI")
		}
		return entity.Update(root, req.Scope, *req.Request)
	default:
		return nil, fmt.Errorf("unknown context operation")
	}
}

// SandboxCLI preserves the public skill command shape, while every operation
// goes through the task's bound socket. The ordinary host CLI is unaffected.
func SandboxCLI(ctx context.Context, args []string, out io.Writer) (bool, error) {
	if os.Getenv("HQ_RUNNER_SANDBOX") != "1" {
		return false, nil
	}
	if len(args) >= 2 && args[0] == "runner" && (args[1] == "inside" || args[1] == "broker") {
		return false, nil
	}
	if len(args) == 1 && args[0] == "ctx" || len(args) == 2 && args[0] == "ctx" && args[1] == "--plain" {
		raw, err := Broker(ctx, "context.get", json.RawMessage(`{}`))
		if err != nil {
			return true, err
		}
		var b Binding
		if err = json.Unmarshal(raw, &b); err != nil {
			return true, err
		}
		parts := strings.Split(b.EntityRef, "/")
		fmt.Fprintf(out, "kind=%s\ncompany=%s\npath=%s\n", b.Kind, b.CompanyID, b.WorktreePath)
		if len(parts) > 1 {
			fmt.Fprintf(out, "product=%s\n", parts[1])
		}
		if len(parts) > 2 {
			fmt.Fprintf(out, "repo=%s\n", parts[2])
		}
		return true, nil
	}
	if len(args) >= 3 && args[0] == "entity" {
		req := map[string]any{"scope": args[2]}
		op := ""
		switch {
		case args[1] == "show" && (len(args) == 3 || len(args) == 4 && args[3] == "--json"):
			op = "entity.show"
		case args[1] == "update" && len(args) == 5 && args[3] == "--json-base64":
			payload, err := base64.StdEncoding.DecodeString(args[4])
			if err != nil || !json.Valid(payload) {
				return true, fmt.Errorf("invalid entity update payload")
			}
			req["request"] = json.RawMessage(payload)
			op = "entity.update"
		default:
			return true, fmt.Errorf("sandbox supports entity show SCOPE --json and entity update SCOPE --json-base64 PAYLOAD")
		}
		raw, _ := json.Marshal(req)
		result, err := Broker(ctx, op, raw)
		if err == nil {
			_, err = fmt.Fprintln(out, string(result))
		}
		return true, err
	}
	return true, fmt.Errorf("HQ owner commands are unavailable in a task; use ctx, entity show/update or runner broker")
}

func contextTask(b Binding) bool { return b.Kind == "company" || b.Kind == "product" }

// Context tasks get a private writable draft directory. Only product tasks add
// individual registered child checkouts, each mounted read-only by Run.
func prepareContextWorkspace(b Binding, home string) (string, []string, error) {
	root, err := world.Root()
	if err != nil {
		return "", nil, err
	}
	workspace, err := world.SafePath(home, "workspace")
	if err != nil {
		return "", nil, err
	}
	if err = os.MkdirAll(workspace, 0700); err != nil {
		return "", nil, err
	}
	card, err := entity.Show(root, b.EntityRef)
	if err != nil {
		return "", nil, err
	}
	if err = safeWrite(workspace, "AGENTS.md", []byte(card.Document.Content)); err != nil {
		return "", nil, err
	}
	// A single architecture document is useful context, not a whole directory
	// containing arbitrary company files. Existing task drafts are retained.
	if err = copyContextDocument(b.WorktreePath, workspace, "docs/ARCHITECTURE.md"); err != nil {
		return "", nil, err
	}
	var repos []string
	if b.Kind == "product" {
		for _, child := range card.Children {
			if child.Kind != "repo" {
				continue
			}
			path, err := world.SafePath(root, strings.Split(child.Scope, "/")...)
			if err != nil {
				return "", nil, err
			}
			if strings.ContainsAny(path, ":\r\n") {
				return "", nil, fmt.Errorf("unsupported repository mount path")
			}
			if _, err = world.SafePath(workspace, filepath.Base(path)); err != nil {
				return "", nil, err
			}
			repos = append(repos, path)
		}
	}
	return workspace, repos, nil
}

func copyContextDocument(source, dest, name string) error {
	canonical, err := world.SafePath(source, strings.Split(name, "/")...)
	if err != nil {
		return err
	}
	info, err := os.Lstat(canonical)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return fmt.Errorf("context document must be a regular file of at most 1 MiB")
	}
	raw, exists, err := readAnalysisDocument(source, name)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	dst, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err = dst.Stat(name); err == nil {
		return nil
	}
	if err = dst.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return err
	}
	return dst.WriteFile(name, raw, 0600)
}

func appendBoundInstructions(home string, b Binding) error {
	f, err := os.OpenFile(filepath.Join(home, "AGENTS.md"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n\n## HQ task authority\n\nBound entity: %s. Use `hq ctx --plain`, `hq entity show SCOPE --json`, and `hq entity update SCOPE --json-base64 PAYLOAD`. Updates require the current revision and content only. Confirmations and policy settings remain owner UI actions. Ancestor and descendant cards within this bound entity are readable; unrelated scopes are denied. Templates are in /home/codex/dotfiles/templates. Company memory is mounted read-only at %s.\n", b.EntityRef, filepath.Join(wikiRoot(), "dev/20-projects", b.CompanyID))
	if err == nil && contextTask(b) {
		_, err = f.WriteString("\nThis company/product directory is a durable private draft workspace. Its AGENTS.md is a snapshot; save canonical context through the entity broker. Product child repositories are individually read-only, without Git metadata or credentials. Other files stay in this task workspace. To publish an authorized product architecture document, use `hq runner broker document.get '{\"name\":\"docs/ARCHITECTURE.md\"}'` then document.save with name, returned revision and complete content. Repository tasks may publish design.md or docs/design.md by the same protocol. Do not claim canonical publication of a scratch draft. Read-only source bytes do not count toward the task's writable disk budget.\n")
	}
	return err
}

func copyTemplates(home string) error {
	source := os.Getenv("HQ_TEMPLATES_ROOT")
	if source == "" {
		skills := os.Getenv("HQ_SKILLS_ROOT")
		if skills == "" {
			return nil
		}
		source = filepath.Join(filepath.Dir(skills), "templates")
	}
	for _, kind := range []string{"company", "product", "repo"} {
		name := "AGENTS.md." + kind + ".tmpl"
		raw, err := os.ReadFile(filepath.Join(source, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if len(raw) > 1<<20 {
			return fmt.Errorf("template exceeds 1 MiB")
		}
		dest, err := world.SafePath(home, "dotfiles", "templates")
		if err != nil {
			return err
		}
		if err = os.MkdirAll(dest, 0700); err != nil {
			return err
		}
		if err = safeWrite(dest, name, raw); err != nil {
			return err
		}
	}
	return nil
}
