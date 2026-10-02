package runner

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const MaxTasks = 2
const TaskMemory = "2g"
const TaskCPUs = "1.5"
const TaskPIDs = "256"
const TaskDiskBytes int64 = 4 << 30
const MinFreeDiskBytes uint64 = 5 << 30

type Status struct {
	ID         string     `json:"id"`
	Container  string     `json:"container"`
	Binding    Binding    `json:"binding"`
	State      string     `json:"state"`
	Error      string     `json:"error,omitempty"`
	PID        int        `json:"pid"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}
type admission struct {
	status Status
	locks  []*os.File
	path   string
}

func lock(path string) (*os.File, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func acquire(b Binding) (*admission, error) {
	root := filepath.Join(dataRoot(), "runner")
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	company := containerScope(b)
	if company == "" {
		company = "research"
	}
	f, err := lock(filepath.Join(root, "company-"+company+".lock"))
	if err != nil {
		return nil, fmt.Errorf("HQ capacity: this company already has a running task")
	}
	a := &admission{locks: []*os.File{f}}
	var slot *os.File
	slotIndex := 0
	for i := 0; i < MaxTasks; i++ {
		slot, err = lock(filepath.Join(root, fmt.Sprintf("slot-%d.lock", i)))
		if err == nil {
			slotIndex = i
			break
		}
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("HQ capacity: all %d task slots are busy", MaxTasks)
	}
	a.locks = append(a.locks, slot)
	var id [12]byte
	if _, err = rand.Read(id[:]); err != nil {
		a.release("failed", "")
		return nil, err
	}
	a.status = Status{ID: hex.EncodeToString(id[:]), Container: fmt.Sprintf("hq-task-slot-%d", slotIndex), Binding: b, State: "starting", PID: os.Getpid(), StartedAt: time.Now().UTC()}
	a.path = filepath.Join(root, a.status.ID+".json")
	if err := a.save(); err != nil {
		a.release("failed", "")
		return nil, err
	}
	return a, nil
}
func (a *admission) save() error {
	v, e := json.MarshalIndent(a.status, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(a.path, v, 0600)
}
func (a *admission) release(state, message string) {
	if a.path != "" {
		now := time.Now().UTC()
		a.status.State = state
		a.status.Error = message
		a.status.FinishedAt = &now
		_ = a.save()
	}
	for _, f := range a.locks {
		f.Close()
	}
	a.locks = nil
}
func List() ([]Status, error) {
	root := filepath.Join(dataRoot(), "runner")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []Status{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Status{}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		var s Status
		if err = json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if s.FinishedAt == nil && syscall.Kill(s.PID, 0) != nil {
			s.State = "interrupted"
		}
		out = append(out, s)
	}
	return out, nil
}
