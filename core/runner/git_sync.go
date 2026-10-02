package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Gerc0g/dotfiles/core/world"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type gitState struct {
	Common   string
	Checkout string
	Private  string
	Branch   string
	Head     string
	Index    string
	Baseline string
}

var objectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var branchRef = regexp.MustCompile(`^refs/heads/[A-Za-z0-9][A-Za-z0-9._/-]*$`)

func safeBranch(value string) bool {
	return branchRef.MatchString(value) && !strings.Contains(value, "..") && !strings.Contains(value, "//") && !strings.HasSuffix(value, ".lock") && !strings.HasSuffix(value, "/")
}
func gitLocations(b Binding) (string, string, error) {
	root, err := world.Root()
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(b.RepoID, "/")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid bound repository")
	}
	for _, part := range parts {
		if err = world.ValidateSegment(part); err != nil {
			return "", "", err
		}
	}
	common, err := world.SafePath(root, append(parts, ".git")...)
	if err != nil {
		return "", "", err
	}
	checkout := filepath.Join(b.WorktreePath, ".git")
	info, err := os.Lstat(checkout)
	if err != nil {
		return "", "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("symlink Git metadata denied")
	}
	if info.IsDir() {
		if checkout != common {
			return "", "", fmt.Errorf("worktree metadata must belong to the bound repository")
		}
		return common, checkout, nil
	}
	raw, err := os.ReadFile(checkout)
	if err != nil {
		return "", "", err
	}
	if !strings.HasPrefix(string(raw), "gitdir: ") {
		return "", "", fmt.Errorf("invalid Git marker")
	}
	checkout = strings.TrimSpace(strings.TrimPrefix(string(raw), "gitdir: "))
	if !filepath.IsAbs(checkout) {
		checkout = filepath.Join(b.WorktreePath, checkout)
	}
	checkout = filepath.Clean(checkout)
	expected := filepath.Join(common, "worktrees")
	relative, err := filepath.Rel(expected, checkout)
	if err != nil || strings.Contains(relative, string(filepath.Separator)) || relative == ".." || relative == "." {
		return "", "", fmt.Errorf("Git directory is outside the bound repository")
	}
	checkout, err = world.SafePath(common, "worktrees", relative)
	if err != nil {
		return "", "", err
	}
	return common, checkout, nil
}
func readHead(root *os.Root) (string, error) {
	raw, e := root.ReadFile("HEAD")
	if e != nil {
		return "", e
	}
	head := strings.TrimSpace(strings.TrimPrefix(string(raw), "ref: "))
	if !safeBranch(head) {
		return "", fmt.Errorf("HQ requires a named task branch; detached or invalid HEAD denied")
	}
	return head, nil
}
func readRef(root *os.Root, branch string) (string, error) {
	raw, e := root.ReadFile(branch)
	if e == nil {
		value := strings.TrimSpace(string(raw))
		if !objectID.MatchString(value) {
			return "", fmt.Errorf("invalid Git object ID")
		}
		return value, nil
	}
	if !os.IsNotExist(e) {
		return "", e
	}
	raw, e = root.ReadFile("packed-refs")
	if os.IsNotExist(e) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	for _, line := range strings.Split(string(raw), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == branch {
			if !objectID.MatchString(parts[0]) {
				return "", fmt.Errorf("invalid packed object ID")
			}
			return parts[0], nil
		}
	}
	return "", nil
}
func indexDigest(root *os.Root) (string, error) {
	raw, e := root.ReadFile("index")
	if os.IsNotExist(e) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func beginGit(b Binding, private string) (*gitState, error) {
	common, checkout, e := gitLocations(b)
	if e != nil {
		return nil, e
	}
	c, e := os.OpenRoot(common)
	if e != nil {
		return nil, e
	}
	defer c.Close()
	w, e := os.OpenRoot(checkout)
	if e != nil {
		return nil, e
	}
	defer w.Close()
	branch, e := readHead(w)
	if e != nil {
		return nil, e
	}
	head, e := readRef(c, branch)
	if e != nil {
		return nil, e
	}
	index, e := indexDigest(w)
	if e != nil {
		return nil, e
	}
	baseline := filepath.Join(dataRoot(), "runner/git-baselines", b.HomeKey+".json")
	state := &gitState{Common: common, Checkout: checkout, Private: private, Branch: branch, Head: head, Index: index, Baseline: baseline}
	if raw, e := os.ReadFile(baseline); e == nil {
		var previous gitState
		if json.Unmarshal(raw, &previous) != nil {
			return nil, fmt.Errorf("invalid Git baseline")
		}
		if previous.Branch != branch || previous.Head != head || previous.Index != index {
			return nil, fmt.Errorf("HQ Git conflict: host state changed; private task Git preserved for review")
		}
		if _, e = os.Stat(filepath.Join(private, "HEAD")); e != nil {
			return nil, e
		}
		return state, nil
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if _, e = prepareGit(b, private); e != nil {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Dir(baseline), 0700); e != nil {
		return nil, e
	}
	raw, _ := json.Marshal(state)
	if e = os.WriteFile(baseline, raw, 0600); e != nil {
		return nil, e
	}
	return state, nil
}
func cleanGitEnv() []string {
	return []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/nonexistent", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_COUNT=0", "LC_ALL=C"}
}
func verifyGit(ctx context.Context, dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	gitArgs := append([]string{"--git-dir", dir, "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	if runtime.GOOS == "linux" {
		cmd = exec.CommandContext(ctx, "/usr/bin/prlimit", append([]string{"--as=1073741824", "--cpu=20", "--fsize=536870912", "--", "/usr/bin/git"}, gitArgs...)...)
	}
	cmd.Env = cleanGitEnv()
	cmd.Stdout = nil
	cmd.Stderr = nil
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("Git validation failed: %w", e)
	}
	return nil
}
func finishGit(ctx context.Context, s *gitState) error {
	candidate, e := os.OpenRoot(s.Private)
	if e != nil {
		return e
	}
	defer candidate.Close()
	branch, e := readHead(candidate)
	if e != nil {
		return e
	}
	if branch != s.Branch {
		return fmt.Errorf("HQ Git conflict: task switched branch; private Git preserved")
	}
	head, e := readRef(candidate, branch)
	if e != nil {
		return e
	}
	temp, e := os.MkdirTemp(filepath.Join(dataRoot(), "runner"), "git-import-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(temp)
	if e = os.MkdirAll(filepath.Join(temp, "refs/heads"), 0700); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(temp, "config"), []byte("[core]\n bare = true\n hooksPath = /dev/null\n"), 0600); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(temp, "HEAD"), []byte("ref: "+branch+"\n"), 0600); e != nil {
		return e
	}
	if e = copyDataTree(candidate, "objects", temp); e != nil {
		return e
	}
	if head != "" {
		if e = os.MkdirAll(filepath.Dir(filepath.Join(temp, branch)), 0700); e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(temp, branch), []byte(head+"\n"), 0600); e != nil {
			return e
		}
	}
	if e = copyDataTree(candidate, "index", temp); e != nil {
		return e
	}
	if e = verifyGit(ctx, temp, "fsck", "--full", "--strict", "--no-reflogs"); e != nil {
		return e
	}
	if e = verifyGit(ctx, temp, "ls-files", "--stage"); e != nil {
		return e
	}
	host, e := os.OpenRoot(s.Common)
	if e != nil {
		return e
	}
	defer host.Close()
	worktree, e := os.OpenRoot(s.Checkout)
	if e != nil {
		return e
	}
	defer worktree.Close()
	if e = host.MkdirAll(filepath.Dir(branch), 0700); e != nil {
		return e
	}
	refLock, e := host.OpenFile(branch+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return fmt.Errorf("HQ Git conflict: branch is locked")
	}
	defer func() { refLock.Close(); host.Remove(branch + ".lock") }()
	indexLock, e := worktree.OpenFile("index.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return fmt.Errorf("HQ Git conflict: index is locked")
	}
	defer func() { indexLock.Close(); worktree.Remove("index.lock") }()
	headLock, e := worktree.OpenFile("HEAD.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return fmt.Errorf("HQ Git conflict: HEAD is locked")
	}
	defer func() { headLock.Close(); worktree.Remove("HEAD.lock") }()
	currentBranch, e := readHead(worktree)
	if e != nil {
		return e
	}
	currentHead, e := readRef(host, branch)
	if e != nil {
		return e
	}
	currentIndex, e := indexDigest(worktree)
	if e != nil {
		return e
	}
	if currentBranch != s.Branch || currentHead != s.Head || currentIndex != s.Index {
		return fmt.Errorf("HQ Git conflict: host HEAD or index changed; private Git preserved")
	}
	validated, e := os.OpenRoot(temp)
	if e != nil {
		return e
	}
	defer validated.Close()
	if e = fs.WalkDir(validated.FS(), "objects", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return host.MkdirAll(path, 0700)
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("invalid object file")
		}
		raw, e := validated.ReadFile(path)
		if e != nil {
			return e
		}
		out, e := host.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0444)
		if errors.Is(e, os.ErrExist) {
			return nil
		}
		if e != nil {
			return e
		}
		_, e = out.Write(raw)
		closeErr := out.Close()
		if e != nil {
			return e
		}
		return closeErr
	}); e != nil {
		return e
	}
	rawIndex, e := validated.ReadFile("index")
	hasIndex := e == nil
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if hasIndex {
		if _, e = indexLock.Write(rawIndex); e != nil {
			return e
		}
		if e = indexLock.Close(); e != nil {
			return e
		}
	}
	if head != "" {
		if _, e = refLock.Write([]byte(head + "\n")); e != nil {
			return e
		}
		if e = refLock.Close(); e != nil {
			return e
		}
		if e = host.Rename(branch+".lock", branch); e != nil {
			return e
		}
	}
	if hasIndex {
		if e = worktree.Rename("index.lock", "index"); e != nil {
			return e
		}
	}
	s.Head = head
	s.Index, e = indexDigest(worktree)
	if e != nil {
		return e
	}
	raw, _ := json.Marshal(s)
	return os.WriteFile(s.Baseline, raw, 0600)
}
