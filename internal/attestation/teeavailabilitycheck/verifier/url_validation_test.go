package verifier

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

type resolverMock struct {
	ips []net.IPAddr
	err error
}

func (r resolverMock) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.ips, nil
}

func TestResolveExternalURLValidation(t *testing.T) {
	t.Run("allows https hostname resolving to public IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://example.com", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}},
		}, false)
		require.NoError(t, err)
	})

	t.Run("allows http public IP literal", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://8.8.8.8", resolverMock{}, false)
		require.NoError(t, err)
	})

	t.Run("rejects localhost hostname", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://localhost:8080", resolverMock{}, false)
		require.ErrorContains(t, err, "local hostnames are not allowed")
	})

	t.Run("rejects loopback IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://127.0.0.1", resolverMock{}, false)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects private IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://10.0.0.12", resolverMock{}, false)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects unsupported scheme", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "ftp://example.com", resolverMock{}, false)
		require.ErrorContains(t, err, "only http and https are allowed")
	})

	t.Run("rejects URL with userinfo", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://user:pass@example.com", resolverMock{}, false)
		require.ErrorContains(t, err, "userinfo is not allowed")
	})

	t.Run("rejects hostname that resolves to private IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://proxy.example", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("192.168.1.10")}},
		}, false)
		require.ErrorContains(t, err, "resolves to private/local IP")
	})

	t.Run("rejects hostname resolution error", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://proxy.example", resolverMock{
			err: errors.New("dns error"),
		}, false)
		require.ErrorContains(t, err, "cannot resolve hostname")
	})

	t.Run("rejects IPv6 loopback", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[::1]", resolverMock{}, false)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects IPv6 documentation prefix", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[2001:db8::1]", resolverMock{}, false)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects IPv6 discard prefix", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[100::1]", resolverMock{}, false)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects IPv6 6to4 prefix", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[2002:c0a8:0101::1]", resolverMock{}, false)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects IPv6 Teredo prefix", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[2001::1]", resolverMock{}, false)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects hostname resolving to blocked IPv6", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://proxy.example", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("2002:c0a8:0101::1")}},
		}, false)
		require.ErrorContains(t, err, "resolves to private/local IP")
	})

	t.Run("allows public IPv6 literal", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[2607:f8b0:4004:800::200e]", resolverMock{}, false)
		require.NoError(t, err)
	})

	t.Run("rejects hostname resolving to carrier-grade NAT IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://proxy.example", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("100.64.0.1")}},
		}, false)
		require.ErrorContains(t, err, "resolves to private/local IP")
	})

	t.Run("rejects hostname resolving to benchmark testing IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://proxy.example", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("198.18.0.1")}},
		}, false)
		require.ErrorContains(t, err, "resolves to private/local IP")
	})

	t.Run("rejects link-local unicast IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://169.254.1.1", resolverMock{}, false)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects multicast IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://224.0.0.1", resolverMock{}, false)
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects hostname resolving to unspecified IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://proxy.example", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("0.0.0.0")}},
		}, false)
		require.ErrorContains(t, err, "resolves to private/local IP")
	})

	t.Run("rejects empty host", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://", resolverMock{}, false)
		require.ErrorContains(t, err, "URL host is required")
	})

	t.Run("rejects hostname resolving to no IPs", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://proxy.example", resolverMock{
			ips: []net.IPAddr{},
		}, false)
		require.ErrorContains(t, err, "resolved to no IP addresses")
	})
}

func TestResolveExternalURL(t *testing.T) {
	t.Run("returns pinned IP for hostname", func(t *testing.T) {
		resolved, err := resolveExternalURL(context.Background(), "https://example.com", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}},
		}, false)
		require.NoError(t, err)
		require.Equal(t, "https", resolved.Scheme)
		require.Equal(t, "example.com", resolved.Hostname)
		require.Equal(t, "", resolved.Port)
		require.Equal(t, net.ParseIP("93.184.216.34"), resolved.IP)
	})

	t.Run("returns pinned IP for literal", func(t *testing.T) {
		resolved, err := resolveExternalURL(context.Background(), "http://8.8.8.8:8080", resolverMock{}, false)
		require.NoError(t, err)
		require.Equal(t, "http", resolved.Scheme)
		require.Equal(t, "8.8.8.8", resolved.Hostname)
		require.Equal(t, "8080", resolved.Port)
		require.Equal(t, net.ParseIP("8.8.8.8"), resolved.IP)
	})
}

