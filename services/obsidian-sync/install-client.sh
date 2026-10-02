#!/bin/sh
# Install a private, pinned runtime without changing the system Node or npm.
set -eu
umask 077

test "$(uname -s)" = Linux && test "$(uname -m)" = x86_64 || {
    echo 'This installer supports Linux x86_64 only.' >&2
    exit 1
}

runtime="$HOME/.local/lib/obsidian-headless"
node_version=22.23.3
client_version=0.0.14
archive="node-v${node_version}-linux-x64.tar.xz"
mkdir -p "$runtime" "$HOME/.local/bin" "$HOME/.config/obsidian-headless"
cd "$runtime"
if [ ! -x "node-v${node_version}-linux-x64/bin/node" ]; then
    curl -fsSLO "https://nodejs.org/dist/v${node_version}/$archive"
    printf '%s  %s\n' df450af89261115ef9f9e3830c3eeb2cc9213b63c720b1af623cb5dcbe2e02de "$archive" | sha256sum -c -
    tar -xf "$archive"
fi
PATH="$runtime/node-v${node_version}-linux-x64/bin:$PATH"
export PATH
npm install --prefix "$runtime/client" --save-exact --omit=dev --no-audit --no-fund "obsidian-headless@$client_version"
cat > "$HOME/.local/bin/ob" <<'EOF'
#!/bin/sh
exec "$HOME/.local/lib/obsidian-headless/node-v22.23.3-linux-x64/bin/node" "$HOME/.local/lib/obsidian-headless/client/node_modules/obsidian-headless/cli.js" "$@"
EOF
chmod 700 "$HOME/.local/bin/ob"
"$HOME/.local/bin/ob" --version
