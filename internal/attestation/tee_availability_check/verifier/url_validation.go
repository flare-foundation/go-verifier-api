package verifier

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const dnsLookupTimeout = 750 * time.Millisecond

var blockedIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),     // carrier-grade NAT
	netip.MustParsePrefix("198.18.0.0/15"),     // benchmark testing
	netip.MustParsePrefix("2001:db8::/32"),     // documentation (RFC 3849)
	netip.MustParsePrefix("100::/64"),          // discard prefix (RFC 6666)
	netip.MustParsePrefix("2002::/16"),         // 6to4 (RFC 3056) — can embed private IPv4
	netip.MustParsePrefix("2001::/32"),         // Teredo (RFC 4380)
	netip.MustParsePrefix("fd00:ec2::254/128"), // AWS EC2 IPv6 metadata
}

type ipResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// ResolvedURL holds the validated URL components and a pinned IP for DNS rebinding prevention.
type ResolvedURL struct {
	Scheme   string
	Host     string
	Hostname string
	Port     string
	IP       net.IP
}

// ResolveExternalURL validates the URL and returns a pinned public IP to prevent DNS rebinding.
// When allowPrivateNetworks is true, private/loopback IPs are permitted but dangerous IPs
// (link-local, metadata, multicast, unspecified, Teredo, 6to4) are still blocked.
func ResolveExternalURL(ctx context.Context, rawURL string, allowPrivateNetworks bool) (*ResolvedURL, error) {
	return resolveExternalURL(ctx, rawURL, net.DefaultResolver, allowPrivateNetworks)
}

// BuildPinnedAddr returns the dial address and headers needed to pin the connection.
func BuildPinnedAddr(resolved *ResolvedURL) (dialAddr, hostHeader, serverName string) {
	port := resolved.Port
	if port == "" {
		if resolved.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return net.JoinHostPort(resolved.IP.String(), port), resolved.Host, resolved.Hostname
}

func resolveExternalURL(ctx context.Context, rawURL string, resolver ipResolver, allowPrivateNetworks bool) (*ResolvedURL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("URL host is required")
	}
	if parsedURL.User != nil {
		return nil, fmt.Errorf("URL userinfo is not allowed")
	}

	host := strings.TrimSuffix(strings.ToLower(parsedURL.Hostname()), ".")
	if host == "" {
		return nil, fmt.Errorf("URL hostname is required")
	}
	if !allowPrivateNetworks {
		if host == "localhost" || strings.HasSuffix(host, ".localhost") {
			return nil, fmt.Errorf("local hostnames are not allowed: %s", host)
		}
	}

	ipCheckFn := isPrivateOrLocalIP
	ipLiteralMsg := "private/local IPs are not allowed: %s"
	ipResolveMsg := "hostname %q resolves to private/local IP %s"
	if allowPrivateNetworks {
		ipCheckFn = isDangerousIP
		ipLiteralMsg = "dangerous IPs are not allowed: %s"
		ipResolveMsg = "hostname %q resolves to dangerous IP %s"
	}

	if ip := net.ParseIP(host); ip != nil {
		if ipCheckFn(ip) {
			return nil, fmt.Errorf(ipLiteralMsg, ip.String())
		}
		return &ResolvedURL{
			Scheme:   parsedURL.Scheme,
			Host:     parsedURL.Host,
			Hostname: parsedURL.Hostname(),
			Port:     parsedURL.Port(),
			IP:       ip,
		}, nil
	}

	dnsCtx, cancel := context.WithTimeout(ctx, dnsLookupTimeout)
	defer cancel()
	resolvedIPs, err := resolver.LookupIPAddr(dnsCtx, host)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve hostname %q: %w", host, err)
	}
	if len(resolvedIPs) == 0 {
		return nil, fmt.Errorf("hostname %q resolved to no IP addresses", host)
	}
	for _, ipAddr := range resolvedIPs {
		if ipCheckFn(ipAddr.IP) {
			return nil, fmt.Errorf(ipResolveMsg, host, ipAddr.IP.String())
		}
	}

	// Pick the first resolved IP to pin the connection.
	return &ResolvedURL{
		Scheme:   parsedURL.Scheme,
		Host:     parsedURL.Host,
		Hostname: parsedURL.Hostname(),
		Port:     parsedURL.Port(),
		IP:       resolvedIPs[0].IP,
	}, nil
}

func isPrivateOrLocalIP(ip net.IP) bool {
	if isDangerousIP(ip) {
		return true
	}
	addr, _ := netip.AddrFromSlice(ip) // already validated by isDangerousIP
	addr = addr.Unmap()
	return addr.IsLoopback() || addr.IsPrivate()
}

// isDangerousIP checks only always-blocked IPs: link-local, multicast, unspecified, and blockedIPPrefixes.
// Unlike isPrivateOrLocalIP, it allows loopback, private (RFC1918), and IPv6 ULA addresses.
func isDangerousIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	// net.ParseIP returns 16-byte IPv4-mapped-IPv6 slices for IPv4 addresses.
	// Unmap normalises these to plain IPv4 so that prefix.Contains and
	// IsUnspecified work correctly against our IPv4 blocked prefixes.
	addr = addr.Unmap()

	if addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return true
	}

	for _, prefix := range blockedIPPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
