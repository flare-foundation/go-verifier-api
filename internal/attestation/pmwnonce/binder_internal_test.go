package pmwnonce

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStoreNonceUpdatesExistingKey covers storeNonce's already-present-key path.
// It is reachable in production only when two goroutines both miss the cache and
// store the same account (initialNonce is immutable, so the store is an
// idempotent refresh); the refresh must update in place and must not append a
// duplicate list element. Exercised directly here because the race is not
// deterministically reproducible through the public API.
func TestStoreNonceUpdatesExistingKey(t *testing.T) {
	b := NewBinder(nil)

	b.storeNonce("k", 1)
	v, ok := b.cachedNonce("k")
	require.True(t, ok)
	require.Equal(t, uint64(1), v)

	// Same key again: must refresh the value and keep a single list element.
	b.storeNonce("k", 2)
	v, ok = b.cachedNonce("k")
	require.True(t, ok)
	require.Equal(t, uint64(2), v)
	require.Equal(t, 1, b.ll.Len(), "refreshing a key must not duplicate its entry")
}
