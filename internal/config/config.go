package config

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/convert"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs/fdc2"
)

const (
	EnvSourceRPCURL                   = "SOURCE_RPC_URL" // source-chain node (e.g. XRP) — used by PMWMultisigAccountConfigured
	EnvFlareRPCURL                    = "FLARE_RPC_URL"  // Flare C-chain node — used by PMWPaymentStatus/PMWFeeProof (getInitialNonce) and TeeAvailabilityCheck (Relay)
	EnvRelayContractAddress           = "RELAY_CONTRACT_ADDRESS"
	EnvFlareTeeManagerContractAddress = "FLARE_TEE_MANAGER_CONTRACT_ADDRESS"
	EnvTeePaymentsContractAddress     = "TEE_PAYMENTS_CONTRACT_ADDRESS"
	EnvSourceDatabaseURL              = "SOURCE_DATABASE_URL"
	EnvCChainDatabaseURL              = "CCHAIN_DATABASE_URL"
	EnvPort                           = "PORT"
	EnvAPIKeys                        = "API_KEYS"
	EnvSourceID                       = "SOURCE_ID"
	EnvAllowTeeDebug                  = "ALLOW_TEE_DEBUG"               // Needed only for test deployment. Not mandatory to set. Defaults to false.
	EnvDisableAttestationCheckE2E     = "DISABLE_ATTESTATION_CHECK_E2E" // Needed only for e2e test. Not mandatory to set. Defaults to false.
	EnvAllowPrivateNetworks           = "ALLOW_PRIVATE_NETWORKS"        // Test/E2E only. Allows private/loopback IPs while still blocking dangerous IPs. Defaults to false.
	EnvTeeAudience                    = "TEE_AUDIENCE"                  // Optional override for the expected aud claim on Confidential Space attestation tokens. Defaults to DefaultTeeAudience when unset.
	EnvChainID                        = "CHAIN_ID"                      // EVM chain ID this verifier serves; attested TeeInfo.ChainID must match. Required and non-zero.
)

// DefaultTeeAudience is the aud claim the verifier expects on Confidential Space
// attestation tokens when TEE_AUDIENCE is not set. It must match the audience
// tee-node requests its token for, which tee-node currently hardcodes (see
// tee-node/internal/attestation/attestation_token.go). Override via TEE_AUDIENCE
// only if that value diverges.
const DefaultTeeAudience = "https://sts.google.com"

type EnvConfig struct {
	SourceRPCURL                   string
	FlareRPCURL                    string
	RelayContractAddress           string
	FlareTeeManagerContractAddress string
	TeePaymentsContractAddress     string
	SourceDatabaseURL              string
	CChainDatabaseURL              string
	AllowTeeDebug                  string
	DisableAttestationCheckE2E     string
	AllowPrivateNetworks           string
	TeeAudience                    string
	ChainID                        string
	Port                           string
	APIKeys                        []string
	// AttestationType is the single type view used by the per-type config loaders
	// and service constructors. In a multi-type deployment LoadModule sets it per
	// type while iterating AttestationTypes.
	AttestationType fdc2.AttestationType
	// AttestationTypes is the full set of types this deployment serves for its
	// source. Empty means fall back to the single AttestationType.
	AttestationTypes []fdc2.AttestationType
	SourceID         SourceName
}

// ServedAttestationTypes returns the attestation types this deployment serves:
// the explicit AttestationTypes list when set, otherwise the single
// AttestationType, otherwise nil.
func (c EnvConfig) ServedAttestationTypes() []fdc2.AttestationType {
	if len(c.AttestationTypes) > 0 {
		return c.AttestationTypes
	}
	if c.AttestationType != "" {
		return []fdc2.AttestationType{c.AttestationType}
	}
	return nil
}

type SourceName string

const (
	SourceTEE     SourceName = "TEE"
	SourceXRP     SourceName = "XRP"
	SourceTestXRP SourceName = "testXRP"
	SourceBTC     SourceName = "BTC"
	SourceTestBTC SourceName = "testBTC"
)

// SourceAttestationTypes is the canonical set of attestation types each source
// serves. It is the grouping for a per-source deployment: SOURCE_ID selects the
// deployment and, by default, every type listed here for that source is
// registered. Adding a type to a source is a one-line change here.
var SourceAttestationTypes = map[SourceName][]fdc2.AttestationType{
	SourceTEE:     {fdc2.AvailabilityCheck},
	SourceXRP:     {fdc2.PMWMultisigAccountConfigured, fdc2.PMWPaymentStatus, fdc2.PMWFeeProof},
	SourceTestXRP: {fdc2.PMWMultisigAccountConfigured, fdc2.PMWPaymentStatus, fdc2.PMWFeeProof},
	SourceBTC:     {fdc2.PMWMultisigUtxoConfigured},
	SourceTestBTC: {fdc2.PMWMultisigUtxoConfigured},
}

// AttestationTypesForSource returns the attestation types a per-source deployment
// serves for source, and whether the source is known.
func AttestationTypesForSource(source SourceName) ([]fdc2.AttestationType, bool) {
	types, ok := SourceAttestationTypes[source]
	return types, ok
}

type SourceIDEncodedPair struct {
	SourceID        SourceName
	SourceIDEncoded common.Hash
}

type AttestationTypeEncodedPair struct {
	AttestationType        fdc2.AttestationType
	AttestationTypeEncoded common.Hash
}

type ABIArgPair struct {
	Request  abi.Argument
	Response abi.Argument
}

type TeeAvailabilityCheckConfig struct {
	EncodedAndABI
	RelayContractAddress       common.Address
	AllowTeeDebug              bool
	DisableAttestationCheckE2E bool
	AllowPrivateNetworks       bool
	FlareRPCURL                string
	GoogleRootCertificate      *x509.Certificate
	TeeAudience                string
	ChainID                    uint64
}

