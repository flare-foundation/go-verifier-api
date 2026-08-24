package btcverifier

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/teepaymentsutxo"
	paymentdb "github.com/flare-foundation/go-verifier-api/internal/attestation/pmwpaymentstatus/db"
	"github.com/stretchr/testify/require"
)

type stubBatchCaller struct {
	batchID     uint64
	err         error
	anchorState teepaymentsutxo.ITeePaymentsUtxoUtxoAnchorState
	anchorErr   error
	gotAcct     teepaymentsutxo.ITeePaymentsBasePMWMultisigAccount
	gotID       uint64
	gotAnchorIx *big.Int
}

func (s *stubBatchCaller) GetBatchPaymentId(_ *bind.CallOpts, acct teepaymentsutxo.ITeePaymentsBasePMWMultisigAccount, paymentId uint64) (uint64, error) {
	s.gotAcct = acct
	s.gotID = paymentId
	return s.batchID, s.err
}

func (s *stubBatchCaller) GetAnchor(_ *bind.CallOpts, acct teepaymentsutxo.ITeePaymentsBasePMWMultisigAccount, anchorIndex *big.Int) (teepaymentsutxo.ITeePaymentsUtxoUtxoAnchorState, error) {
	s.gotAcct = acct
	s.gotAnchorIx = anchorIndex
	return s.anchorState, s.anchorErr
}

func TestResolveBatch_OK(t *testing.T) {
	stub := &stubBatchCaller{batchID: 900000}
	r := NewResolver(stub)
	src := common.HexToHash("0xB7C")
	got, err := r.ResolveBatch(context.Background(), src, "bc1qacct", 900007)
	require.NoError(t, err)
	require.Equal(t, uint64(900000), got)
	// account struct is assembled from sourceID + account address.
	require.Equal(t, src, common.Hash(stub.gotAcct.SourceId))
	require.Equal(t, "bc1qacct", stub.gotAcct.AccountAddress)
	require.Equal(t, uint64(900007), stub.gotID)
}

func TestResolveBatch_ZeroPaymentIdRejected(t *testing.T) {
	r := NewResolver(&stubBatchCaller{batchID: 1})
	_, err := r.ResolveBatch(context.Background(), common.Hash{}, "acct", 0)
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestResolveBatch_CallerErrorFailsClosed(t *testing.T) {
	r := NewResolver(&stubBatchCaller{err: errors.New("InvalidPaymentId()")})
	_, err := r.ResolveBatch(context.Background(), common.Hash{}, "acct", 5)
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestResolveAnchorOutpoint_OK(t *testing.T) {
	var txid [32]byte
	for i := range txid {
		txid[i] = 0xab // palindromic, so display-order == internal-order for the assertion
	}
	stub := &stubBatchCaller{anchorState: teepaymentsutxo.ITeePaymentsUtxoUtxoAnchorState{
		GenesisAnchorTxid: txid,
		GenesisAnchorVout: 3,
	}}
	r := NewResolver(stub)
	src := common.HexToHash("0xB7C")
	gotTxid, gotVout, err := r.ResolveAnchorOutpoint(context.Background(), src, "bc1qacct", 5)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("ab", 32), gotTxid)
	require.Equal(t, uint32(3), gotVout)
	require.Equal(t, src, common.Hash(stub.gotAcct.SourceId))
	require.Equal(t, "bc1qacct", stub.gotAcct.AccountAddress)
	require.Equal(t, big.NewInt(5), stub.gotAnchorIx)
}

func TestResolveAnchorOutpoint_CallerErrorFailsClosed(t *testing.T) {
	r := NewResolver(&stubBatchCaller{anchorErr: errors.New("boom")})
	_, _, err := r.ResolveAnchorOutpoint(context.Background(), common.Hash{}, "acct", 0)
	require.ErrorIs(t, err, paymentdb.ErrDatabase)
}

func TestOnChainResolver_CloseNoClient(t *testing.T) {
	// A resolver built from a caller (no ethclient) closes cleanly.
	require.NoError(t, NewResolver(&stubBatchCaller{}).Close())
}
