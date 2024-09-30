// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
)

const (
	ValidatorContractAddress = "0x5F584C2D56B4c356e7d82EC6129349393dc5df17"
)

//go:embed deployed_poa_validator_manager_bytecode.txt
var deployedPoAValidatorManagerBytecode []byte

func AddPoAValidatorManagerContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedPoaValidatorManagerBytes := common.FromHex(strings.TrimSpace(string(deployedPoAValidatorManagerBytecode)))
	allocs[common.HexToAddress(ValidatorContractAddress)] = core.GenesisAccount{
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
	ownerAddress common.Address,
) error {
	type Params struct {
		SubnetID               [32]byte
		ChurnPeriodSeconds     uint64
		MaximumChurnPercentage uint8
	}
	churnPeriodSeconds := uint64(0)
	maximumChurnPercentage := uint8(20)
	params := Params{
		SubnetID:               subnetID,
		ChurnPeriodSeconds:     churnPeriodSeconds,
		MaximumChurnPercentage: maximumChurnPercentage,
	}
	_, _, err := contract.TxToMethod(
		rpcURL,
		privateKey,
		remoteAddress,
		nil,
		"initialize((bytes32,uint64,uint8),address)",
		params,
		ownerAddress,
	)
	return err
}

func SetupPoA(
	app *application.Avalanche,
	network models.Network,
	blockchainName string,
) error {
	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}
	rpcURL, _, err := contract.GetBlockchainEndpoints(
		app,
		network,
		chainSpec,
		true,
		false,
	)
	if err != nil {
		return err
	}
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	sc, err := app.LoadSidecar(chainSpec.BlockchainName)
	if err != nil {
		return err
	}
	ownerAddress := common.HexToAddress(sc.PoAValidatorManagerOwner)
	_, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(app, network, chainSpec)
	if err != nil {
		return err
	}
	_ = InitializePoAValidatorManager(
		rpcURL,
		common.HexToAddress(ValidatorContractAddress),
		genesisPrivateKey,
		subnetID,
		ownerAddress,
	)
	return nil
}