type PMWPaymentStatusConfig struct {
	EncodedAndABI
	SourceDatabaseURL              string
	CchainDatabaseURL              string
	FlareTeeManagerContractAddress common.Address
	TeePaymentsContractAddress     common.Address
	FlareRPCURL                    string
	ParsedTeeInstructionsABI       abi.ABI
}

type PMWFeeProofConfig struct {
	EncodedAndABI
	SourceDatabaseURL              string
	CchainDatabaseURL              string
	FlareTeeManagerContractAddress common.Address
	TeePaymentsContractAddress     common.Address
	FlareRPCURL                    string
	ParsedTeeInstructionsABI       abi.ABI
}

type PMWMultisigAccountConfig struct {
	EncodedAndABI
	SourceRPCURL string
}

type PMWMultisigUtxoConfig struct {
	EncodedAndABI
	// SourceRPCURL is the Bitcoin node reached for gettxout.
	SourceRPCURL string
}

type EncodedAndABI struct {
	SourceIDPair        SourceIDEncodedPair
	AttestationTypePair AttestationTypeEncodedPair
	ABIPair             ABIArgPair
}

func EncodeAttestationOrSourceName(attestationTypeOrSourceName string) (common.Hash, error) {
	if len(attestationTypeOrSourceName) >= 2 && (attestationTypeOrSourceName[:2] == "0x" || attestationTypeOrSourceName[:2] == "0X") {
		return common.Hash{}, fmt.Errorf("attestation type or source id name must not start with '0x'. Provided: %s", attestationTypeOrSourceName)
	}
	return convert.StringToCommonHash(attestationTypeOrSourceName)
}

var abiStructNames = map[fdc2.AttestationType]struct {
	Request  string
	Response string
}{
	fdc2.AvailabilityCheck: {
		Request:  "availabilityCheckRequestBodyStruct",
		Response: "availabilityCheckResponseBodyStruct",
	},
	fdc2.PMWMultisigAccountConfigured: {
		Request:  "pmwMultisigAccountConfiguredRequestBodyStruct",
		Response: "pmwMultisigAccountConfiguredResponseBodyStruct",
	},
	fdc2.PMWMultisigUtxoConfigured: {
		Request:  "pmwMultisigUtxoConfiguredRequestBodyStruct",
		Response: "pmwMultisigUtxoConfiguredResponseBodyStruct",
	},
	fdc2.PMWPaymentStatus: {
		Request:  "pmwPaymentStatusRequestBodyStruct",
		Response: "pmwPaymentStatusResponseBodyStruct",
	},
	fdc2.PMWFeeProof: {
		Request:  "pmwFeeProofRequestBodyStruct",
		Response: "pmwFeeProofResponseBodyStruct",
	},
}

func LoadEncodedAndABI(envConfig EnvConfig) (EncodedAndABI, error) {
	names, ok := abiStructNames[envConfig.AttestationType]
	if !ok {
		return EncodedAndABI{}, fmt.Errorf("no ABI struct names defined for attestation type %s", envConfig.AttestationType)
	}
	sourceIDEnc, err := EncodeAttestationOrSourceName(string(envConfig.SourceID))
	if err != nil {
		return EncodedAndABI{}, err
	}
	attestationTypeEnc, err := EncodeAttestationOrSourceName(string(envConfig.AttestationType))
	if err != nil {
		return EncodedAndABI{}, err
	}
	requestABI, err := getABIArguments(names.Request)
	if err != nil {
		return EncodedAndABI{}, err
	}
	responseABI, err := getABIArguments(names.Response)
	if err != nil {
		return EncodedAndABI{}, err
	}
	return EncodedAndABI{
		SourceIDPair:        SourceIDEncodedPair{SourceID: envConfig.SourceID, SourceIDEncoded: sourceIDEnc},
		AttestationTypePair: AttestationTypeEncodedPair{AttestationType: envConfig.AttestationType, AttestationTypeEncoded: attestationTypeEnc},
		ABIPair:             ABIArgPair{Request: requestABI, Response: responseABI},
	}, nil
}

func CheckMissingFields(cfg EnvConfig, fields []string) error {
	missing := []string{}
	for _, field := range fields {
		switch field {
		case EnvSourceRPCURL:
			if cfg.SourceRPCURL == "" {
				missing = append(missing, field)
			}
		case EnvFlareRPCURL:
			if cfg.FlareRPCURL == "" {
				missing = append(missing, field)
			}
		case EnvRelayContractAddress:
			if cfg.RelayContractAddress == "" {
				missing = append(missing, field)
			}
		case EnvFlareTeeManagerContractAddress:
			if cfg.FlareTeeManagerContractAddress == "" {
				missing = append(missing, field)
			}
		case EnvTeePaymentsContractAddress:
			if cfg.TeePaymentsContractAddress == "" {
				missing = append(missing, field)
			}
		case EnvSourceDatabaseURL:
			if cfg.SourceDatabaseURL == "" {
				missing = append(missing, field)
			}
		case EnvCChainDatabaseURL:
			if cfg.CChainDatabaseURL == "" {
				missing = append(missing, field)
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

func getABIArguments(structNeeded string) (abi.Argument, error) {
	parsedABI, err := abi.JSON(strings.NewReader(fdc2.Fdc2MetaData.ABI))
	if err != nil {
		return abi.Argument{}, fmt.Errorf("failed to parse ABI: %w", err)
	}

	method, ok := parsedABI.Methods[structNeeded]
	if !ok || len(method.Inputs) != 1 {
		return abi.Argument{}, fmt.Errorf("invalid method definition for %s", structNeeded)
	}

	return method.Inputs[0], nil
}
