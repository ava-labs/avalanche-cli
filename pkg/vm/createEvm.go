// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
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

func CreateEvmSubnetConfig(app *application.Avalanche, subnetName string, genesisPath string, subnetEVMVersion string) ([]byte, *models.Sidecar, error) {
	var (
		genesisBytes []byte
		sc           *models.Sidecar
		err          error
	)

	if genesisPath == "" {
		genesisBytes, sc, err = createEvmGenesis(app, subnetName, subnetEVMVersion)
		if err != nil {
			return []byte{}, &models.Sidecar{}, err
		}
	} else {
		ux.Logger.PrintToUser("Importing genesis")
		genesisBytes, err = os.ReadFile(genesisPath)
		if err != nil {
			return []byte{}, &models.Sidecar{}, err
		}

		subnetEVMVersion, err = getVMVersion(app, "Subnet-EVM", constants.SubnetEVMRepoName, subnetEVMVersion)
		if err != nil {
			return []byte{}, &models.Sidecar{}, err
		}

		sc = &models.Sidecar{
			Name:      subnetName,
			VM:        models.SubnetEvm,
			VMVersion: subnetEVMVersion,
			Subnet:    subnetName,
			TokenName: "",
		}
	}

	return genesisBytes, sc, nil
}

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

func createEvmGenesis(app *application.Avalanche, subnetName string, subnetEVMVersion string) ([]byte, *models.Sidecar, error) {
	ux.Logger.PrintToUser("creating subnet %s", subnetName)

	genesis := core.Genesis{}
	conf := params.SubnetEVMDefaultChainConfig

	stage := startStage

	var (
		chainID    *big.Int
		tokenName  string
		vmVersion  string
		allocation core.GenesisAlloc
		direction  stateDirection
		err        error
	)

	for stage != doneStage {
		switch stage {
		case startStage:
			direction = forward
		case descriptorStage:
			chainID, tokenName, vmVersion, direction, err = getDescriptors(app, subnetEVMVersion)
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
		Name:      subnetName,
		VM:        models.SubnetEvm,
		VMVersion: vmVersion,
		Subnet:    subnetName,
		TokenName: tokenName,
	}

	return prettyJSON.Bytes(), sc, nil
}
