// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"bytes"
	"encoding/json"

	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
)

func CreateEvmGenesis(name string, log logging.Logger) ([]byte, error) {
	ux.Logger.PrintToUser("creating subnet %s", name)

	genesis := core.Genesis{}
	conf := params.SubnetEVMDefaultChainConfig

	chainId, err := getChainId()
	if err != nil {
		return []byte{}, err
	}
	conf.ChainID = chainId

	*conf, err = getFeeConfig(*conf)
	if err != nil {
		return []byte{}, err
	}

	allocation, err := getAllocation()
	if err != nil {
		return []byte{}, err
	}

	*conf, err = getPrecompiles(*conf)
	if err != nil {
		return []byte{}, err
	}

	genesis.Alloc = allocation
	genesis.Config = conf
	genesis.Difficulty = Difficulty
	genesis.GasLimit = GasLimit

	jsonBytes, err := genesis.MarshalJSON()
	if err != nil {
		return []byte{}, err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		return []byte{}, err
	}

	return prettyJSON.Bytes(), nil
}
