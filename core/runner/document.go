package runner

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
	"golang.org/x/sys/unix"
)

type taskDocument struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	Revision string `json:"revision"`
}

func documentOperation(b Binding, operation string, raw json.RawMessage) (any, error) {
	var req struct {
		Name     string  `json:"name"`
		Content  *string `json:"content,omitempty"`
		Revision string  `json:"revision,omitempty"`
	}
	if err := decodeBound(raw, &req); err != nil {
		return nil, err
	}
	allowed := b.Kind == "product" && (req.Name == "ARCHITECTURE.md" || req.Name == "docs/ARCHITECTURE.md") || b.RepoID != "" && (req.Name == "design.md" || req.Name == "docs/design.md")
	if b.Preset != "work" || !allowed {
		return nil, fmt.Errorf("document is outside the bound analysis allowlist")
	}
	root, err := world.Root()
	if err != nil {
		return nil, err
	}
	scope, err := world.SafePath(root, strings.Split(b.EntityRef, "/")...)
	if err != nil {
		return nil, err
	}
	state, err := world.SafePath(scope, ".hq")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(state, 0700); err != nil {
		return nil, err
	}
	lockPath, err := world.SafePath(state, "analysis-doc.lock")
	if err != nil {
		return nil, err
	}
	locked, err := lock(lockPath)
	if err != nil {
		return nil, fmt.Errorf("HQ_DOCUMENT_BUSY")
	}
	defer locked.Close()
	if _, err = world.SafePath(scope, strings.Split(req.Name, "/")...); err != nil {
		return nil, err
	}
	doc := taskDocument{Name: req.Name}
	marker := "missing:"
	data, exists, err := readAnalysisDocument(scope, req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		doc.Content = string(data)
		marker = "present:"
	}
	digest := sha256.Sum256([]byte(marker + doc.Content))
	doc.Revision = hex.EncodeToString(digest[:])
	if operation == "document.get" {
		if req.Content != nil || req.Revision != "" {
			return nil, fmt.Errorf("document.get does not accept an edit")
		}
		return doc, nil
	}
	if operation != "document.save" || req.Content == nil || len(*req.Content) > 1<<20 {
		return nil, fmt.Errorf("invalid analysis document edit")
	}
	if req.Revision != doc.Revision {
		return nil, fmt.Errorf("HQ_DOCUMENT_REVISION_CONFLICT")
	}
	dir, err := analysisParent(scope, req.Name, true)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	var nonce [12]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	tmp := filepath.Base(req.Name) + ".hq-pending-" + hex.EncodeToString(nonce[:])
	// The task never chooses the temporary name or a directory outside scope.
	fd, err := unix.Openat(int(dir.Fd()), tmp, unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), tmp)
	defer unix.Unlinkat(int(dir.Fd()), tmp, 0)
	_, err = file.WriteString(*req.Content)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err = unix.Renameat(int(dir.Fd()), tmp, int(dir.Fd()), filepath.Base(req.Name)); err != nil {
		return nil, err
	}
	doc.Content = *req.Content
	digest = sha256.Sum256([]byte("present:" + doc.Content))
	doc.Revision = hex.EncodeToString(digest[:])
	return doc, nil
}

// Pin every allowed directory without following even an in-scope symlink.
// os.Root alone permits such aliases, which could reveal masked credentials.
func analysisParent(root, name string, create bool) (*os.File, error) {
	parent := filepath.Dir(name)
	if parent != "." && parent != "docs" {
		return nil, fmt.Errorf("invalid analysis document directory")
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open(canonical, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	if parent == "." {
		return os.NewFile(uintptr(fd), canonical), nil
	}
	defer unix.Close(fd)
	if create {
		if err = unix.Mkdirat(fd, "docs", 0700); err != nil && err != unix.EEXIST {
			return nil, err
		}
	}
	child, err := unix.Openat(fd, "docs", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(child), filepath.Join(canonical, "docs")), nil
}

func readAnalysisDocument(root, name string) ([]byte, bool, error) {
	dir, err := analysisParent(root, name, false)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer dir.Close()
	fd, err := unix.Openat(int(dir.Fd()), filepath.Base(name), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, false, fmt.Errorf("analysis document must be a regular file of at most 1 MiB")
	}
	raw, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return nil, false, err
	}
	if len(raw) > 1<<20 {
		return nil, false, fmt.Errorf("analysis document exceeds 1 MiB")
	}
	return raw, true, nil
}
