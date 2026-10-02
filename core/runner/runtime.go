package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Gerc0g/dotfiles/core/agentconfig"
	"github.com/Gerc0g/dotfiles/core/audit"
	"github.com/Gerc0g/dotfiles/core/connections"
	"github.com/Gerc0g/dotfiles/core/world"
	"golang.org/x/sys/unix"
)

const Image = "localhost/hq-codex:0.157.1"

func Validate(directory, preset string) (Binding, error) {
	b, err := Resolve(directory, preset)
	if err != nil {
		return b, err
	}
	if runtime.GOOS != "linux" || os.Geteuid() == 0 {
		return b, fmt.Errorf("HQ runner requires non-root Linux")
	}
	if _, err = exec.LookPath("podman"); err != nil {
		return b, fmt.Errorf("HQ rootless Podman is not installed")
	}
	return b, nil
}

// Run owns the process lifetime and keeps stdin/stdout exclusively for app-server JSON-RPC.
func Run(ctx context.Context, directory, preset string, stdin io.Reader, stdout, stderr io.Writer, options ...agentconfig.SessionOptions) (retErr error) {
	b, err := Validate(directory, preset)
	if err != nil {
		return err
	}
	preset = b.Preset
	a, err := acquire(b)
	if err != nil {
		return err
	}
	state := "failed"
	defer func() {
		message := ""
		if retErr != nil {
			message = retErr.Error()
		}
		a.release(state, message)
	}()
	existing, err := exec.CommandContext(ctx, "podman", "ps", "-q", "--filter", "label=hq.company="+containerScope(b)).Output()
	if err != nil {
		return fmt.Errorf("HQ rootless runtime unavailable: %w", err)
	}
	if strings.TrimSpace(string(existing)) != "" {
		return fmt.Errorf("HQ capacity: a task container for this company is still running")
	}
	var disk unix.Statfs_t
	if err = unix.Statfs(dataRoot(), &disk); err != nil {
		return err
	}
	if disk.Bavail*uint64(disk.Bsize) < MinFreeDiskBytes {
		return fmt.Errorf("HQ disk admission denied: less than 5 GiB free")
	}
	if !contextTask(b) {
		size, err := directorySize(b.WorktreePath)
		if err != nil {
			return err
		}
		if size > TaskDiskBytes {
			return fmt.Errorf("HQ task exceeds 4 GiB disk budget")
		}
	}
	task := filepath.Join(dataRoot(), "runner", "tasks", a.status.ID)
	if err = os.MkdirAll(task, 0700); err != nil {
		return err
	}
	staged := filepath.Join(task, "config")
	effective, err := agentconfig.Materialize(preset, staged, options...)
	if err != nil {
		return err
	}
	policy := rpcPolicy{Model: effective.Model, Effort: effective.Effort, ServiceTier: effective.ServiceTier, WebSearch: effective.WebSearch}
	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	policyFile := filepath.Join(task, "rpc-policy.json")
	if err = os.WriteFile(policyFile, policyBytes, 0400); err != nil {
		return err
	}
	home := filepath.Join(dataRoot(), "runner", "homes", b.HomeKey)
	if err = os.MkdirAll(home, 0700); err != nil {
		return err
	}
	if err = applyConfig(staged, home); err != nil {
		return err
	}
	workspaceSource := b.WorktreePath
	var readonlyRepos []string
	if contextTask(b) {
		workspaceSource, readonlyRepos, err = prepareContextWorkspace(b, home)
		if err != nil {
			return err
		}
	}
	if b.Kind == "worktree" {
		root, e := world.Root()
		if e != nil {
			return e
		}
		canonical, e := world.SafePath(root, strings.Split(b.RepoID, "/")...)
		if e != nil {
			return e
		}
		readonlyRepos = append(readonlyRepos, canonical)
	}
	if preset == "work" {
		if err = copyTemplates(home); err != nil {
			return err
		}
		if err = appendBoundInstructions(home, b); err != nil {
			return err
		}
	}
	authFile := os.Getenv("HQ_CODEX_AUTH_FILE")
	if authFile == "" {
		userHome, _ := os.UserHomeDir()
		authFile = filepath.Join(userHome, ".codex/auth.json")
	}
	auth, err := os.ReadFile(authFile)
	if err != nil {
		return fmt.Errorf("HQ Codex login is unavailable; sign in to the server Codex account")
	}
	if !json.Valid(auth) {
		return fmt.Errorf("invalid Codex auth file")
	}
	if _, err = os.Lstat(filepath.Join(home, "auth.json")); os.IsNotExist(err) {
		if err = safeWrite(home, "auth.json", auth); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	gitDir := ""
	var gitSync *gitState
	if b.RepoID != "" {
		gitDir = filepath.Join(home, "git")
		gitSync, err = beginGit(b, gitDir)
		if err != nil {
			return err
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	socketPath := filepath.Join(task, "broker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	listener = limitTaskConnections(listener, 64)
	defer listener.Close()
	if err = os.Chmod(socketPath, 0600); err != nil {
		return err
	}
	environmentHosts, err := connections.EnvironmentHosts()
	if err != nil {
		return err
	}
	proxy := Proxy{Internet: effective.Network, BlockedHosts: environmentHosts}
	addresses, _ := net.InterfaceAddrs()
	for _, address := range addresses {
		if ip, _, e := net.ParseCIDR(address.String()); e == nil {
			proxy.BlockedIPs = append(proxy.BlockedIPs, ip)
		}
	}
	for _, s := range strings.Split(os.Getenv("HQ_RUNNER_BLOCKED_IPS"), ",") {
		if ip := net.ParseIP(strings.TrimSpace(s)); ip != nil {
			proxy.BlockedIPs = append(proxy.BlockedIPs, ip)
		}
	}
	brokerSlot := make(chan struct{}, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/broker" && r.Method == http.MethodPost {
			select {
			case brokerSlot <- struct{}{}:
				defer func() { <-brokerSlot }()
			default:
				http.Error(w, "HQ broker is busy", http.StatusTooManyRequests)
				return
			}
			if preset != "work" {
				http.Error(w, "this preset has no company broker", http.StatusForbidden)
				return
			}
			var req struct {
				Operation string          `json:"operation"`
				Args      json.RawMessage `json:"args"`
			}
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 96<<20))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&req) != nil {
				http.Error(w, "invalid request", 400)
				return
			}
			operation := req.Operation
			switch operation {
			case "context.get", "entity.show", "entity.update", "document.get", "document.save", "memory.capture", "git.read", "git.fetch", "git.push", "git.pr.create", "git.pr.update", "environment.request":
			default:
				operation = "denied"
			}
			if auditErr := audit.Record("runner."+operation, "STARTED"); auditErr != nil {
				http.Error(w, "HQ_AUDIT_UNAVAILABLE", http.StatusServiceUnavailable)
				return
			}
			var result any
			var e error
			switch req.Operation {
			case "context.get", "entity.show", "entity.update":
				result, e = contextOperation(b, req.Operation, req.Args)
			case "document.get", "document.save":
				result, e = documentOperation(b, req.Operation, req.Args)
			case "memory.capture":
				if !effective.Hooks.InboxCapture {
					e = fmt.Errorf("memory capture is disabled")
				} else {
					result, e = captureMemory(b, req.Args)
				}
			default:
				result, e = connections.Execute(r.Context(), connections.Binding{CompanyID: b.CompanyID, RepoID: b.RepoID, WorktreePath: b.WorktreePath}, req.Operation, req.Args)
			}
			outcome := "ok"
			if e != nil {
				outcome = "denied"
			}
			if auditErr := audit.Record("runner."+operation, outcome); auditErr != nil && e == nil {
				e = fmt.Errorf("HQ_OPERATION_APPLIED_AUDIT_UNAVAILABLE")
			}
			w.Header().Set("Content-Type", "application/json")
			if e != nil {
				w.WriteHeader(403)
				json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
				return
			}
			json.NewEncoder(w).Encode(result)
			return
		}
		proxy.ServeHTTP(w, r)
	})
	server := &http.Server{Handler: handler, MaxHeaderBytes: 16 << 10, ReadHeaderTimeout: 15 * time.Second, ReadTimeout: 2 * time.Minute, IdleTimeout: time.Minute}
	go server.Serve(listener)
	defer server.Close()
	args := []string{"run", "--rm", "--interactive", "--name", a.status.Container, "--label", "hq.company=" + containerScope(b), "--pull=never", "--network=none", "--pid=private", "--ipc=private", "--uts=private", "--cgroupns=private", "--read-only", "--cap-drop=all", "--security-opt=no-new-privileges", "--userns=keep-id", "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "--memory", TaskMemory, "--memory-swap", TaskMemory, "--cpus", TaskCPUs, "--pids-limit", TaskPIDs, "--ulimit", "nofile=4096:4096", "--ulimit", "fsize=536870912:536870912", "--tmpfs", "/tmp:rw,nosuid,nodev,size=536870912,mode=1777", "--workdir", b.WorktreePath}
	mount := func(source, dest string, readOnly bool) {
		mode := "rw"
		if readOnly {
			mode = "ro"
		}
		args = append(args, "--volume", source+":"+dest+":"+mode+",rprivate")
	}
	for _, p := range []string{b.WorktreePath, home, task, executable} {
		if strings.ContainsAny(p, ":\n\r") {
			return fmt.Errorf("unsupported mount path")
		}
	}
	mount(workspaceSource, b.WorktreePath, false)
	for _, repo := range readonlyRepos {
		mount(repo, repo, true)
	}
	mount(home, "/home/codex", false)
	mount(filepath.Join(staged, "config.toml"), "/home/codex/config.toml", true)
	mount(policyFile, "/run/hq-policy.json", true)
	mount(executable, "/usr/local/bin/hq", true)
	mount(socketPath, "/run/hq-broker.sock", true)
	if gitDir != "" {
		mount(gitDir, "/home/codex/git", false)
		gitConfig := filepath.Join(task, "git-config")
		if err = os.WriteFile(gitConfig, []byte("[core]\n repositoryformatversion = 0\n bare = false\n hooksPath = /dev/null\n[credential]\n helper =\n[user]\n name = HQ Agent\n email = agent@localhost\n"), 0400); err != nil {
			return err
		}
		mount(gitConfig, "/home/codex/git/config", true)
		info, e := os.Stat(filepath.Join(b.WorktreePath, ".git"))
		if e != nil {
			return e
		}
		if info.IsDir() {
			mount(gitDir, filepath.Join(b.WorktreePath, ".git"), false)
			mount(gitConfig, filepath.Join(b.WorktreePath, ".git/config"), true)
		} else {
			marker := filepath.Join(task, "git-marker")
			if e = os.WriteFile(marker, []byte("gitdir: /home/codex/git\n"), 0400); e != nil {
				return e
			}
			mount(marker, filepath.Join(b.WorktreePath, ".git"), true)
		}
	}
	blank := filepath.Join(task, "masked")
	if err = os.WriteFile(blank, nil, 0400); err != nil {
		return err
	}
	emptyDirectory := filepath.Join(task, "masked-directory")
	if err = os.Mkdir(emptyDirectory, 0500); err != nil {
		return err
	}
	masks, err := secretMasks(workspaceSource, readonlyRepos...)
	if err != nil {
		return err
	}
	for i := range masks {
		if contextTask(b) {
			rel, _ := filepath.Rel(workspaceSource, masks[i].Path)
			if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				masks[i].Path = filepath.Join(b.WorktreePath, rel)
			}
		}
	}
	for _, repo := range readonlyRepos {
		extra, e := secretMasks(repo, readonlyRepos...)
		if e != nil {
			return e
		}
		masks = append(masks, extra...)
		info, e := os.Lstat(filepath.Join(repo, ".git"))
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("repository Git marker symlink denied")
		}
		masks = append(masks, secretMask{filepath.Join(repo, ".git"), info.IsDir()})
	}
	// The task must not forge human confirmation or policy metadata, even when
	// no entity state has been created yet. A read-only bind pins this directory.
	if b.RepoID != "" {
		statePath, e := world.SafePath(b.WorktreePath, ".hq")
		if e != nil {
			return e
		}
		if e = os.MkdirAll(statePath, 0700); e != nil {
			return e
		}
		masks = append(masks, secretMask{statePath, true})
	}
	masked := map[string]bool{}
	for _, mask := range masks {
		if masked[mask.Path] {
			continue
		}
		masked[mask.Path] = true
		if strings.ContainsAny(mask.Path, ":\n\r") {
			return fmt.Errorf("unsupported credential path")
		}
		source := blank
		if mask.Directory {
			source = emptyDirectory
		}
		mount(source, mask.Path, true)
	}
	if preset == "work" {
		memory := filepath.Join(wikiRoot(), "dev/20-projects", b.CompanyID)
		if info, e := os.Stat(memory); e == nil && info.IsDir() {
			canonical, e := filepath.EvalSymlinks(memory)
			if e != nil || canonical != memory {
				return fmt.Errorf("company memory symlink denied")
			}
			mount(memory, memory, true)
		}
		if effective.Hooks.InboxCapture {
			if b.RepoID != "" {
				if err = appendMemoryInstructions(home); err != nil {
					return err
				}
			}
			defer func() {
				if e := reviewMemory(b); e != nil && retErr == nil {
					retErr = fmt.Errorf("HQ memory inbox review failed: %w", e)
					state = "memory-review-failed"
				}
			}()
		}
		if effective.Hooks.SessionContext && b.RepoID != "" {
			if err = appendContext(home, b); err != nil {
				return err
			}
		}
	}
	for _, kv := range []string{"HOME=/home/codex", "CODEX_HOME=/home/codex", "PATH=/usr/local/bin:/usr/bin:/bin", "HTTPS_PROXY=http://127.0.0.1:3130", "HTTP_PROXY=http://127.0.0.1:3130", "ALL_PROXY=http://127.0.0.1:3130", "NO_PROXY=", "https_proxy=http://127.0.0.1:3130", "http_proxy=http://127.0.0.1:3130", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "HQ_RUNNER_SANDBOX=1"} {
		args = append(args, "--env", kv)
	}
	if gitDir != "" {
		args = append(args, "--env", "GIT_DIR=/home/codex/git", "--env", "GIT_WORK_TREE="+b.WorktreePath)
	}
	args = append(args, Image, "hq", "runner", "inside")
	child := exec.CommandContext(ctx, "podman", args...)
	child.Stdin = stdin
	child.Stdout = stdout
	child.Stderr = stderr
	child.WaitDelay = 5 * time.Second
	// Podman's client can die independently. Stop the named container before
	// importing Git, so no task process can mutate metadata during publication.
	containerStopped := false
	stopContainer := func() error {
		if containerStopped {
			return nil
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if e := exec.CommandContext(stopCtx, "podman", "rm", "--force", "--ignore", a.status.Container).Run(); e != nil {
			return fmt.Errorf("HQ task container cleanup failed: %w", e)
		}
		containerStopped = true
		return nil
	}
	defer func() { _ = stopContainer() }()
	if err = child.Start(); err != nil {
		return err
	}
	a.status.State = "running"
	_ = a.save()
	done := make(chan struct{})
	var diskExceeded atomic.Bool
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var used int64
				var e error
				if !contextTask(b) {
					used, e = directorySize(workspaceSource)
				}
				homeSize, he := directorySize(home)
				var s unix.Statfs_t
				se := unix.Statfs(dataRoot(), &s)
				if e != nil || he != nil || se != nil || used+homeSize > TaskDiskBytes || s.Bavail*uint64(s.Bsize) < MinFreeDiskBytes {
					diskExceeded.Store(true)
					_ = exec.Command("podman", "kill", a.status.Container).Run()
					return
				}
			}
		}
	}()
	err = child.Wait()
	close(done)
	if stopErr := stopContainer(); stopErr != nil {
		return stopErr
	}
	if diskExceeded.Load() {
		state = "disk-limit"
		return fmt.Errorf("HQ task stopped: disk limit reached")
	}
	if gitSync != nil {
		syncCtx, syncCancel := context.WithTimeout(context.Background(), 45*time.Second)
		syncErr := finishGit(syncCtx, gitSync)
		syncCancel()
		if syncErr != nil {
			state = "git-conflict"
			return syncErr
		}
	}
	state, err = taskProcessExit(ctx.Err(), err)
	return err
}

