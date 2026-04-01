package verifier

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fuzzResolver struct {
	ips []net.IPAddr
	err error
}

func (r fuzzResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.ips, nil
}

func FuzzResolveExternalURL(f *testing.F) {
	f.Add("https://example.com", []byte(net.ParseIP("93.184.216.34")), []byte(net.ParseIP("93.184.216.35")), false)
	f.Add("https://localhost", []byte(net.ParseIP("127.0.0.1")), []byte(net.ParseIP("127.0.0.2")), false)
	f.Add("https://proxy.internal", []byte(net.ParseIP("192.168.1.10")), []byte(net.ParseIP("10.0.0.5")), false)
	f.Add("https://evil.example", []byte(net.ParseIP("169.254.169.254")), []byte(net.ParseIP("93.184.216.34")), false)
	f.Add("https://[::1]", []byte(nil), []byte(nil), false)
	f.Add("https://example.com:99999", []byte(net.ParseIP("93.184.216.34")), []byte(nil), false)
	f.Add("https://", []byte(net.ParseIP("93.184.216.34")), []byte(nil), true)
	f.Add("ftp://example.com", []byte(net.ParseIP("93.184.216.34")), []byte(nil), false)
	f.Add("", []byte(nil), []byte(nil), false)

	f.Fuzz(func(t *testing.T, rawURL string, ip1Bytes []byte, ip2Bytes []byte, lookupErr bool) {
		resolver := fuzzResolver{
			ips: []net.IPAddr{{IP: net.IP(ip1Bytes)}, {IP: net.IP(ip2Bytes)}},
		}
		if lookupErr {
			resolver.err = errors.New("dns error")
			resolver.ips = nil
		}

		strictResolved, strictErr := resolveExternalURL(context.Background(), rawURL, resolver, false)
		if strictErr == nil {
			assertResolvedURLInvariant(t, strictResolved, false)
		}

		permissiveResolved, permissiveErr := resolveExternalURL(context.Background(), rawURL, resolver, true)
		if permissiveErr == nil {
			assertResolvedURLInvariant(t, permissiveResolved, true)
		}
	})
}

func assertResolvedURLInvariant(t *testing.T, resolved *ResolvedURL, allowPrivateNetworks bool) {
	t.Helper()
	if resolved == nil {
		t.Fatal("successful URL resolution returned nil result")
	}
	if resolved.Scheme == "" || resolved.Host == "" || resolved.Hostname == "" {
		t.Fatalf("successful URL resolution returned incomplete result: %+v", resolved)
	}
	if resolved.IP == nil {
		t.Fatalf("successful URL resolution returned nil IP: %+v", resolved)
	}
	if allowPrivateNetworks {
		if isDangerousIP(resolved.IP) {
			t.Fatalf("permissive resolution returned dangerous IP %q for %+v", resolved.IP, resolved)
		}
		return
	}
	if isPrivateOrLocalIP(resolved.IP) {
		t.Fatalf("strict resolution returned private/local IP %q for %+v", resolved.IP, resolved)
	}
}
