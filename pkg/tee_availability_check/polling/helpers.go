package polling

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/flare-foundation/go-flare-common/pkg/contracts/teeregistry"
	"gitlab.com/urskak/verifier-api/pkg/tee_availability_check/config"
)

type ActiveTees struct {
	TeeIds []common.Address
	Urls   []string
}

func GetActiveTees(client *ethclient.Client) (ActiveTees, error) {
	contractAddrStr, err := config.TeeRegistryContractAddress()
	if err != nil {
		return ActiveTees{}, err
	}
	contractAddress := common.HexToAddress(contractAddrStr)
	teeregistryCaller, err := teeregistry.NewTeeRegistryCaller(contractAddress, client)
	if err != nil {
		return ActiveTees{}, fmt.Errorf("failed to create contract caller: %w", err)
	}
	callOpts := &bind.CallOpts{
		Context: context.Background(),
	}
	activeTees, err := teeregistryCaller.GetActiveTees(callOpts)
	if err != nil {
		return ActiveTees{}, fmt.Errorf("failed to call GetActiveTeeIds: %w", err)
	}
	return activeTees, nil
}
