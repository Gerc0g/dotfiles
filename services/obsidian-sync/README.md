# WikiPedik: Obsidian Sync and GitHub archive

Obsidian is the knowledge UI on desktop and phone. The official Obsidian
Headless client joins the same remote vault on the HQ server. Obsidian Sync owns
bidirectional file synchronization; GitHub stores versioned snapshots only.
Happier retains Work/Research chats and HQ settings, without a wiki editor,
search, graph, or memory-maintenance screen. HQ's scoped memory, capture broker,
and hourly maintenance remain active.

## Server client

The official client is currently an open beta. Use it only on the headless
server; the desktop app remains the only Sync client on the Mac. Upstream:
<https://obsidian.md/help/sync/headless>.

Run `install-client.sh` as the unprivileged `agent` user. It installs Node
22.23.3 (verified against the official SHA-256) and `obsidian-headless` 0.0.14
under `~/.local/lib/obsidian-headless`, with a private `~/.local/bin/ob` wrapper.
It does not install global npm packages or change the system runtime.

Authenticate interactively, never by putting passwords in shell history:

```sh
ssh -t agent '~/.local/bin/ob login'
ssh agent '~/.local/bin/ob sync-list-remote'
```

For the initial migration, install `connect-vault.sh` as the user's
`~/.local/bin/obsidian-connect` and run `ssh -t agent '~/.local/bin/obsidian-connect'`.
It prompts for the existing remote vault and its encryption password, then
downloads a staging copy in pull-only mode. It neither uploads the old server
copy nor starts the persistent service. A prior successful `ob login` is reused.
For a remote vault with standard encryption, the official client obtains the
managed key from the account and does not prompt for an E2EE password.

As of 2026-10-01, the server is connected to the existing E2EE remote vault
`WikiPedik`, Europe, ID `f677…6f56`. The owner recovered
the password and completed authentication. The proposed standard-encryption
replacement was never created; the existing vault and its history are intact.
The service is enabled and running against `/home/agent/Desktop/WikiPedik`.
Mac/server creation, editing and deletion were verified through Sync, with all
689 working files matching by SHA-256. The phone remains on the same remote;
it has not been separately inspected. See snapshots and reconciliation evidence
in [HQ server delivery](../../docs/platform/hq-server-delivery.md).

Initial migration must back up the Mac and server vaults, including the server's
uncommitted changes. Download the remote vault into an empty staging directory
first, in `pull-only` mode with configuration syncing disabled and all attachment
types enabled. Do not initially sync the stale server copy: first sync resolves
same-path files by modification time and can preserve an older document that a
maintenance job recently rewrote. Reconcile server-only edits against its Git
baseline and the current remote copy before enabling bidirectional sync. Keep
both versions of genuinely conflicting edits for owner review.

Final path: `/home/agent/Desktop/WikiPedik`. Sync state and credentials stay in
the owner's `~/.config/obsidian-headless`, outside the vault, Git and task mounts.
Keep `.git` local to the Mac archive writer. The server does not pull or push
the GitHub repository. Preserve any old server Git metadata in the migration
backup.

The completed relocation preserved the downloaded per-vault Sync database and
encryption fields, changing only the private configuration's `vaultPath` before
enabling bidirectional mode. The migration verified relative database paths,
zero pending downloads and unchanged source hashes. The old server Git directory
is retained at `~/.local/state/obsidian-headless/server-git-before-live-sync`.
Private state backups include credentials: keep them outside the vault, Git,
release bundles and diagnostic output.

After migration and account/vault setup:

```sh
~/.local/bin/ob sync-config --path ~/Desktop/WikiPedik \
  --mode bidirectional --conflict-strategy conflict \
  --file-types image,audio,video,pdf,unsupported --configs '' \
  --device-name hq-vps
mkdir -p ~/.config/systemd/user
install -m 600 obsidian-sync.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now obsidian-sync.service
systemctl --user status obsidian-sync.service
~/.local/bin/ob sync-status --path ~/Desktop/WikiPedik
```

The account needs lingering (`loginctl show-user agent -p Linger`). The unit
restarts after failures and writes logs to the user journal. Its filesystem
write scope is the vault and private Sync state. `sync-status` reports the
configuration; inspect the journal and verify a round trip through another
device to prove synchronization. Never print the private configuration file:
it contains the vault encryption key.

## GitHub archive

The Mac's existing `vault-sync` launchd routine retains its stable identifier
and six-hour schedule. It invokes `hq wiki autocommit`, which records local
changes and performs a normal push. It must never pull, rebase, force-push or
restore files into the live Sync vault. A rejected push is a visible failure
requiring manual reconciliation; local notes and commits remain intact.

The archive is `JustChimera/WikiPedik`, private. The Mac is its sole automatic
writer. While the Mac is offline, Obsidian Sync continues between the server and
phone; GitHub snapshots resume when the Mac runs again. Recovery from Git is an
explicit restore into a separate directory, followed by selective recovery.

## Maintenance and recovery

`hq-memory.timer` continues hourly index, lint, candidate review and company
`hot.md` refreshes. Its journal is separate from file sync:

```sh
systemctl status hq-memory.timer
journalctl -u hq-memory.service
journalctl --user -u obsidian-sync.service
```

Sync conflicts create separate files rather than silently combining competing
edits. Review those in Obsidian. Stop the Sync service before a vault restore or
bulk migration; do not roll back Sync state independently of its vault snapshot.
The release installer must not bundle or replace vaults, Sync credentials or
state. Code rollback does not roll back user data.
