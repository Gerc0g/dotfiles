package knowledge

import (
	"errors"
	"os"
	"path/filepath"
)

// OS advisory locks are released on process exit, including abrupt termination.
func acquireScopeLock(scope Scope) (func(), error) {
	if err := os.MkdirAll(dataRoot(), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dataRoot(), "knowledge-"+digest([]byte(scopeKey(scope)))+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	ok, err := lockFile(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	if !ok {
		file.Close()
		return nil, errors.New("memory job already running")
	}
	return func() { unlockFile(file); file.Close() }, nil
}
