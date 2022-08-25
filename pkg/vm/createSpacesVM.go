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
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/spacesvm/chain"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
)

func prueba() {
	allocs := []*chain.CustomAllocation{
		{
			Address: common.HexToAddress("0xF9370fa73846393798C2d23aa2a4aBA7489d9810"),
			Balance: 10000000,
		},
		{
			Address: common.HexToAddress("0x8Db3219F3f59b504BCF132EfB4B87Bf08c771d83"),
			Balance: 10000000,
		},
		{
			Address: common.HexToAddress("0x162a5fadfdd769f9a665701348FbeEd12A4FFce7"),
			Balance: 10000000,
		},
		{
			Address: common.HexToAddress("0x69fd199Aca8250d520F825d22F4ad9db4A58E9D9"),
			Balance: 10000000,
		},
		{
			Address: common.HexToAddress("0x454474642C32b19E370d9A55c20431d85833cDD6"),
			Balance: 10000000,
		},
		{
			Address: common.HexToAddress("0xeB4Fc761FAb7501abe8cD04b2d831a45E8913DdF"),
			Balance: 10000000,
		},
		{
			Address: common.HexToAddress("0xD23cbfA7eA985213aD81223309f588A7E66A246A"),
			Balance: 10000000,
		},
	}
	genesis := chain.DefaultGenesis()
	genesis.CustomAllocation = allocs
}

func CreateSpacesVMSubnetConfig(app *application.Avalanche, subnetName string, genesisPath string, subnetEVMVersion string) ([]byte, *models.Sidecar, error) {
	var (
		genesisBytes []byte
		sc           *models.Sidecar
		err          error
	)

	if genesisPath == "" {
		genesisBytes, sc, err = createSpacesVMGenesis(app, subnetName, subnetEVMVersion)
		if err != nil {
			return []byte{}, &models.Sidecar{}, err
		}
	} else {
		ux.Logger.PrintToUser("Importing genesis")
		genesisBytes, err = os.ReadFile(genesisPath)
		if err != nil {
			return []byte{}, &models.Sidecar{}, err
		}

		if subnetEVMVersion == "latest" {
			subnetEVMVersion, err = binutils.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
				constants.AvaLabsOrg,
				constants.SubnetEVMRepoName,
			))
			if err != nil {
				return []byte{}, &models.Sidecar{}, err
			}
		} else if subnetEVMVersion == "" {
			subnetEVMVersion, err = getSubnetEVMVersion(app)
			if err != nil {
				return []byte{}, &models.Sidecar{}, err
			}
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

func createSpacesVMGenesis(app *application.Avalanche, subnetName string, subnetEVMVersion string) ([]byte, *models.Sidecar, error) {
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
