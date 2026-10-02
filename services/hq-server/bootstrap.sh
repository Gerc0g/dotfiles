#!/usr/bin/env bash
# Provision the Linux prerequisites, then delegate release state to HQ.
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
bundle=''
private_url=''
daemon_user='agent'
dry_run=0
activate=0
while (($#)); do
    case "$1" in
        --bundle|--private-url|--user)
            if (($# < 2)); then echo "Missing value for $1" >&2; exit 2; fi
            case "$1" in
                --bundle) bundle=$2 ;;
                --private-url) private_url=$2 ;;
                --user) daemon_user=$2 ;;
            esac
            shift 2 ;;
        --dry-run) dry_run=1; shift ;;
        --activate) activate=1; shift ;;
        *) echo "Unknown argument: $1" >&2; exit 2 ;;
    esac
done
if [[ -z "$bundle" || -z "$private_url" || ! "$daemon_user" =~ ^[a-z_][a-z0-9_-]*$ ]]; then
    echo 'Usage: bootstrap.sh --bundle /trusted/release --private-url https://private.host [--user agent] [--dry-run] [--activate]' >&2
    exit 2
fi
bundle=$(CDPATH= cd -- "$bundle" && pwd)
if [[ ! -x "$bundle/hq" || ! -f "$bundle/manifest.json" ]]; then
    echo 'A complete trusted Linux release bundle is required.' >&2; exit 2
fi
if ((dry_run)); then
    echo "Plan: provision unprivileged $daemon_user; install distro Podman prerequisites; build Codex 0.157.1 task image; apply verified HQ release."
    echo 'DNS, VPN, firewall, certificates and current datasets are not changed. No credentials are imported.'
    if [[ $(uname -s) == Linux ]] && id "$daemon_user" >/dev/null 2>&1; then
        "$bundle/hq" setup --mode server --bundle "$bundle" --private-url "$private_url" --user "$daemon_user" --dry-run
    else
        echo 'Full file preview requires Linux and the service account; no host state has been changed.'
    fi
    if ((activate)); then echo 'Activation: restart owner daemon, start private loopback relay, enable memory timer.'; fi
    exit 0
fi
if [[ $(uname -s) != Linux || $(id -u) != 0 ]] || ! command -v apt-get >/dev/null 2>&1; then
    echo 'Apply requires a Debian/Ubuntu Linux host and root.' >&2; exit 2
fi
if ! id "$daemon_user" >/dev/null 2>&1; then
    useradd --create-home --shell /bin/bash "$daemon_user"
fi
daemon_uid=$(id -u "$daemon_user")
daemon_home=$(getent passwd "$daemon_user" | cut -d: -f6)
if [[ "$daemon_uid" == 0 || "$daemon_home" != "/home/$daemon_user" ]]; then
    echo 'HQ requires a non-root service account with its own /home/<user> directory.' >&2; exit 2
fi
# Canonical verification happens before installing packages or switching release.
"$bundle/hq" setup --mode server --bundle "$bundle" --private-url "$private_url" --user "$daemon_user" --dry-run
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y podman uidmap slirp4netns fuse-overlayfs ca-certificates python3
if ! awk -F: -v u="$daemon_user" '$1 == u { found=1 } END {exit !found}' /etc/subuid || ! awk -F: -v u="$daemon_user" '$1 == u { found=1 } END {exit !found}' /etc/subgid; then
    echo 'Assign non-overlapping subordinate UID/GID ranges to the service account, then rerun.' >&2; exit 1
fi
loginctl enable-linger "$daemon_user"
systemctl start "user@${daemon_uid}.service"
as_agent() {
    (cd "$daemon_home" && runuser -u "$daemon_user" -- env HOME="$daemon_home" XDG_RUNTIME_DIR="/run/user/$daemon_uid" DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/$daemon_uid/bus" "$@")
}
if ! as_agent podman image exists localhost/hq-codex:0.157.1; then
    build_dir=$(mktemp -d /tmp/hq-image-build.XXXXXX)
    trap 'rm -f "$build_dir/Containerfile"; rmdir "$build_dir"' EXIT
    chmod 755 "$build_dir"
    install -m 0644 "$script_dir/../hq-runner/Containerfile" "$build_dir/Containerfile"
    as_agent podman build --build-arg CODEX_VERSION=0.157.1 --tag localhost/hq-codex:0.157.1 --file "$build_dir/Containerfile" "$build_dir"
fi
codex_version=$(as_agent podman run --rm --network=none localhost/hq-codex:0.157.1 codex --version)
if [[ "$codex_version" != 'codex-cli 0.157.1' ]]; then
    echo 'The existing runner image does not contain the pinned Codex version; release activation refused.' >&2
    exit 1
fi
expected_image=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("codexImage", ""))' "$bundle/manifest.json")
if [[ -n "$expected_image" ]]; then
    image_digests=$(as_agent podman image inspect --format '{{json .RepoDigests}}' localhost/hq-codex:0.157.1)
    if ! python3 -c 'import json,sys; sys.exit(0 if sys.argv[1] in (json.loads(sys.argv[2]) or []) else 1)' "$expected_image" "$image_digests"; then
        echo 'Runner image does not match the release manifest digest; activation refused.' >&2
        exit 1
    fi
fi
"$bundle/hq" setup --mode server --bundle "$bundle" --private-url "$private_url" --user "$daemon_user"
if ((activate)); then
    systemctl daemon-reload
    systemctl enable --now happier-server.service hq-memory.timer
    as_agent systemctl --user daemon-reload
    as_agent systemctl --user enable happier-daemon.default.service
    as_agent systemctl --user restart happier-daemon.default.service
fi
echo 'Release installed. Existing logins are preserved. On a fresh host, pair Happier and log into Codex as the service user; credentials never belong in the bundle.'