func TestResolveExternalURLAllowPrivateNetworks(t *testing.T) {
	// Allowed: loopback, private, ULA, public
	t.Run("allows localhost hostname", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://localhost:8080", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}},
		}, true)
		require.NoError(t, err)
	})

	t.Run("allows .localhost hostname", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://proxy.localhost:8080", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}},
		}, true)
		require.NoError(t, err)
	})

	t.Run("allows loopback IP 127.0.0.1", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://127.0.0.1", resolverMock{}, true)
		require.NoError(t, err)
	})

	t.Run("allows IPv6 loopback ::1", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[::1]", resolverMock{}, true)
		require.NoError(t, err)
	})

	t.Run("allows Docker bridge IP 172.17.0.1", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://172.17.0.1:8080", resolverMock{}, true)
		require.NoError(t, err)
	})

	t.Run("allows 10.x private IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://10.0.0.5", resolverMock{}, true)
		require.NoError(t, err)
	})

	t.Run("allows 192.168.x private IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://192.168.1.10", resolverMock{}, true)
		require.NoError(t, err)
	})

	t.Run("allows IPv6 ULA fc00::/7", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[fd12:3456:789a::1]", resolverMock{}, true)
		require.NoError(t, err)
	})

	t.Run("still blocks AWS EC2 IPv6 metadata", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[fd00:ec2::254]", resolverMock{}, true)
		require.ErrorContains(t, err, "dangerous IPs are not allowed")
	})

	t.Run("allows hostname resolving to private IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://proxy.internal", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("192.168.1.10")}},
		}, true)
		require.NoError(t, err)
	})

	t.Run("allows public IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://8.8.8.8", resolverMock{}, true)
		require.NoError(t, err)
	})

	// Still blocked: dangerous IPs
	t.Run("still blocks metadata IP 169.254.169.254", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://169.254.169.254", resolverMock{}, true)
		require.ErrorContains(t, err, "dangerous IPs are not allowed")
	})

	t.Run("still blocks 0.0.0.0", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://0.0.0.0", resolverMock{}, true)
		require.ErrorContains(t, err, "dangerous IPs are not allowed")
	})

	t.Run("still blocks multicast IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://224.0.0.1", resolverMock{}, true)
		require.ErrorContains(t, err, "dangerous IPs are not allowed")
	})

	t.Run("still blocks carrier-grade NAT", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://100.64.0.1", resolverMock{}, true)
		require.ErrorContains(t, err, "dangerous IPs are not allowed")
	})

	t.Run("still blocks Teredo", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[2001::1]", resolverMock{}, true)
		require.ErrorContains(t, err, "dangerous IPs are not allowed")
	})

	t.Run("still blocks 6to4", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[2002:c0a8:0101::1]", resolverMock{}, true)
		require.ErrorContains(t, err, "dangerous IPs are not allowed")
	})

	t.Run("still blocks fe80:: link-local", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "http://[fe80::1]", resolverMock{}, true)
		require.ErrorContains(t, err, "dangerous IPs are not allowed")
	})

	t.Run("still blocks hostname resolving to dangerous IP", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://evil.example", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}},
		}, true)
		require.ErrorContains(t, err, "resolves to dangerous IP")
	})

	// Still validates scheme and userinfo
	t.Run("still rejects unsupported scheme", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "ftp://localhost", resolverMock{}, true)
		require.ErrorContains(t, err, "only http and https are allowed")
	})

	t.Run("still rejects URL with userinfo", func(t *testing.T) {
		_, err := resolveExternalURL(context.Background(), "https://user:pass@localhost", resolverMock{}, true)
		require.ErrorContains(t, err, "userinfo is not allowed")
	})
}

func TestIsDangerousIP(t *testing.T) {
	t.Run("invalid IP slice is treated as dangerous", func(t *testing.T) {
		require.True(t, isDangerousIP(nil))
		require.True(t, isDangerousIP(net.IP{}))
	})
	t.Run("0.0.0.0/8 is blocked", func(t *testing.T) {
		require.True(t, isDangerousIP(net.ParseIP("0.0.0.0")))
		require.True(t, isDangerousIP(net.ParseIP("0.0.0.1")))
		require.True(t, isDangerousIP(net.ParseIP("0.255.255.255")))
	})
	t.Run("NAT64 well-known prefix is blocked", func(t *testing.T) {
		require.True(t, isDangerousIP(net.ParseIP("64:ff9b::1")))
		require.True(t, isDangerousIP(net.ParseIP("64:ff9b::ffff:ffff")))
	})
}

func TestBuildPinnedAddr(t *testing.T) {
	t.Run("defaults https port and preserves host header", func(t *testing.T) {
		resolved := &ResolvedURL{
			Scheme:   "https",
			Host:     "example.com",
			Hostname: "example.com",
			Port:     "",
			IP:       net.ParseIP("93.184.216.34"),
		}
		dialAddr, hostHeader, serverName := BuildPinnedAddr(resolved)
		require.Equal(t, "93.184.216.34:443", dialAddr)
		require.Equal(t, "example.com", hostHeader)
		require.Equal(t, "example.com", serverName)
	})

	t.Run("defaults http port", func(t *testing.T) {
		resolved := &ResolvedURL{
			Scheme:   "http",
			Host:     "example.com",
			Hostname: "example.com",
			Port:     "",
			IP:       net.ParseIP("93.184.216.34"),
		}
		dialAddr, hostHeader, serverName := BuildPinnedAddr(resolved)
		require.Equal(t, "93.184.216.34:80", dialAddr)
		require.Equal(t, "example.com", hostHeader)
		require.Equal(t, "example.com", serverName)
	})

	t.Run("non-default port preserved in dial and host header", func(t *testing.T) {
		resolved := &ResolvedURL{
			Scheme:   "http",
			Host:     "example.com:8080",
			Hostname: "example.com",
			Port:     "8080",
			IP:       net.ParseIP("93.184.216.34"),
		}
		dialAddr, hostHeader, serverName := BuildPinnedAddr(resolved)
		require.Equal(t, "93.184.216.34:8080", dialAddr)
		require.Equal(t, "example.com:8080", hostHeader)
		require.Equal(t, "example.com", serverName)
	})
}
