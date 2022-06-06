// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
)

func CreateEvmGenesis(name string, log logging.Logger) ([]byte, string, error) {
	ux.Logger.PrintToUser("creating subnet %s", name)

	genesis := core.Genesis{}
	conf := params.SubnetEVMDefaultChainConfig

	stage := startStage

	var chainId *big.Int
	var tokenName string
	var allocation core.GenesisAlloc
	var err error

	for stage != doneStage {
		switch stage {
		case startStage:
			stage = descriptorStage
		case descriptorStage:
			chainId, tokenName, stage, err = getDescriptors()
			fmt.Println("Creating token", tokenName)
		case feeStage:
			*conf, stage, err = getFeeConfig(*conf)
		case airdropStage:
			allocation, stage, err = getAllocation()
		case precompileStage:
			*conf, stage, err = getPrecompiles(*conf)
		}
		if err != nil {
			return []byte{}, "", err
		}
	}

	conf.ChainID = chainId

	genesis.Alloc = allocation
	genesis.Config = conf
	genesis.Difficulty = Difficulty
	genesis.GasLimit = GasLimit

	jsonBytes, err := genesis.MarshalJSON()
	if err != nil {
		return []byte{}, "", err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		return []byte{}, "", err
	}

	return prettyJSON.Bytes(), tokenName, nil
}
