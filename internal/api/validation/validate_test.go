package validation

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-verifier-api/internal/config"
	"github.com/stretchr/testify/require"
)

type testStructHash struct {
	Hash common.Hash `validate:"hash32"`
}

type testStructWrongHashType struct {
	Hash string `validate:"hash32"`
}

type testStructAddr struct {
	Addr common.Address `validate:"eth_addr"`
}

type testStructWrongAddrType struct {
	Addr string `validate:"eth_addr"`
}

func TestIsHash32(t *testing.T) {
	// Valid hash
	validHash := common.HexToHash("0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	ts := testStructHash{Hash: validHash}
	err := ValidateRequest(ts)
	require.NoError(t, err)

	// Invalid hash
	ts2 := testStructWrongHashType{Hash: "0x"}
	err = ValidateRequest(ts2)
	require.Error(t, err)
}

func TestIsCommonAddress(t *testing.T) {
	// Valid address
	validAddr := common.HexToAddress("0x0123456789abcdef0123456789abcdef01234567")
	ts := testStructAddr{Addr: validAddr}
	err := ValidateRequest(ts)
	require.NoError(t, err)

	// Invalid address
	ts2 := testStructWrongAddrType{Addr: "0x"}
	err = ValidateRequest(ts2)
	require.Error(t, err)
}

func TestValidateSystemAndRequestAttestationNameAndSourceId(t *testing.T) {
	attestationTypePair := config.AttestationTypeEncodedPair{
		AttestationType:        "TestType",
		AttestationTypeEncoded: common.HexToHash("0x1234"),
	}
	sourceIdPair := config.SourceIdEncodedPair{
		SourceId:        "TestSource",
		SourceIdEncoded: common.HexToHash("0x5678"),
	}

	// Matching values
	err := ValidateSystemAndRequestAttestationNameAndSourceId(
		attestationTypePair,
		sourceIdPair,
		attestationTypePair.AttestationTypeEncoded.Hex(),
		sourceIdPair.SourceIdEncoded.Hex(),
	)
	require.NoError(t, err)

	// Mismatched attestation type
	err = ValidateSystemAndRequestAttestationNameAndSourceId(
		attestationTypePair,
		sourceIdPair,
		"0xdeadbeef",
		sourceIdPair.SourceIdEncoded.Hex(),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "attestation type and source id combination not supported")

	// Mismatched source id
	err = ValidateSystemAndRequestAttestationNameAndSourceId(
		attestationTypePair,
		sourceIdPair,
		attestationTypePair.AttestationTypeEncoded.Hex(),
		"0xdeadbeef",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "attestation type and source id combination not supported")
}