// Cleanup, disk and Git failures return before this process-exit classification.
// A requested stop is therefore interrupted only after safe cleanup succeeds.
func taskProcessExit(contextErr, processErr error) (string, error) {
	if errors.Is(contextErr, context.Canceled) {
		return "interrupted", nil
	}
	if processErr != nil {
		var exit *exec.ExitError
		if errors.As(processErr, &exit) && exit.ExitCode() == 137 {
			return "resource-limit", fmt.Errorf("HQ task stopped: memory limit or forced termination")
		}
		return "failed", processErr
	}
	return "completed", nil
}

func safeWrite(dir, name string, raw []byte) error {
	r, e := os.OpenRoot(dir)
	if e != nil {
		return e
	}
	defer r.Close()
	_ = r.Remove(name)
	return r.WriteFile(name, raw, 0600)
}
func applyConfig(source, dest string) error {
	for _, name := range []string{"config.toml", "AGENTS.md"} {
		raw, e := os.ReadFile(filepath.Join(source, name))
		if e != nil {
			return e
		}
		if e = safeWrite(dest, name, raw); e != nil {
			return e
		}
	}
	root, e := os.OpenRoot(dest)
	if e != nil {
		return e
	}
	defer root.Close()
	if e = root.RemoveAll("skills"); e != nil {
		return e
	}
	if _, e = os.Stat(filepath.Join(source, "skills")); os.IsNotExist(e) {
		return nil
	}
	return os.Rename(filepath.Join(source, "skills"), filepath.Join(dest, "skills"))
}
func directorySize(root string) (int64, error) {
	var size int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type().IsRegular() {
			info, e := d.Info()
			if e != nil {
				return e
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}
func appendContext(home string, b Binding) error {
	parts := strings.Split(b.RepoID, "/")
	root, e := world.SafePath(wikiRoot(), "dev", "20-projects", parts[0], parts[1], "repos", parts[2])
	if e != nil {
		return e
	}
	r, e := os.OpenRoot(root)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer r.Close()
	raw, e := r.ReadFile("hot.md")
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if len(raw) > 64<<10 {
		raw = raw[:64<<10]
	}
	f, e := os.OpenFile(filepath.Join(home, "AGENTS.md"), os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = f.Write(append([]byte("\n\n## Scoped project memory\n\n"), raw...))
	return e
}

// Inside runs only inside the container. The loopback proxy forwards raw HTTP
// to its bound task socket; it cannot select another company's broker.
func Inside(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	if os.Getenv("HQ_RUNNER_SANDBOX") != "1" {
		return fmt.Errorf("inside command requires task sandbox")
	}
	l, err := net.Listen("tcp", "127.0.0.1:3130")
	if err != nil {
		return err
	}
	defer l.Close()
	go func() {
		for {
			client, e := l.Accept()
			if e != nil {
				return
			}
			go func() {
				upstream, e := net.Dial("unix", "/run/hq-broker.sock")
				if e != nil {
					client.Close()
					return
				}
				go func() { defer client.Close(); defer upstream.Close(); io.Copy(upstream, client) }()
				go func() { defer client.Close(); defer upstream.Close(); io.Copy(client, upstream) }()
			}()
		}
	}()
	child := exec.CommandContext(ctx, "codex", "app-server", "--listen", "stdio://", "-c", "sandbox_mode=\"danger-full-access\"")
	var policy rpcPolicy
	raw, err := os.ReadFile("/run/hq-policy.json")
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &policy); err != nil {
		return err
	}
	policy, err = policy.resolveDefault(ctx)
	if err != nil {
		return err
	}
	input, err := child.StdinPipe()
	if err != nil {
		return err
	}
	defer input.Close()
	child.Stdout = stdout
	child.Stderr = stderr
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err = child.Start(); err != nil {
		return err
	}
	forwarded := make(chan error, 1)
	go func() {
		err := policy.forward(stdin, input)
		input.Close()
		forwarded <- err
		if err != nil {
			_ = child.Process.Kill()
		}
	}()
	err = child.Wait()
	select {
	case forwardErr := <-forwarded:
		if forwardErr != nil {
			return fmt.Errorf("HQ Codex input policy: %w", forwardErr)
		}
	default:
	}
	return err
}
func Broker(ctx context.Context, operation string, args json.RawMessage) (json.RawMessage, error) {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", "/run/hq-broker.sock")
	}}
	defer transport.CloseIdleConnections()
	body, e := json.Marshal(map[string]any{"operation": operation, "args": args})
	if e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "http://hq/broker", strings.NewReader(string(body)))
	if e != nil {
		return nil, e
	}
	resp, e := (&http.Client{Transport: transport, Timeout: 2 * time.Minute}).Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 96<<20))
	if e != nil {
		return nil, e
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HQ broker refused operation: %s", strings.TrimSpace(string(raw)))
	}
	return raw, nil
}
