// Package entity exposes the file-backed HQ context cards to any client.
// AGENTS.md remains canonical; confirmations reference section hashes rather
// than treating generated prose or a missing TODO as a human decision.
package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/world"
)

type Document struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Revision string `json:"revision"`
}
type Settings struct {
	Namespace string `json:"namespace"`
	VCS       string `json:"vcs,omitempty"`
	Host      string `json:"host,omitempty"`
	GitEmail  string `json:"gitEmail,omitempty"`
}
type Parent struct {
	Scope        string `json:"scope"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	Path         string `json:"path"`
	DocumentPath string `json:"documentPath"`
}
type Item struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Automatic   bool   `json:"automatic"`
}
type Onboarding struct {
	Status    string `json:"status"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Items     []Item `json:"items"`
}
type Check struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
	Path     string `json:"path,omitempty"`
	Blocking bool   `json:"blocking"`
}
type Health struct {
	CheckedAt string  `json:"checkedAt"`
	Status    string  `json:"status"`
	Checks    []Check `json:"checks"`
}
type Child struct {
	Scope            string `json:"scope"`
	Kind             string `json:"kind"`
	Title            string `json:"title"`
	OnboardingStatus string `json:"onboardingStatus"`
	HealthStatus     string `json:"healthStatus"`
	ErrorCount       int    `json:"errorCount"`
	WarningCount     int    `json:"warningCount"`
}
type Event struct {
	At     string `json:"at"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Detail string `json:"detail"`
}
type Card struct {
	Version      int           `json:"version"`
	Scope        string        `json:"scope"`
	Kind         string        `json:"kind"`
	Title        string        `json:"title"`
	Path         string        `json:"path"`
	Document     Document      `json:"document"`
	Settings     Settings      `json:"settings"`
	Parents      []Parent      `json:"parents"`
	Onboarding   Onboarding    `json:"onboarding"`
	Health       Health        `json:"health"`
	Children     []Child       `json:"children"`
	History      []Event       `json:"history"`
	AgentActions []AgentAction `json:"agentActions,omitempty"`
}
type state struct {
	Version   int               `json:"version"`
	Confirmed map[string]string `json:"confirmed"`
	History   []Event           `json:"history"`
}
type snapshot struct {
	path, scope, kind, marker   string
	parts                       []string
	content, config, stateBytes []byte
	docExists                   bool
	state                       state
	settings                    Settings
}

func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func (s snapshot) revision() string {
	b, _ := json.Marshal([]any{s.scope, s.docExists, string(s.content), string(s.config), string(s.stateBytes)})
	return digest(string(b))
}

func readOptional(p string) ([]byte, bool, error) {
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return b, err == nil, err
}

func loadSnapshot(root, scope string) (snapshot, error) {
	s := snapshot{scope: scope, parts: strings.Split(scope, "/"), state: state{Version: 1, Confirmed: map[string]string{}, History: []Event{}}}
	if len(s.parts) < 1 || len(s.parts) > 3 {
		return s, fmt.Errorf("scope must be company[/product[/repo]]")
	}
	for _, p := range s.parts {
		if err := world.ValidateSegment(p); err != nil {
			return s, err
		}
	}
	kinds := []string{"company", "product", "repo"}
	markers := []string{".company-config", ".product-config", ".git"}
	s.kind = kinds[len(s.parts)-1]
	s.marker = markers[len(s.parts)-1]
	path, err := world.SafePath(root, s.parts...)
	if err != nil {
		return s, err
	}
	s.path = path
	for i := range s.parts {
		p, err := world.SafePath(root, append(append([]string{}, s.parts[:i+1]...), markers[i])...)
		if err != nil {
			return s, err
		}
		if _, err = os.Stat(p); err != nil {
			return s, fmt.Errorf("entity %s is not registered: %w", strings.Join(s.parts[:i+1], "/"), err)
		}
		if i < 2 {
			b, err := os.ReadFile(p)
			if err != nil {
				return s, err
			}
			if i == len(s.parts)-1 {
				s.config = b
			}
			config, err := world.ReadConfig(p)
			if err != nil {
				return s, err
			}
			for _, key := range config.Keys() {
				switch key {
				case "namespace":
					s.settings.Namespace = config.Get(key)
				case "vcs":
					s.settings.VCS = config.Get(key)
				case "host":
					s.settings.Host = config.Get(key)
				case "git_email":
					s.settings.GitEmail = config.Get(key)
				}
			}
		}
	}
	doc, err := world.SafePath(root, append(append([]string{}, s.parts...), "AGENTS.md")...)
	if err != nil {
		return s, err
	}
	s.content, s.docExists, err = readOptional(doc)
	if err != nil {
		return s, err
	}
	sp, err := world.SafePath(root, append(append([]string{}, s.parts...), ".hq", "entity.json")...)
	if err != nil {
		return s, err
	}
	s.stateBytes, _, err = readOptional(sp)
	if err != nil {
		return s, err
	}
	if len(s.stateBytes) > 0 {
		if err = json.Unmarshal(s.stateBytes, &s.state); err != nil {
			return s, fmt.Errorf("read onboarding state: %w", err)
		}
		if s.state.Version != 1 {
			return s, fmt.Errorf("unsupported entity state version %d", s.state.Version)
		}
	}
	if s.state.Confirmed == nil {
		s.state.Confirmed = map[string]string{}
	}
	if s.state.History == nil {
		s.state.History = []Event{}
	}
	return s, nil
}

// Show runs only local, read-only checks. Missing context never blocks a session.
func Show(root, scope string) (Card, error) { return show(root, scope, true) }
func show(root, scope string, children bool) (Card, error) {
	s, err := loadSnapshot(root, scope)
	if err != nil {
		return Card{}, err
	}
	c := Card{Version: 1, Scope: scope, Kind: s.kind, Title: s.parts[len(s.parts)-1], Path: s.path, Document: Document{Path: filepath.Join(s.path, "AGENTS.md"), Content: string(s.content), Revision: s.revision()}, Settings: s.settings, Parents: []Parent{}, Children: []Child{}, History: s.state.History}
	for i := 0; i < len(s.parts)-1; i++ {
		p, err := world.SafePath(root, s.parts[:i+1]...)
		if err != nil {
			return Card{}, err
		}
		kind := []string{"company", "product"}[i]
		c.Parents = append(c.Parents, Parent{Scope: strings.Join(s.parts[:i+1], "/"), Kind: kind, Title: s.parts[i], Path: p, DocumentPath: filepath.Join(p, "AGENTS.md")})
	}
	c.Onboarding = progress(s)
	c.AgentActions = agentActions()
	c.Health = health(root, s, c.Onboarding, c.Parents)
	if children && s.kind != "repo" {
		entries, err := os.ReadDir(s.path)
		if err != nil {
			return Card{}, err
		}
		marker := ".product-config"
		kind := "product"
		if s.kind == "product" {
			marker = ".git"
			kind = "repo"
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if _, err := os.Lstat(filepath.Join(s.path, e.Name(), marker)); os.IsNotExist(err) {
				continue
			}
			cs := scope + "/" + e.Name()
			child, err := show(root, cs, true)
			row := Child{Scope: cs, Kind: kind, Title: e.Name()}
			if err != nil {
				row.OnboardingStatus = "not_started"
				row.HealthStatus = "error"
				row.ErrorCount = 1
			} else {
				row.OnboardingStatus = child.Onboarding.Status
				row.HealthStatus = child.Health.Status
				for _, check := range child.Health.Checks {
					if check.Status == "error" {
						row.ErrorCount++
					}
					if check.Status == "warning" {
						row.WarningCount++
					}
				}
				for _, desc := range child.Children {
					row.ErrorCount += desc.ErrorCount
					row.WarningCount += desc.WarningCount
				}
				if row.ErrorCount > 0 {
					row.HealthStatus = "error"
				} else if row.WarningCount > 0 {
					row.HealthStatus = "warning"
				}
			}
			c.Children = append(c.Children, row)
		}
	}
	return c, nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
