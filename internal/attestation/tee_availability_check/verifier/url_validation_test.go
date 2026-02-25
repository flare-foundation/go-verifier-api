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

func TestValidateExternalURL(t *testing.T) {
	t.Run("allows https hostname resolving to public IP", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "https://example.com", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}},
		})
		require.NoError(t, err)
	})

	t.Run("allows http public IP literal", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "http://8.8.8.8", resolverMock{})
		require.NoError(t, err)
	})

	t.Run("rejects localhost hostname", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "http://localhost:8080", resolverMock{})
		require.ErrorContains(t, err, "local hostnames are not allowed")
	})

	t.Run("rejects loopback IP", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "http://127.0.0.1", resolverMock{})
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects private IP", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "https://10.0.0.12", resolverMock{})
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects unsupported scheme", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "ftp://example.com", resolverMock{})
		require.ErrorContains(t, err, "only http and https are allowed")
	})

	t.Run("rejects URL with userinfo", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "https://user:pass@example.com", resolverMock{})
		require.ErrorContains(t, err, "userinfo is not allowed")
	})

	t.Run("rejects hostname that resolves to private IP", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "https://proxy.example", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("192.168.1.10")}},
		})
		require.ErrorContains(t, err, "resolves to private/local IP")
	})

	t.Run("rejects hostname resolution error", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "https://proxy.example", resolverMock{
			err: errors.New("dns error"),
		})
		require.ErrorContains(t, err, "cannot resolve hostname")
	})

	t.Run("rejects IPv6 loopback", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "http://[::1]", resolverMock{})
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects IPv6 documentation prefix", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "http://[2001:db8::1]", resolverMock{})
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects IPv6 discard prefix", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "http://[100::1]", resolverMock{})
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects IPv6 6to4 prefix", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "http://[2002:c0a8:0101::1]", resolverMock{})
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects IPv6 Teredo prefix", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "http://[2001::1]", resolverMock{})
		require.ErrorContains(t, err, "private/local IPs are not allowed")
	})

	t.Run("rejects hostname resolving to blocked IPv6", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "https://proxy.example", resolverMock{
			ips: []net.IPAddr{{IP: net.ParseIP("2002:c0a8:0101::1")}},
		})
		require.ErrorContains(t, err, "resolves to private/local IP")
	})

	t.Run("allows public IPv6 literal", func(t *testing.T) {
		err := validateExternalURL(context.Background(), "http://[2607:f8b0:4004:800::200e]", resolverMock{})
		require.NoError(t, err)
	})
}

