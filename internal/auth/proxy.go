package auth

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type proxyKey struct{}

// HTTPSOriginWithoutProxies reports the startup warning condition from the
// trusted-proxy port: TRUSTED_PROXIES is empty and PUBLIC_ORIGIN is https.
// The process still starts. X-Forwarded-For and X-Forwarded-Proto stay ignored
// until a proxy CIDR is set.
func HTTPSOriginWithoutProxies(proxies []string, publicOrigin string) bool {
	origin := strings.TrimSpace(publicOrigin)
	if !strings.HasPrefix(strings.ToLower(origin), "https://") {
		return false
	}
	for _, p := range proxies {
		if strings.TrimSpace(p) != "" {
			return false
		}
	}
	return true
}

// ClientIP is the client address for r.
//
// X-Forwarded-For is honored only when the direct peer is in the trusted
// proxy list installed by Middleware. The address is the rightmost hop that
// is not itself a trusted proxy. An untrusted peer is the client, and a
// forwarded header on that request is ignored.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	nets := proxiesFrom(r.Context())
	peer := peerIP(r)
	if peer == nil {
		return r.RemoteAddr
	}
	if !ipTrusted(nets, peer) {
		return peer.String()
	}
	if ip, ok := rightmostUntrusted(r.Header.Values("X-Forwarded-For"), nets); ok {
		return ip.String()
	}
	return peer.String()
}

// ForwardedProto is the scheme of the request as the immediate proxy saw it.
// X-Forwarded-Proto is read only when the direct peer is trusted, and only
// the rightmost field is used, and only when that field is http or https.
// Otherwise the scheme is the socket's TLS state.
func ForwardedProto(r *http.Request) string {
	if r == nil {
		return "http"
	}
	peer := peerIP(r)
	if peer != nil && ipTrusted(proxiesFrom(r.Context()), peer) {
		if p := rightmostProto(r.Header.Values("X-Forwarded-Proto")); p != "" {
			return p
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func proxiesFrom(ctx context.Context) []net.IPNet {
	if ctx == nil {
		return nil
	}
	nets, _ := ctx.Value(proxyKey{}).([]net.IPNet)
	return nets
}

func peerIP(r *http.Request) net.IP {
	if r == nil || r.RemoteAddr == "" {
		return nil
	}
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	return normalizeIP(host)
}

func rightmostUntrusted(values []string, nets []net.IPNet) (net.IP, bool) {
	var hops []net.IP
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			ip := normalizeIP(part)
			if ip == nil {
				continue
			}
			hops = append(hops, ip)
		}
	}
	if len(hops) == 0 {
		return nil, false
	}
	for i := len(hops) - 1; i >= 0; i-- {
		if !ipTrusted(nets, hops[i]) {
			return hops[i], true
		}
	}
	return hops[0], true
}

func rightmostProto(values []string) string {
	var last string
	found := false
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			last = strings.ToLower(strings.TrimSpace(part))
			found = true
		}
	}
	if !found || (last != "http" && last != "https") {
		return ""
	}
	return last
}

func ipTrusted(nets []net.IPNet, ip net.IP) bool {
	if ip == nil {
		return false
	}
	for i := range nets {
		if nets[i].Contains(ip) {
			return true
		}
	}
	return false
}

func normalizeIP(s string) net.IP {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil
	}
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}

func parseProxies(entries []string) (nets []net.IPNet, invalid []string) {
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		n, ok := parseProxy(entry)
		if !ok {
			invalid = append(invalid, entry)
			continue
		}
		nets = append(nets, n)
	}
	return nets, invalid
}

func parseProxy(s string) (net.IPNet, bool) {
	if !strings.Contains(s, "/") {
		ip := normalizeIP(s)
		if ip == nil {
			return net.IPNet{}, false
		}
		bits := 8 * len(ip)
		return net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}, true
	}
	_, n, err := net.ParseCIDR(s)
	if err != nil || n == nil {
		return net.IPNet{}, false
	}
	if v4 := n.IP.To4(); v4 != nil {
		n.IP = v4
	}
	return *n, true
}
