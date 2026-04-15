package verifier

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const dnsLookupTimeout = 750 * time.Millisecond

// ErrURLValidation is returned (wrapped) when an external URL fails SSRF
// validation (bad scheme, blocked IP, DNS resolution failure, etc.). Callers
// can detect it with errors.Is to classify the failure as a deterministic
// registry/config fault rather than a transient transport error.
var ErrURLValidation = errors.New("URL validation failed")

var blockedIPPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),         // "this network" (RFC 791) — non-routable
	netip.MustParsePrefix("100.64.0.0/10"),     // carrier-grade NAT
	netip.MustParsePrefix("198.18.0.0/15"),     // benchmark testing
	netip.MustParsePrefix("2001:db8::/32"),     // documentation (RFC 3849)
	netip.MustParsePrefix("100::/64"),          // discard prefix (RFC 6666)
	netip.MustParsePrefix("2002::/16"),         // 6to4 (RFC 3056) — can embed private IPv4
	netip.MustParsePrefix("2001::/32"),         // Teredo (RFC 4380)
	netip.MustParsePrefix("64:ff9b::/96"),      // NAT64 well-known prefix (RFC 6052) — maps to IPv4
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
		return nil, fmt.Errorf("%w: invalid URL %q: %w", ErrURLValidation, rawURL, err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("%w: unsupported URL scheme %q: only http and https are allowed", ErrURLValidation, parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("%w: URL host is required", ErrURLValidation)
	}
	if parsedURL.User != nil {
		return nil, fmt.Errorf("%w: URL userinfo is not allowed", ErrURLValidation)
	}

	host := strings.TrimSuffix(strings.ToLower(parsedURL.Hostname()), ".")
	if host == "" {
		return nil, fmt.Errorf("%w: URL hostname is required", ErrURLValidation)
	}
	if !allowPrivateNetworks {
		if host == "localhost" || strings.HasSuffix(host, ".localhost") {
			return nil, fmt.Errorf("%w: local hostnames are not allowed: %s", ErrURLValidation, host)
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
			return nil, fmt.Errorf("%w: "+ipLiteralMsg, ErrURLValidation, ip.String())
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
		return nil, fmt.Errorf("%w: cannot resolve hostname %q: %w", ErrURLValidation, host, err)
	}
	if len(resolvedIPs) == 0 {
		return nil, fmt.Errorf("%w: hostname %q resolved to no IP addresses", ErrURLValidation, host)
	}
	for _, ipAddr := range resolvedIPs {
		if ipCheckFn(ipAddr.IP) {
			return nil, fmt.Errorf("%w: "+ipResolveMsg, ErrURLValidation, host, ipAddr.IP.String())
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
