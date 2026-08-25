package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/tee/instructions"
)

var (
	pmwPaymentStatusConfig     *PMWPaymentStatusConfig
	pmwPaymentStatusConfigOnce sync.Once
	errPmwPaymentStatusConfig  error
)

func LoadPMWPaymentStatusConfig(envConfig EnvConfig) (*PMWPaymentStatusConfig, error) {
	pmwPaymentStatusConfigOnce.Do(func() {
		pmwPaymentStatusConfig, errPmwPaymentStatusConfig = BuildPMWPaymentStatusConfig(envConfig)
	})
	return pmwPaymentStatusConfig, errPmwPaymentStatusConfig
}

func BuildPMWPaymentStatusConfig(envConfig EnvConfig) (*PMWPaymentStatusConfig, error) {
	switch envConfig.SourceID {
	case SourceBTC, SourceTestBTC:
		return buildBtcPMWPaymentStatusConfig(envConfig)
	default:
		return buildXrpPMWPaymentStatusConfig(envConfig)
	}
}

// buildBtcPMWPaymentStatusConfig builds the BTC node-mode config: the settling
// batch is read from a Bitcoin node (SOURCE_RPC_URL) and per-payment records from
// the channel's PaymentBatched events (CHANNEL_ADDRESS); the verifier-utxo-indexer
// (SOURCE_DATABASE_URL) is not used, so it is not required.
func buildBtcPMWPaymentStatusConfig(envConfig EnvConfig) (*PMWPaymentStatusConfig, error) {
	if err := CheckMissingFields(envConfig, []string{EnvCChainDatabaseURL, EnvTeePaymentsContractAddress, EnvFlareRPCURL, EnvChannelAddress, EnvSourceRPCURL}); err != nil {
		return nil, err
	}
	teePaymentsAddr, err := parseContractAddress(envConfig.TeePaymentsContractAddress, EnvTeePaymentsContractAddress)
	if err != nil {
		return nil, err
	}
	channelAddr, err := parseContractAddress(envConfig.ChannelAddress, EnvChannelAddress)
	if err != nil {
		return nil, err
	}
	// Fail fast on an unresolvable network rather than encoding addresses no chain
	// serves (BtcNetworkParams is also called at verifier construction).
	if _, err := BtcNetworkParams(envConfig.SourceID, envConfig.BtcNetwork); err != nil {
		return nil, err
	}
	commonConfig, err := LoadEncodedAndABI(envConfig)
	if err != nil {
		return nil, err
	}
	// The confirmation-depth floor is intentionally NOT configurable here: it is a
	// consensus parameter (a fixed constant in the verifier), because a per-DP
	// value would split attestation agreement on borderline-depth batches.
	return &PMWPaymentStatusConfig{
		EncodedAndABI:              commonConfig,
		CchainDatabaseURL:          envConfig.CChainDatabaseURL,
		TeePaymentsContractAddress: teePaymentsAddr,
		FlareRPCURL:                envConfig.FlareRPCURL,
		ChannelAddress:             channelAddr,
		SourceRPCURL:               envConfig.SourceRPCURL,
		BtcNetwork:                 envConfig.BtcNetwork,
	}, nil
}

func buildXrpPMWPaymentStatusConfig(envConfig EnvConfig) (*PMWPaymentStatusConfig, error) {
	err := CheckMissingFields(envConfig, []string{EnvCChainDatabaseURL, EnvSourceDatabaseURL, EnvFlareTeeManagerContractAddress, EnvTeePaymentsContractAddress, EnvFlareRPCURL})
	if err != nil {
		return nil, err
	}
	flareTeeManagerAddr, err := parseContractAddress(envConfig.FlareTeeManagerContractAddress, EnvFlareTeeManagerContractAddress)
	if err != nil {
		return nil, err
	}
	teePaymentsAddr, err := parseContractAddress(envConfig.TeePaymentsContractAddress, EnvTeePaymentsContractAddress)
	if err != nil {
		return nil, err
	}
	commonConfig, err := LoadEncodedAndABI(envConfig)
	if err != nil {
		return nil, err
	}
	parsedTeeInstructionsABI, err := abi.JSON(strings.NewReader(instructions.InstructionsMetaData.ABI))
	if err != nil {
		return nil, fmt.Errorf("cannot parse TeeInstructions ABI: %w", err)
	}
	return &PMWPaymentStatusConfig{
		EncodedAndABI:                  commonConfig,
		SourceDatabaseURL:              envConfig.SourceDatabaseURL,
		CchainDatabaseURL:              envConfig.CChainDatabaseURL,
		FlareTeeManagerContractAddress: flareTeeManagerAddr,
		TeePaymentsContractAddress:     teePaymentsAddr,
		FlareRPCURL:                    envConfig.FlareRPCURL,
		ParsedTeeInstructionsABI:       parsedTeeInstructionsABI,
	}, nil
}

// parseContractAddress validates a 0x-prefixed hex address and rejects zero/malformed values.
func parseContractAddress(raw, envName string) (common.Address, error) {
	if !common.IsHexAddress(raw) {
		return common.Address{}, fmt.Errorf("%s is not a valid hex address: %q", envName, raw)
	}
	addr := common.HexToAddress(raw)
	if addr == (common.Address{}) {
		return common.Address{}, fmt.Errorf("%s must not be the zero address", envName)
	}
	return addr, nil
}

func ClearPMWPaymentStatusConfigForTest() {
	pmwPaymentStatusConfig = nil
	pmwPaymentStatusConfigOnce = sync.Once{}
	errPmwPaymentStatusConfig = nil
}
