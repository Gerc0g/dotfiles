#!/usr/bin/env bash
# Re-announce currently running dotfiles agent wrapper processes to Masko Code.
# Useful after Masko restart or after upgrading masko-agent-wrap.sh while agents
# are already running.

set -euo pipefail

python3 - <<'PY'
import json
import re
import subprocess
import urllib.request

ps = subprocess.check_output(["ps", "-axo", "pid,ppid,args"], text=True)
procs = {}
for line in ps.splitlines()[1:]:
    parts = line.strip().split(None, 2)
    if len(parts) < 3:
        continue
    pid = int(parts[0])
    ppid = int(parts[1])
    args = parts[2]
    procs[pid] = (ppid, args)

attach_by_session = {}
for pid, (_, args) in procs.items():
    match = re.search(r"\btmux attach -t ([^\s]+)", args)
    if match:
        attach_by_session.setdefault(match.group(1), pid)

count = 0
marker = "bash /Users/_gerc0g/dotfiles/scripts/masko-agent-wrap.sh "
for pid, (ppid, args) in sorted(procs.items()):
    if marker not in args:
        continue
    tail = args.split(marker, 1)[1]
    try:
        before_cmd = tail.split(" -- ", 1)[0]
        source, repo_dir, launch_session = before_cmd.split(None, 2)
    except ValueError:
        continue

    session_id = f"dotfiles-{source}-{launch_session}-{pid}"
    payload = {
        "hook_event_name": "SessionStart",
        "session_id": session_id,
        "source": source,
        "cwd": repo_dir,
        "project_dir": repo_dir,
        "launch_session": launch_session,
        "process_id": pid,
        "shell_pid": ppid,
    }
    if launch_session in attach_by_session:
        payload["terminal_pid"] = attach_by_session[launch_session]

    req = urllib.request.Request(
        "http://127.0.0.1:45832/hook",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        urllib.request.urlopen(req, timeout=1.0).read()
    except Exception as exc:
        print(f"failed {session_id}: {exc}")
        continue
    count += 1
    print(f"resynced {session_id} {repo_dir}")

print(f"total {count}")
PY
