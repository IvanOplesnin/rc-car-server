package access

import (
	"fmt"
	"net/netip"
	"strings"
)

// ClientIPResolver accepts a forwarded client address only when the direct
// peer is explicitly trusted. It uses the right-most X-Forwarded-For value,
// which is the address appended by the nearest reverse proxy.
type ClientIPResolver struct {
	trustedProxies []netip.Prefix
}

func NewClientIPResolver(values []string) (*ClientIPResolver, error) {
	resolver := &ClientIPResolver{
		trustedProxies: make([]netip.Prefix, 0, len(values)),
	}

	for _, value := range values {
		prefix, err := parseIPOrPrefix(strings.TrimSpace(value))
		if err != nil {
			return nil, err
		}

		resolver.trustedProxies = append(resolver.trustedProxies, prefix)
	}

	return resolver, nil
}

func (r *ClientIPResolver) Resolve(remoteAddr, forwardedFor string) string {
	peer := parseAddr(ExtractIP(remoteAddr))
	if !peer.IsValid() || !r.isTrusted(peer) {
		return remoteAddr
	}

	values := strings.Split(forwardedFor, ",")
	for i := len(values) - 1; i >= 0; i-- {
		client := parseAddr(strings.TrimSpace(values[i]))
		if client.IsValid() {
			return client.String()
		}
	}

	return remoteAddr
}

func (r *ClientIPResolver) isTrusted(addr netip.Addr) bool {
	addr = addr.Unmap()

	for _, prefix := range r.trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

func parseIPOrPrefix(value string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return normalizePrefix(prefix), nil
	}

	addr := parseAddr(value)
	if !addr.IsValid() {
		return netip.Prefix{}, fmt.Errorf("invalid trusted proxy IP address or prefix %q", value)
	}

	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func normalizePrefix(prefix netip.Prefix) netip.Prefix {
	addr := prefix.Addr().Unmap()
	bits := prefix.Bits()

	if prefix.Addr().Is4In6() {
		bits -= 96
	}

	return netip.PrefixFrom(addr, bits).Masked()
}

func parseAddr(value string) netip.Addr {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}
	}

	return addr.Unmap()
}
