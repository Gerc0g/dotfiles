// Package server will own the multi-machine model: which boxes are registered,
// what runs on each (dev-stack, agents, builds) and how a workspace targets
// one. Today it is a deliberate stub — the surface exists so the model gets
// designed as a whole instead of accreting piecemeal, and so the CLI keeps
// zero domain logic even for a stub.
//
// The previous server layer assumed exactly one machine at a fixed ssh alias:
// provisioning script, tmux sessions over ssh, a remote dev-stack, a tailnet
// route pin. Every piece hardcoded that single box, so adding a second would
// have meant rewriting all of them. It was removed rather than adapted.
package server

import "errors"

// Server is one registered machine. The shape is a sketch for the future
// model; nothing persists it yet.
type Server struct {
	// Name is the short handle a workspace or dev-stack targets.
	Name string
	// Host is how the machine is reached: tailnet address or ssh alias.
	Host string
}

// ErrNotImplemented is returned by every mutating operation until the
// multi-server model lands.
var ErrNotImplemented = errors.New("серверный слой пока не реализован.\n\n" +
	"Старый был удалён целиком: он предполагал ровно одну машину по\n" +
	"фиксированному ssh-алиасу, и второй сервер в него не помещался.\n" +
	"Новая модель будет рассчитана на несколько машин.")

// Registry is the set of registered servers. The zero value is the empty
// registry; Load is where persistence will attach later.
type Registry struct {
	servers []Server
}

// Load reads the registry. There is no storage yet, so it is always empty —
// which is also the honest answer `hq server` gives.
func Load() (*Registry, error) {
	return &Registry{}, nil
}

// List reports the registered servers.
func (r *Registry) List() []Server {
	return append([]Server(nil), r.servers...)
}

// Connect registers a machine.
func (r *Registry) Connect(name string) error {
	return ErrNotImplemented
}

// Remove unregisters a machine.
func (r *Registry) Remove(name string) error {
	return ErrNotImplemented
}
