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
	netip.MustParsePrefix("100.64.0.0/10"),  // carrier-grade NAT
	netip.MustParsePrefix("198.18.0.0/15"),  // benchmark testing
	netip.MustParsePrefix("2001:db8::/32"),  // documentation (RFC 3849)
	netip.MustParsePrefix("100::/64"),       // discard prefix (RFC 6666)
	netip.MustParsePrefix("2002::/16"),      // 6to4 (RFC 3056) — can embed private IPv4
	netip.MustParsePrefix("2001::/32"),      // Teredo (RFC 4380)
}

type ipResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

func ValidateExternalURL(ctx context.Context, rawURL string) error {
	return validateExternalURL(ctx, rawURL, net.DefaultResolver)
}

func validateExternalURL(ctx context.Context, rawURL string, resolver ipResolver) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("URL host is required")
	}
	if parsedURL.User != nil {
		return fmt.Errorf("URL userinfo is not allowed")
	}

	host := strings.TrimSuffix(strings.ToLower(parsedURL.Hostname()), ".")
	if host == "" {
		return fmt.Errorf("URL hostname is required")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("local hostnames are not allowed: %s", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("private/local IPs are not allowed: %s", ip.String())
		}
		return nil
	}

	dnsCtx, cancel := context.WithTimeout(ctx, dnsLookupTimeout)
	defer cancel()
	resolvedIPs, err := resolver.LookupIPAddr(dnsCtx, host)
	if err != nil {
		return fmt.Errorf("cannot resolve hostname %q: %w", host, err)
	}
	if len(resolvedIPs) == 0 {
		return fmt.Errorf("hostname %q resolved to no IP addresses", host)
	}
	for _, ipAddr := range resolvedIPs {
		if isPrivateOrLocalIP(ipAddr.IP) {
			return fmt.Errorf("hostname %q resolves to private/local IP %s", host, ipAddr.IP.String())
		}
	}

	return nil
}

func isPrivateOrLocalIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}

	if addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
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
