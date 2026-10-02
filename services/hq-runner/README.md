# HQ Codex task runtime

`core/runner` owns the Linux execution boundary. Happier's daemon remains on the
host and launches `hq runner app-server --directory <registered-root> --preset
work|research|ordinary` over stdio. The exact WikiPedik `research` root automatically
selects Research, including launches from clients without a preset field. The
exact owner-managed
`$HQ_DATA_ROOT/runner/workspaces/ordinary` directory similarly selects Ordinary.
`hq setup server` provisions it; `hq ls --json` advertises `ordinary.path` only
when that canonical directory exists and is not an aliased descendant. Catalog
reads and runner resolution never create it, and arbitrary directories remain
rejected.
`hq runner resolve` validates the binding without creating a task;
`hq runner status` reports observed lifecycle outcomes.

The root-owned `/etc/hq/server-mode` marker enables the Happier server policy.
In that mode the trusted executable is `/usr/local/bin/hq`; provider/profile
binary overrides cannot select another launcher. The wrapper must provide fixed
`HQ_DATA_ROOT`, `PROKECTFILES_ROOT`, `HQ_WIKI_ROOT`, `HQ_SKILLS_ROOT`, and Codex auth
source paths. Do not mount the wrapper into tasks: the runner mounts its actual
compiled HQ executable, which contains the sandbox-side command and broker client.

The materialized preset config and a small RPC policy file are mounted read-only.
For the pinned Codex 0.157.1 app-server v2 protocol, `thread/start`,
`thread/resume`, `thread/fork` and `turn/start` retain the selected preset model,
reasoning effort and service tier even when an older client sends overrides.
Thread config pins web search, and collaboration mode retains the same model and
effort. Other RPCs and approval replies retain their original payloads. Settings
changes apply when a new task starts; already running tasks retain their snapshot.

## Provisioning

Install Podman, uidmap, slirp4netns and fuse-overlayfs. Provision subuid/subgid
ranges, enable lingering for the unprivileged service account, and delegate
cgroup v2 to its user service. Run Podman in that user's environment, including
`HOME`, `XDG_RUNTIME_DIR`, and `DBUS_SESSION_BUS_ADDRESS`; its working directory
must be readable by that user.

```sh
podman build -f Containerfile -t localhost/hq-codex:0.157.1 .
```

HQ fails closed when Podman, its image, cgroup resource enforcement, the selected
workspace or server Codex login is unavailable. It never falls back to a host
agent process.

## Boundary

Each task has private user, PID, IPC, UTS, cgroup and network namespaces; an
immutable image; no capabilities; no new privileges; 2 GiB memory including
swap, 1.5 CPUs, 256 processes, a 512 MiB temporary filesystem and a 512 MiB
per-file write limit. There are two container slots and one active task per
company (Research and Ordinary each have a separate scope). Busy admission is an
immediate error.
Stable slot container names prevent an orphan from creating an extra slot.

Only the selected checkout/worktree, its private Codex home, and its company
memory are mounted. Research receives only the research directory and its
separate home. Ordinary receives only its managed ordinary workspace and the
private `runner/homes/ordinary` home, with no company or research mounts, company
broker, bound-entity context, or company-memory hooks. Its workspace persists
between Ordinary sessions and is not part of Obsidian Sync. Company memory is read-only; `memory.capture` appends a candidate
to the bound repository inbox. With inbox review enabled, task exit invokes the
canonical knowledge review job, which retains human review requirements.

Company and product tasks use a persistent private draft directory at their
canonical logical cwd. Product tasks additionally mount each registered child
repository read-only, with Git metadata and known credentials hidden. Company
tasks receive no child source checkout. Read-only product sources do not count
against the writable disk budget. `hq ctx --plain` and `hq entity show/update`
route to the task socket. Related cards are readable; only the exact bound
entity document accepts content updates with a current revision. Human
confirmation and policy edits require the owner UI. `.hq` and workspace metadata
are masked against direct shell edits in writable repository tasks.
Linked worktrees also receive their owning canonical checkout read-only for
canonical repo analysis, with Git metadata and credentials hidden. No other
repository is added to a linked-worktree task.

`document.get` / `document.save` publish a revision-checked analysis document
only at `ARCHITECTURE.md` or `docs/ARCHITECTURE.md` for products, and `design.md`
or `docs/design.md` for repositories. Other scratch files remain drafts.
Public onboarding templates come from fixed `HQ_TEMPLATES_ROOT` and appear at
`/home/codex/dotfiles/templates`; they contain no host credentials or history.

Known credential files (`.env*`, `.netrc`, `.npmrc`, common private-key names and
`credentials.json`) and local provider/cloud credential directories are masked
read-only without changing their host contents. This is a filename policy, not
a scanner that can identify secrets embedded in arbitrary source or Git history.
No daemon home, controller socket, rootful Docker socket, shared Git config or
other company checkout is mounted.

The container has no external route. A single bound Unix socket supplies HTTPS
CONNECT and the narrow company broker. The proxy validates DNS answers, dials
validated numeric addresses, and denies private/special networks, the control
host and configured environment destinations. With internet disabled, only
Codex API/auth destinations are allowed. `HQ_RUNNER_BLOCKED_IPS` can add
comma-separated public infrastructure addresses. Git and environment credentials
remain in the host broker. `hq runner broker OPERATION JSON` cannot select a
company or repository.

## Git, history and limits

Git uses a persistent private metadata directory with a generated read-only
configuration. On exit HQ validates objects and the index in a fresh bare Git
directory with no inherited config/helpers/hooks, then imports only objects,
the selected worktree index and the original task branch. It compares host HEAD
and index under Git lock files. A task branch switch or concurrent host change
produces `git-conflict`; private metadata is retained for owner review. No force
push, remote operation, unrelated ref import or automatic conflict resolution
occurs. Detached HEAD is refused.

Runtime history and task-specific Codex auth survive restarts in the bound home.
The server login supplies the initial auth file. Task-modified auth is never
copied into the shared server account; expired scoped authentication requires
owner renewal. Mac auth and history are not implicitly activated.

Workspace plus runtime-home usage is checked every two seconds against 4 GiB;
admission requires 5 GiB free host disk. Crossing either limit stops the task and
records `disk-limit`. This watchdog is not a filesystem byte quota: concurrent
writes may temporarily exceed the threshold. Storage for completed homes and
status records is retained; operators must account for it in server backups and
capacity management.

## Live verification

After loading the current HQ and Happier builds, launch a disposable registered
repository through the actual Happier session path and verify Codex initialize,
a model response, filesystem edits, and a scoped commit. Inspect the running
`hq-task-slot-*` container as the owner: check cgroup memory/CPU/PID values,
private process visibility, no host network route, denied sibling-company files,
masked credentials, inaccessible daemon keys and controller sockets. Check a
second same-company launch is refused and Git state appears on the host after
stop. Repeat Research with no company mount or broker access. A unit test or an
image build alone does not prove this composed runtime gate.
