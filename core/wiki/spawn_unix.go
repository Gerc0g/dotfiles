//go:build !windows

package wiki

import "syscall"

// detachAttr puts the drain in its own process group, so it survives the
// session that spawned it and does not receive its signals.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
