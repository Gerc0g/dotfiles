#!/bin/sh
# tailnet-route-pin.sh — keep Tailscale subnet routes pinned to the live
# tailscaled interface.
#
# Why: a full-tunnel VPN on this Mac (Happ / sing-box) captures the whole
# CGNAT range 100.64.0.0/10 into its own tunnel, so kernel-routed apps (ssh,
# ping) to tailnet IPs get sent through the VPN instead of into Tailscale and
# die ("connection closed" / timeout). Tailscale's own utun interface number
# changes across restarts, so a hand-added route does not survive.
#
# This script finds the utun that currently owns the Tailscale IPv4 address and
# pins the tailnet range to it as two /11s — more specific than the VPN's /10,
# so they win for every tailnet peer. Idempotent; safe to run repeatedly.
#
# Runs as root via the LaunchDaemon com.gerc0g.tailnet-route-pin
# (RunAtLoad + WatchPaths on network changes), so the route self-heals.

PATH=/usr/local/bin:/usr/sbin:/sbin:/usr/bin:/bin
export PATH

TS_IP=$(tailscale ip -4 2>/dev/null | head -1)
[ -z "$TS_IP" ] && exit 0   # Tailscale not up yet — nothing to pin

# utun interface that currently owns the Tailscale IPv4 address
TS_IF=$(ifconfig 2>/dev/null | awk -v ip="$TS_IP" '
  /^utun[0-9]+:/ { ifn = substr($1, 1, length($1) - 1) }
  $1 == "inet" && $2 == ip { print ifn; exit }
')
[ -z "$TS_IF" ] && exit 0

for net in 100.64.0.0/11 100.96.0.0/11; do
  # representative host inside the block to read the current effective route
  probe=$(echo "$net" | sed 's,\.0/11$,.0.1,')
  cur=$(route -n get "$probe" 2>/dev/null | awk '/interface:/ { print $2 }')
  [ "$cur" = "$TS_IF" ] && continue
  route -n delete -net "$net" >/dev/null 2>&1
  if route -n add -net "$net" -interface "$TS_IF" >/dev/null 2>&1; then
    echo "$(date '+%Y-%m-%dT%H:%M:%S') pinned $net -> $TS_IF (was: ${cur:-none})"
  fi
done

# Keep Tailscale from hijacking the system DNS. Its DNS-override makes
# 100.100.100.100 the catch-all resolver and breaks split-DNS from corporate
# VPNs (Pritunl): internal domains like *.nrdsk.team stop resolving. We reach
# the tailnet by IP, so MagicDNS is not needed — force it off whenever it flips
# back on (GUI toggle, reinstall, re-auth).
if tailscale debug prefs 2>/dev/null | grep -q '"CorpDNS": true'; then
  if tailscale set --accept-dns=false 2>/dev/null; then
    echo "$(date '+%Y-%m-%dT%H:%M:%S') disabled Tailscale DNS override (CorpDNS)"
  fi
fi

# Because MagicDNS is intentionally off (above), tailnet *names* don't resolve via
# the system resolver. Pin the dev-server name(s) to their current tailnet IP in
# /etc/hosts so `gerc0g` etc. resolve for every tool (ssh, curl, dev-stack) without
# re-enabling MagicDNS. Self-healing + idempotent, like the routes. The tailscale
# CLI resolves peer IPs directly, so this works with accept-dns=false.
HOST_PINS="gerc0g"
hp_start="# tailnet-host-pin BEGIN"
hp_end="# tailnet-host-pin END"
hp_block=$(for name in $HOST_PINS; do
  nip=$(tailscale ip -4 "$name" 2>/dev/null | head -1)
  [ -n "$nip" ] && printf '%s %s\n' "$nip" "$name"
done)
if [ -n "$hp_block" ]; then
  hp_tmp=$(mktemp 2>/dev/null || echo "/tmp/.hosts.$$")
  sed "/$hp_start/,/$hp_end/d" /etc/hosts > "$hp_tmp" 2>/dev/null
  { printf '%s\n' "$hp_start"; printf '%s\n' "$hp_block"; printf '%s\n' "$hp_end"; } >> "$hp_tmp"
  if ! cmp -s "$hp_tmp" /etc/hosts 2>/dev/null; then
    cp "$hp_tmp" /etc/hosts && echo "$(date '+%Y-%m-%dT%H:%M:%S') host-pin: $hp_block" | tr '\n' ' '
    echo
  fi
  rm -f "$hp_tmp"
fi
