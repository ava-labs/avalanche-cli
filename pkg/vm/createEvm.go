// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
)

type wizardState int64

const (
	startStage wizardState = iota
	descriptorStage
	feeStage
	airdropStage
	precompileStage
	doneStage
	errored
)

type stateDirection int64

const (
	forward stateDirection = iota
	backward
	stop
)

func nextStage(currentState wizardState, direction stateDirection) wizardState {
	switch direction {
	case forward:
		currentState++
	case backward:
		currentState--
	default:
		return errored
	}
	return currentState
}

func CreateEvmGenesis(name string, app *application.Avalanche) ([]byte, *models.Sidecar, error) {
	ux.Logger.PrintToUser("creating subnet %s", name)

	genesis := core.Genesis{}
	conf := params.SubnetEVMDefaultChainConfig

	stage := startStage

	var (
		chainID    *big.Int
		tokenName  string
		allocation core.GenesisAlloc
		direction  stateDirection
		err        error
	)

	for stage != doneStage {
		switch stage {
		case startStage:
			direction = forward
		case descriptorStage:
			chainID, tokenName, direction, err = getDescriptors(app)
		case feeStage:
			*conf, direction, err = getFeeConfig(*conf, app)
		case airdropStage:
			allocation, direction, err = getAllocation(app)
		case precompileStage:
			*conf, direction, err = getPrecompiles(*conf, app)
		default:
			err = errors.New("invalid creation stage")
		}
		if err != nil {
			return []byte{}, nil, err
		}
		stage = nextStage(stage, direction)
	}

	conf.ChainID = chainID

	genesis.Alloc = allocation
	genesis.Config = conf
	genesis.Difficulty = Difficulty
	genesis.GasLimit = GasLimit

	jsonBytes, err := genesis.MarshalJSON()
	if err != nil {
		return []byte{}, nil, err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		return []byte{}, nil, err
	}

	sc := &models.Sidecar{
		Name:      name,
		VM:        models.SubnetEvm,
		Subnet:    name,
		TokenName: tokenName,
	}

	return prettyJSON.Bytes(), sc, nil
}
