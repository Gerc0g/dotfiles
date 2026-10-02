package serversetup

import (
	"fmt"
	"io/fs"
)

type managedFile struct {
	content  string
	mode     fs.FileMode
	preserve bool
}

func relayEngine(platform string) string {
	if platform == "linux-arm64" {
		return "libquery_engine-linux-arm64-openssl-3.0.x.so.node"
	}
	return "libquery_engine-debian-openssl-3.0.x.so.node"
}
func managedFiles(o Options, platform string) map[string]managedFile {
	home := "/home/" + o.User
	runtimeDir := fmt.Sprintf("/run/user/%d", o.UID)
	wrapper := `#!/bin/sh
# Installed by HQ server setup. Caller environment cannot redirect owner state.
export HOME=` + home + `
export XDG_RUNTIME_DIR=` + runtimeDir + `
export DBUS_SESSION_BUS_ADDRESS=unix:path=` + runtimeDir + `/bus
export HQ_CODEX_AUTH_HOME=` + home + `/.codex
export HQ_CODEX_AUTH_FILE=` + home + `/.codex/auth.json
export HQ_CODEX_BINARY=/usr/local/lib/hq/current/codex/bin/codex
export HQ_PROJECTS_FILE=` + home + `/.local/state/hq/projects.json
export HQ_SKILLS_ROOT=/usr/local/lib/hq/current/skills
export HQ_TEMPLATES_ROOT=/usr/local/lib/hq/current/templates
export HQ_DATA_ROOT=/srv/hq-data
export HQ_WIKI_ROOT=` + home + `/Desktop/WikiPedik
export PROKECTFILES_ROOT=/srv/projects
export HAPPIER_HQ_SERVER_MODE=1
exec /usr/local/lib/hq/current/hq "$@"
`
	daemon := `[Unit]
Description=HQ Happier owner daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
KillMode=process
WorkingDirectory=` + home + `
ExecStart=/usr/local/lib/hq/current/cli/happier daemon start-sync
Restart=on-failure
RestartSec=5
Environment=HAPPIER_DAEMON_WAIT_FOR_AUTH=1
Environment=HAPPIER_DAEMON_STARTUP_SOURCE=background-service
Environment=HAPPIER_DAEMON_SERVICE_TARGET_MODE=default-following

[Install]
WantedBy=default.target
`
	dropin := `[Service]
KillMode=process
ExecStart=
ExecStart=/usr/local/lib/hq/current/cli/happier daemon start-sync
Environment=HOME=` + home + `
Environment=XDG_RUNTIME_DIR=` + runtimeDir + `
Environment=DBUS_SESSION_BUS_ADDRESS=unix:path=` + runtimeDir + `/bus
Environment=HQ_CODEX_AUTH_HOME=` + home + `/.codex
Environment=HQ_CODEX_AUTH_FILE=` + home + `/.codex/auth.json
Environment=HQ_CODEX_BINARY=/usr/local/lib/hq/current/codex/bin/codex
Environment=HQ_PROJECTS_FILE=` + home + `/.local/state/hq/projects.json
Environment=HQ_SKILLS_ROOT=/usr/local/lib/hq/current/skills
Environment=HQ_TEMPLATES_ROOT=/usr/local/lib/hq/current/templates
Environment=HAPPIER_HQ_SERVER_MODE=1
Environment=HAPPIER_HQ_BINARY=/usr/local/bin/hq
Environment=HQ_DATA_ROOT=/srv/hq-data
Environment=HQ_WIKI_ROOT=` + home + `/Desktop/WikiPedik
Environment=PROKECTFILES_ROOT=/srv/projects
Environment=HAPPIER_SERVER_URL=` + o.PrivateURL + `
Environment=HAPPIER_LOCAL_SERVER_URL=http://127.0.0.1:3005
Environment=HAPPIER_WEBAPP_URL=` + o.PrivateURL + `
`
	relay := `[Unit]
Description=HQ private Happier relay
After=network-online.target

[Service]
Type=simple
User=` + o.User + `
WorkingDirectory=/srv/hq-data
ExecStart=/usr/local/lib/hq/current/relay/happier-server
Restart=on-failure
RestartSec=5
Environment=HAPPIER_SERVER_FLAVOR=light
Environment=HAPPIER_DB_PROVIDER=sqlite
Environment=HAPPIER_FILES_BACKEND=local
Environment=NODE_PATH=/usr/local/lib/hq/current/relay/node_modules
Environment=PRISMA_CLIENT_ENGINE_TYPE=library
Environment=PRISMA_QUERY_ENGINE_LIBRARY=/usr/local/lib/hq/current/relay/node_modules/.prisma/client/` + relayEngine(platform) + `
Environment=HAPPIER_SQLITE_MIGRATIONS_DIR=/usr/local/lib/hq/current/relay/prisma/sqlite/migrations
Environment=HAPPIER_SERVER_LIGHT_DATA_DIR=/srv/hq-data/relay
Environment=HAPPIER_SERVER_HOST=127.0.0.1
Environment=PORT=3005
Environment=HAPPIER_SERVER_UI_DIR=/usr/local/lib/hq/current/ui
Environment=HAPPIER_SERVER_UI_REQUIRED=1
Environment=PUBLIC_URL=` + o.PrivateURL + `

[Install]
WantedBy=multi-user.target
`
	memoryService := `[Unit]
Description=HQ scoped Wikipedia maintenance
After=local-fs.target

[Service]
Type=oneshot
User=` + o.User + `
WorkingDirectory=` + home + `
ExecStart=/usr/local/bin/hq knowledge tick --json
TimeoutStartSec=15min
UMask=0077
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/srv/hq-data ` + home + `/Desktop/WikiPedik
`
	memoryTimer := `[Unit]
Description=Hourly HQ scoped Wikipedia maintenance

[Timer]
OnCalendar=hourly
RandomizedDelaySec=5min
Persistent=true
Unit=hq-memory.service

[Install]
WantedBy=timers.target
`
	return map[string]managedFile{
		"usr/local/bin/hq":   {wrapper, 0755, false},
		"etc/hq/server-mode": {"server\n", 0644, false},
		"home/" + o.User + "/.config/systemd/user/happier-daemon.default.service":                      {daemon, 0644, true},
		"home/" + o.User + "/.config/systemd/user/happier-daemon.default.service.d/90-hq-release.conf": {dropin, 0644, false},
		"etc/systemd/system/happier-server.service":                                                    {relay, 0644, true},
		"etc/systemd/system/hq-memory.service":                                                         {memoryService, 0644, false},
		"etc/systemd/system/hq-memory.timer":                                                           {memoryTimer, 0644, false},
	}
}
func activationSteps(o Options) []string {
	return []string{
		"Existing service units, network bindings, auth state and datasets are preserved; no services were restarted.",
		"Verify the private HTTPS/VPN reverse proxy routes to 127.0.0.1:3005; setup does not change DNS, VPN, firewall or certificates.",
		"Verify rootless Podman, subordinate UID/GID ranges, and the pinned Codex runner image before launching agents.",
		fmt.Sprintf("As %s: /usr/local/lib/hq/current/cli/happier auth login; complete device pairing against %s (existing login is retained).", o.User, o.PrivateURL),
		"Provision Codex credentials separately into the owner-only broker; archive auth is never activated.",
		fmt.Sprintf("After reviewing the unit files: systemctl daemon-reload; loginctl enable-linger %s; as %s run systemctl --user daemon-reload and systemctl --user enable --now happier-daemon.default.service.", o.User, o.User),
		"On a fresh host only: systemctl enable --now happier-server.service. Existing relay activation remains an explicit update step.",
		"After verifying the release: systemctl daemon-reload; systemctl enable --now hq-memory.timer. Inspect systemctl list-timers hq-memory.timer and scoped Wikipedia job status. Curated inbox promotion still requires human review.",
		"Rollback executables by atomically switching /usr/local/lib/hq/current to the recorded previous target, then explicitly restart the affected services. Never roll back datasets with binaries.",
	}
}
