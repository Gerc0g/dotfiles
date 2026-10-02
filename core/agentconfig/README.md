# HQ Codex presets

`agent.get` returns desired Work/Research/Ordinary presets and the installed source-skill
catalog. `agent.save` takes `{settings, revision}` and uses an atomic revision
check. Runtime homes are derived for new sessions; active sessions retain their
configuration. The private desired settings file is
`$HQ_DATA_ROOT/settings/agent.json` (0600).

Portable fields include model, reasoning effort, service tier, approval reviewer,
web search, JavaScript REPL preference, network policy, instructions, selected
skills, managed hooks and MCP definitions. MCP supports either a task-local
command with arguments or a public HTTPS URL. These transports cannot be mixed.
No MCP credential, environment or header fields are accepted. HTTPS uses port
443 and the runner proxy still blocks private addresses and company environment
destinations. Saving an endpoint does not prove its reachability.

`agent.previewImport` takes `{preset, toml}` and returns imported/replaced/skipped
items without writing. It imports only explicit supported fields. Public HTTPS
MCP entries without authentication settings are portable; Mac executable paths,
secret environment references, desktop plugins, project trust and local hooks
are not copied. HQ owns sandbox and hook policy. `agent.preview` renders the
desired TOML without persistence. The owner can retain the import report with the
saved settings.

`agent.inspect` accepts `{preset}` for saved settings, or `{preset, settings}`
for a draft. It launches the installed Codex app-server with a temporary profile,
uses a temporary copy of the server auth file, and requests only initialize,
config/read, account/read (without forced token refresh), and model/list. It
never starts a thread or sends a prompt. MCP is disabled during this host-side
diagnostic; `mcpExecutionChecked` is false. Results include config acceptance,
non-secret effective preferences, account presence and model/effort catalogue
availability. A missing catalogue or unselected default model remains unknown.
The temporary profile is removed and its auth is never copied back.

`HQ_CODEX_BINARY` selects the absolute installed Codex executable.
`HQ_CODEX_AUTH_FILE` selects the private server login file; otherwise
`HQ_CODEX_AUTH_HOME/auth.json` or the daemon user's Codex home is used.
Mac authentication is not part of configuration import.

Ordinary has its own preset and runtime home. On reading an older two-preset
settings file, HQ supplies an Ordinary default in memory using Work's model and
reasoning effort, with independent general-purpose instructions, live search,
internet access, no selected skills/MCP, and disabled company-memory hooks.
Reads do not rewrite the file or change its revision. The first save persists
all three presets. Older clients that submit only Work/Research preserve the
current Ordinary preset under the same revision check. Once persisted, Ordinary
settings do not track later changes to Work.

A core downgrade predating Ordinary does not support persisted three-preset
settings. Rolling back only the UI can retain the new core; a full core rollback
requires restoring compatible settings deliberately, not silently dropping the
Ordinary preset.
