// Package secret will own scoped 1Password secrets. Today it is a deliberate
// stub: the previous implementation was removed while the layer is being
// redesigned, and the surface stays so the new model lands in a ready slot.
//
// The contract worth preserving from the old design:
//   - one vault per company: `Work-<company>`;
//   - item names encode the scope: `_company__VAR`, `<product>__VAR`,
//     `<product>__<repo>__VAR`;
//   - projects consume values through their .envrc, with a local TTL cache so
//     direnv does not hit Touch ID on every cd;
//   - plaintext secrets never enter git.
//
// `secret signin` stays in the shell either way: `eval "$(op signin)"`
// mutates the session environment, which a child process cannot do.
package secret

import "errors"

// ErrNotImplemented is returned by every operation until the new secrets
// layer lands.
var ErrNotImplemented = errors.New("слой секретов пока не реализован.\n\n" +
	"Старая реализация (скоупные item'ы + TTL-кэш) удалена на время\n" +
	"редизайна. Контракт нового слоя описан в докстринге core/secret.")
