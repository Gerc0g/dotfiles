package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// Proxy is the container's only network path. DNS is checked once and the
// validated numeric address is dialed, preventing DNS-rebinding bypasses.
type Proxy struct {
	Internet     bool
	Lookup       func(context.Context, string) ([]net.IP, error)
	BlockedIPs   []net.IP
	BlockedHosts []string
}

func publicIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "::/96", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001:db8::/32", "2001::/23", "2002::/16", "3fff::/20", "5f00::/16"} {
		if netip.MustParsePrefix(raw).Contains(a) {
			return false
		}
	}
	return true
}
func (p Proxy) target(ctx context.Context, hostport string) (string, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil || port != "443" {
		return "", fmt.Errorf("only HTTPS is allowed")
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, blocked := range p.BlockedHosts {
		if host == blocked {
			return "", fmt.Errorf("configured environment denied")
		}
	}
	if !p.Internet && host != "api.openai.com" && host != "chatgpt.com" && host != "auth.openai.com" {
		return "", fmt.Errorf("internet disabled for this preset")
	}
	lookup := p.Lookup
	if lookup == nil {
		lookup = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	for _, blocked := range p.BlockedHosts {
		if ips, e := lookup(ctx, blocked); e == nil {
			p.BlockedIPs = append(p.BlockedIPs, ips...)
		}
	}
	ips, err := lookup(ctx, host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("host unavailable")
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return "", fmt.Errorf("private or special-use network denied")
		}
		for _, blocked := range p.BlockedIPs {
			if ip.Equal(blocked) {
				return "", fmt.Errorf("control host denied")
			}
		}
	}
	return net.JoinHostPort(ips[0].String(), port), nil
}
func (p Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "HTTPS CONNECT required", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	target, err := p.target(ctx, r.Host)
	if err != nil {
		http.Error(w, "HQ network policy denied destination", http.StatusForbidden)
		return
	}
	upstream, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", target)
	if err != nil {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		upstream.Close()
		http.Error(w, "tunnel unavailable", http.StatusInternalServerError)
		return
	}
	client, buffer, err := hj.Hijack()
	if err != nil {
		upstream.Close()
		return
	}
	// HTTP body/header deadlines must not truncate long model streams after
	// CONNECT. The task lifetime and bounded connection slots own this tunnel.
	_ = client.SetDeadline(time.Time{})
	if _, err = buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		client.Close()
		upstream.Close()
		return
	}
	if err = buffer.Flush(); err != nil {
		client.Close()
		upstream.Close()
		return
	}
	go func() { defer upstream.Close(); defer client.Close(); io.Copy(upstream, buffer) }()
	go func() { defer upstream.Close(); defer client.Close(); io.Copy(client, upstream) }()
}
