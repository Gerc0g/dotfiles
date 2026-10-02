# HQ server release installation

The server installer consumes a trusted local release directory. Company data,
WikiPedik, credentials, imported history and relay databases are separate from
release files. The installer never imports a Mac home or activates archive auth.

Build the HQ binary and Happier artifacts through their existing build owners,
then pack their completed outputs:

```sh
python3 services/hq-server/build-bundle.py \
  --hq /path/to/linux/hq \
  --cli /path/to/happier-cli-payload \
  --relay /path/to/public-relay-runtime \
  --ui /path/to/exported-web \
  --skills /path/to/release-skills \
  --templates /path/to/dotfiles/templates \
  --codex /path/to/public-codex-provider-payload \
  --version 2026.09.28.1 --platform linux-amd64 \
  --output /path/to/hq-2026.09.28.1
```

The CLI payload must contain its binary entrypoint `happier` and its complete
runtime dependency closure. The web payload must contain `index.html`. Build the
web client with the HQ feature enabled and the intended private relay URL. The
packer follows only symlinks that stay inside the selected component, writes a
SHA-256 for every regular file, and excludes common credential/state paths.
Unknown files, escaping symlinks and modified checksums are rejected again by the
installer. SHA-256 verifies integrity; obtain the bundle from a trusted producer.
The relay payload is its complete public runtime directory: `happier-server`,
`node_modules` with the generated Prisma client and platform engine, and
`prisma/sqlite/migrations`. The reviewed VPS source is `/opt/happier/bin/`;
copy that directory, never the relay data directory or its database. The packer
and installer reject a missing Prisma client, platform engine or migrations.
The fresh relay unit selects these release-local resources and sets `PUBLIC_URL`;
existing relay units and their runtime paths remain intact.
The templates payload must contain `AGENTS.md.company.tmpl`,
`AGENTS.md.product.tmpl` and `AGENTS.md.repo.tmpl` from the reviewed dotfiles source.
These are public authoring templates; existing company documents are never packed.
The Codex payload contains its pinned public binaries and resources, including
executable `codex/bin/codex`. It never contains the Codex home or auth state.

`--codex-image registry/image@sha256:...` records the exact runner image expected
by the release. It does not pull or build the image. The runtime owner currently
uses the image defined in `services/hq-runner/Containerfile`; verify its local tag
against the release digest before enabling agent execution.

## First host

`bootstrap.sh` exposes both installation modes. The server mode provisions the
Debian/Ubuntu packages and rootless container image, then calls the verified HQ
installer. It does not run the Mac setup or install Claude:

```sh
sudo ./bootstrap.sh --mode server --bundle /path/to/bundle \
  --private-url https://hq.private.example --dry-run
sudo ./bootstrap.sh --mode server --bundle /path/to/bundle \
  --private-url https://hq.private.example --activate
```

`--activate` explicitly starts the loopback relay and memory timer and restarts
the owner daemon. Omit it when preparing an update before a maintenance window.
Normal `./bootstrap.sh` (or `--mode workstation`) retains the Mac workstation
flow. Authentication and the existing private VPN/HTTPS route are prerequisites
for using a fresh installation; the script never invents or imports credentials.

Use an existing Linux host with its private VPN/HTTPS route. The installer does
not modify DNS, firewall, VPN, certificates, a reverse proxy or public listeners.
Create the unprivileged `agent` account if it is absent, and configure rootless
Podman plus subordinate UID/GID ranges through the host's normal provisioning.
No task receives the Podman management socket or the daemon home.

The `hq setup --mode server` CLI calls `serversetup.Run`; review before applying:

```sh
sudo /path/to/bundle/hq setup --mode server \
  --bundle /path/to/bundle --private-url https://hq.private.example --dry-run
sudo /path/to/bundle/hq setup --mode server \
  --bundle /path/to/bundle --private-url https://hq.private.example
```

A missing account, runner image, Codex authentication or private endpoint remains
an explicit prerequisite. File installation is not proof that a fresh host can
run tasks. Complete the commands reported in `nextSteps`:

1. Review the user daemon unit and its HQ drop-in; existing base units survive.
2. Review a fresh relay unit before enabling it. It listens on `127.0.0.1:3005`.
   Existing relay units and their configuration are preserved.
