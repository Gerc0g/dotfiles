package workspace

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// git runs git in a directory and returns trimmed stdout.
//
// Shelling out rather than linking a git library is deliberate: every operation
// here is one the user could type by hand, and matching git's own semantics for
// worktrees, upstreams and merge-base is exactly what a library would have to
// re-implement anyway.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(errOut.String()))
	}

	return strings.TrimSpace(out.String()), nil
}

// gitOK reports whether a git command succeeded, discarding its output. Used
// for the many `show-ref --verify --quiet` style probes.
func gitOK(dir string, args ...string) bool {
	_, err := git(dir, args...)
	return err == nil
}

// gitLines runs git and splits stdout into non-empty lines.
func gitLines(dir string, args ...string) ([]string, error) {
	out, err := git(dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines, nil
}

// gitCount runs a rev-list --count style command and parses the number.
func gitCount(dir string, args ...string) (int, error) {
	out, err := git(dir, args...)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// isRepo reports whether dir is a git checkout.
func isRepo(dir string) bool {
	return gitOK(dir, "rev-parse", "--git-dir")
}

// hasCommits reports whether the repository has any commit yet. A freshly
// created repo has none, and worktrees have to be created as orphans there.
func hasCommits(dir string) bool {
	return gitOK(dir, "rev-parse", "--verify", "HEAD")
}

// isClean reports whether the working tree has no modified or untracked files.
func isClean(dir string) (bool, error) {
	out, err := git(dir, "status", "--short")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

// dirtyCount counts entries in `git status --short`.
func dirtyCount(dir string) int {
	lines, err := gitLines(dir, "status", "--short")
	if err != nil {
		return 0
	}
	return len(lines)
}

// hasUpstream reports whether the current branch tracks a remote branch.
func hasUpstream(dir string) bool {
	return gitOK(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
}

// unpushed counts commits present locally but not on the upstream branch.
func unpushed(dir string) (int, error) {
	return gitCount(dir, "rev-list", "--count", "@{u}..HEAD")
}

// currentBranch reports the checked-out branch, or "" in a detached state.
func currentBranch(dir string) string {
	branch, err := git(dir, "branch", "--show-current")
	if err != nil {
		return ""
	}
	return branch
}
