// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	sdkutils "github.com/ava-labs/avalanche-tooling-sdk-go/utils"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/subnet-evm/ethclient"

	"go.uber.org/zap"
)

func getCClient(apiEndpoint string, blockchainID string) (ethclient.Client, error) {
	cClient, err := ethclient.Dial(fmt.Sprintf("%s/ext/bc/%s/rpc", apiEndpoint, blockchainID))
	if err != nil {
		return cClient, err
	}
	return cClient, nil
}

func ensureHaveBalanceLocalNetwork(which string, addresses []common.Address, blockchainID string) error {
	cClient, err := getCClient(constants.LocalAPIEndpoint, blockchainID)
	if err != nil {
		return err
	}
	for _, address := range addresses {
		// we can break at the first address who has a non-zero balance
		accountBalance, err := getAccountBalance(cClient, address.String())
		if err != nil {
			return err
		}
		if accountBalance > float64(0) {
			return nil
		}
	}
	return fmt.Errorf("at least one of the %s addresses requires a positive token balance", which)
}

func ensureHaveBalance(
	sc *models.Sidecar,
	which string,
	addresses []common.Address,
) error {
	if len(addresses) < 1 {
		return nil
	}
	switch sc.VM {
	case models.SubnetEvm:
		// Currently only checking if admins have balance for subnets deployed in Local Network
		if networkData, ok := sc.Networks["Local Network"]; ok {
			blockchainID := networkData.BlockchainID.String()
			if err := ensureHaveBalanceLocalNetwork(which, addresses, blockchainID); err != nil {
				return err
			}
		}
	default:
		app.Log.Warn("Unsupported VM type", zap.Any("vm-type", sc.VM))
	}
	return nil
}

func getAccountBalance(cClient ethclient.Client, addrStr string) (float64, error) {
	addr := common.HexToAddress(addrStr)
	ctx, cancel := sdkutils.GetAPIContext()
	balance, err := cClient.BalanceAt(ctx, addr, nil)
	defer cancel()
	if err != nil {
		return 0, err
	}
	// convert to nAvax
	balance = balance.Div(balance, big.NewInt(int64(units.Avax)))
	if balance.Cmp(big.NewInt(0)) == 0 {
		return 0, nil
	}
	return float64(balance.Uint64()) / float64(units.Avax), nil
}