3. Pair the Happier daemon as `agent`, using the existing private relay URL.
   Existing login files are retained. Provision Codex authentication separately
   into the owner broker; imported archive auth is never used.
4. Enable user lingering and explicitly reload/enable the daemon and fresh relay.
5. Verify Work and Research sessions, cross-company denial, and phone access.

The installer does not restart services. Activation belongs to the operator's
reviewed deployment step, so installing a new version does not terminate chats.
The daemon unit and managed drop-in use `KillMode=process`: detached sessions
survive daemon replacement and keep their own lifecycle.
The daemon uses `HAPPIER_LOCAL_SERVER_URL=http://127.0.0.1:3005` for its local
relay connection while `HAPPIER_SERVER_URL` and `HAPPIER_WEBAPP_URL` retain the
private HTTPS identity. Local daemon registration does not depend on the host
resolving its client-facing private DNS name.

## Storage and updates

- `/usr/local/lib/hq/releases/<version>`: immutable checksummed payload.
- `/usr/local/lib/hq/current`: atomic active-release pointer.
- `/usr/local/lib/hq/previous`: previous pointer for rollback.
- `/usr/local/bin/hq`: root-owned wrapper with fixed server data roots.
- `/home/agent/.local/bin/hq`: link to that wrapper.
- `/home/agent/.happier/cli/current`: link to the active CLI payload.
- `/srv/hq-data`: private owner state, SQLite jobs, policies and imported history.
- `/srv/hq-data/runner/workspaces/ordinary`: persistent isolated ordinary-chat files, outside Obsidian Sync.
- `/srv/projects`: company repositories; existing subdirectories are preserved.
- `/home/agent/Desktop/WikiPedik`: existing vault; scope roots are preserved.

The wrapper and daemon drop-in set the daemon user's `HOME`, runtime directory,
DBus address and Codex broker file (`/home/agent/.codex/auth.json`). They also pin
`HQ_PROJECTS_FILE=/home/agent/.local/state/hq/projects.json` and the release's
`HQ_SKILLS_ROOT`. Existing credentials and the project registry are retained;
credentials are provisioned separately, never packaged. The installed skills are
therefore the verified release's skills even when an older dotfiles checkout
exists on the host.
`HQ_TEMPLATES_ROOT` similarly selects the release's verified onboarding templates.
`HQ_CODEX_BINARY` pins the release's `codex/bin/codex`; Happier's independently
managed provider `current` link is retained. The private authentication path is
separate from both executable installations.

The generated `hq-memory.timer` runs hourly with up to five minutes of jitter.
After release activation, enable it with `systemctl enable --now hq-memory.timer`.
Its unprivileged oneshot runs `hq knowledge tick --json`: separate lint, index and
inbox-review jobs per research/company scope, plus company hot-context refresh.
The owner-only full-vault view is excluded. Jobs and failures remain visible in
the HQ job journal; inbox promotion and synthesis still require human review. The timer
does not commit, push, contact a model, drain inboxes or run laptop scripts.

Obsidian is the owner-facing knowledge UI. Continuous file synchronization with
desktop and phone uses the separate [Obsidian Headless service](../obsidian-sync/README.md).
Happier retains Work, Research and Ordinary chats and settings; it no longer hosts a wiki
editor or graph. The Mac's Git routine archives snapshots with push only.

Data-root ownership is set to the unprivileged daemon user. This does not recurse
through existing datasets or replace their permissions. Release installation
never truncates, copies, rolls back or removes credentials and datasets. Old
release directories remain available. Reapplying an unchanged bundle is a no-op
for its files; reusing a version with different content is rejected.

For rollback, verify the target recorded by `readlink /usr/local/lib/hq/previous`,
atomically switch `current` to that target, then explicitly restart only the
changed services. Do not restore an older database alongside an older binary.
If a release requires data migration, it needs a separately reviewed migration
and restore procedure; this installer performs none.

The initial implementation is tested against temporary root filesystems.
A clean VPS installation and restored-backup drill must still be exercised
before claiming reproducibility across hosts.

Focused checks from the dotfiles root:

```sh
(cd core && go test ./serversetup)
python3 services/hq-server/build_bundle_test.py
```
