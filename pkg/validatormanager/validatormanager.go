// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
)

const (
	PoAValidarorMessengerContractAddress = "0x5F584C2D56B4c356e7d82EC6129349393dc5df17"
	PoSValidarorMessengerContractAddress = "0x5F584C2D56B4c356e7d82EC6129349393dc5df17"
)

//go:embed deployed_poa_validator_manager_bytecode.txt
var deployedPoAValidatorManagerBytecode []byte

func AddPoAValidatorManagerContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedPoaValidatorManagerBytes := common.FromHex(strings.TrimSpace(string(deployedPoAValidatorManagerBytecode)))
	allocs[common.HexToAddress(PoAValidarorMessengerContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedPoaValidatorManagerBytes,
		Nonce:   1,
	}
}

func InitializePoAValidatorManager(
	rpcURL string,
	remoteAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	initialOwner common.Address,
) error {
	pChainBlockchainID := ids.Empty
	churnPeriodSeconds := uint64(0)
	maximumChurnPercentage := uint8(20)
	type Params struct {
		PChainBlockchainID     [32]byte
		SubnetID               [32]byte
		ChurnPeriodSeconds     uint64
		MaximumChurnPercentage uint8
	}
	params := Params{
		PChainBlockchainID:     pChainBlockchainID,
		SubnetID:               subnetID,
		ChurnPeriodSeconds:     churnPeriodSeconds,
		MaximumChurnPercentage: maximumChurnPercentage,
	}
	_, _, err := contract.TxToMethod(
		rpcURL,
		privateKey,
		remoteAddress,
		nil,
		"initialize((bytes32,bytes32,uint64,uint8),address)",
		params,
		initialOwner,
	)
	return err
}
