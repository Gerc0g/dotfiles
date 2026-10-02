package entity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Gerc0g/dotfiles/core/world"
)

const RevisionConflict = "HQ_ENTITY_REVISION_CONFLICT"

type Confirmation struct {
	ID        string `json:"id"`
	Confirmed bool   `json:"confirmed"`
}
type SettingsUpdate struct {
	Namespace string `json:"namespace"`
}
type UpdateRequest struct {
	Revision string          `json:"revision"`
	Content  *string         `json:"content,omitempty"`
	Confirm  *Confirmation   `json:"confirm,omitempty"`
	Settings *SettingsUpdate `json:"settings,omitempty"`
}

// Update serializes HQ writers and checks the client's snapshot before saving.
// Each file is replaced atomically. External editors do not participate in the
// lock. A final revision check detects edits made before replacement, but cannot
// offer atomic compare-and-swap against non-cooperating external file writers.
func Update(root, scope string, request UpdateRequest) (Card, error) {
	count := 0
	for _, present := range []bool{request.Content != nil, request.Confirm != nil, request.Settings != nil} {
		if present {
			count++
		}
	}
	if count != 1 || request.Revision == "" {
		return Card{}, fmt.Errorf("exactly one edit and a revision are required")
	}
	initial, err := loadSnapshot(root, scope)
	if err != nil {
		return Card{}, err
	}
	if initial.revision() != request.Revision {
		return Card{}, fmt.Errorf("%s: reload the card; your draft was not saved", RevisionConflict)
	}
	stateDir, err := world.SafePath(root, append(append([]string{}, initial.parts...), ".hq")...)
	if err != nil {
		return Card{}, err
	}
	if err = os.MkdirAll(stateDir, 0755); err != nil {
		return Card{}, err
	}
	// The lock is runtime state, while entity.json can be versioned with context.
	// Preserve an existing ignore policy rather than rewriting user-owned rules.
	ignorePath, err := world.SafePath(stateDir, ".gitignore")
	if err != nil {
		return Card{}, err
	}
	ignore, err := os.OpenFile(ignorePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err == nil {
		_, writeErr := ignore.WriteString("entity.lock\n.hq-write-*\n")
		closeErr := ignore.Close()
		if writeErr != nil {
			return Card{}, writeErr
		}
		if closeErr != nil {
			return Card{}, closeErr
		}
	} else if !os.IsExist(err) {
		return Card{}, err
	}
	lockPath, err := world.SafePath(stateDir, "entity.lock")
	if err != nil {
		return Card{}, err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return Card{}, err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return Card{}, err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	s, err := loadSnapshot(root, scope)
	if err != nil {
		return Card{}, err
	}
	if s.revision() != request.Revision {
		return Card{}, fmt.Errorf("%s: reload the card; your draft was not saved", RevisionConflict)
	}
	var target string
	var replacement []byte
	event := Event{At: now(), Actor: "HQ"}
	switch {
	case request.Content != nil:
		target = filepath.Join(s.path, "AGENTS.md")
		replacement = []byte(*request.Content)
		event.Action = "context.updated"
		event.Detail = "Контекст обновлён. Изменённые разделы ожидают подтверждения."
	case request.Confirm != nil:
		found := false
		for _, d := range definitions(s.kind) {
			if d.id != request.Confirm.ID {
				continue
			}
			found = true
			if d.automatic {
				return Card{}, fmt.Errorf("automatic checks cannot be confirmed manually")
			}
			value := itemContent(s, d)
			if request.Confirm.Confirmed {
				if value == "" || hasPlaceholder(value) {
					return Card{}, fmt.Errorf("fill section %s before confirming it", d.id)
				}
				s.state.Confirmed[d.id] = digest(value)
				event.Action = "onboarding.confirmed"
				event.Detail = "Подтверждён раздел: " + d.title
			} else {
				delete(s.state.Confirmed, d.id)
				event.Action = "onboarding.reopened"
				event.Detail = "Снято подтверждение раздела: " + d.title
			}
		}
		if !found {
			return Card{}, fmt.Errorf("unknown onboarding item %q", request.Confirm.ID)
		}
	case request.Settings != nil:
		if s.kind == "repo" {
			return Card{}, fmt.Errorf("repository settings are read-only")
		}
		ns := request.Settings.Namespace
		for _, part := range strings.Split(ns, "/") {
			if err := world.ValidateSegment(part); err != nil {
				return Card{}, fmt.Errorf("invalid namespace: %w", err)
			}
		}
		lines := strings.Split(string(s.config), "\n")
		found := false
		for i, line := range lines {
			key, _, ok := strings.Cut(line, ":")
			if ok && strings.TrimSpace(key) == "namespace" {
				lines[i] = "namespace: " + ns
				found = true
			}
		}
		if !found {
			lines = append(lines, "namespace: "+ns)
		}
		replacement = []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n")
		target = filepath.Join(s.path, s.marker)
		event.Action = "settings.updated"
		event.Detail = "Пространство имён Git изменено. Адреса существующих репозиториев и текст контекста сохранены."
	}
	s.state.History = append(s.state.History, event)
	body, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return Card{}, err
	}
	body = append(body, '\n')
	// Validate after staging/fsync, immediately before replacement. HQ writers
	// remain serialized; an external editor can still race the final rename.
	guard := func() error {
		latest, err := loadSnapshot(root, scope)
		if err != nil {
			return err
		}
		if latest.revision() != request.Revision {
			return fmt.Errorf("%s: context changed while saving", RevisionConflict)
		}
		return nil
	}
	if target != "" {
		if err = atomicWrite(target, replacement, 0644, guard); err != nil {
			return Card{}, err
		}
	}
	stateGuard := guard
	if target != "" {
		stateGuard = nil
	}
	if err = atomicWrite(filepath.Join(stateDir, "entity.json"), body, 0600, stateGuard); err != nil {
		if target != "" {
			return Card{}, fmt.Errorf("content saved, but onboarding history failed; reload before retrying: %w", err)
		}
		return Card{}, err
	}
	return Show(root, scope)
}

func atomicWrite(path string, body []byte, mode os.FileMode, beforeReplace func() error) error {
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular file %s", path)
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".hq-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(body); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if beforeReplace != nil {
		if err = beforeReplace(); err != nil {
			return err
		}
	}
	return os.Rename(file.Name(), path)
}
