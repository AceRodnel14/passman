package main

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// parseCIDRs parses a comma-separated list of CIDRs. A bare IP means /32 or
// /128. An empty string gives an empty list.
func parseCIDRs(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q", part)
			}
			out = append(out, unmapPrefix(p.Masked()))
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("invalid IP or CIDR %q", part)
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

// unmapPrefix turns ::ffff:a.b.c.d/N into a.b.c.d/(N-96) so IPv4 rules match
// IPv4 clients regardless of how the socket reports them.
func unmapPrefix(p netip.Prefix) netip.Prefix {
	if p.Addr().Is4In6() && p.Bits() >= 96 {
		return netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
	}
	return p
}

func inPrefixes(a netip.Addr, list []netip.Prefix) bool {
	a = a.Unmap().WithZone("")
	for _, p := range list {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// forwardingHeaders mark a request that was relayed by a proxy.
var forwardingHeaders = []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-Ip", "Forwarded"}

// viaProxy reports whether the request carries any forwarding header.
func viaProxy(r *http.Request) bool {
	for _, h := range forwardingHeaders {
		if len(r.Header.Values(h)) > 0 {
			return true
		}
	}
	return false
}

// peerIP is the address of the direct TCP peer. Forwarding headers are never
// consulted. The result is the zero Addr if RemoteAddr is unparsable.
func peerIP(r *http.Request) netip.Addr {
	s := strings.TrimSpace(r.RemoteAddr)
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap().WithZone("")
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return a.Unmap().WithZone("")
	}
	return netip.Addr{}
}
