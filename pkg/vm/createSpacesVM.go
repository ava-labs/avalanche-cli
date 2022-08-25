// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"encoding/json"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/spacesvm/chain"
	"github.com/ethereum/go-ethereum/common"
)

func CreateSpacesVMSubnetConfig(
	app *application.Avalanche,
	subnetName string,
	genesisPath string,
	spacesVMVersion string,
) ([]byte, *models.Sidecar, error) {
	var (
		genesisBytes []byte
		sc           *models.Sidecar
		err          error
	)

	if genesisPath == "" {
		genesisBytes, sc, err = createSpacesVMGenesis(app, subnetName, spacesVMVersion)
		if err != nil {
			return []byte{}, &models.Sidecar{}, err
		}
	} else {
		ux.Logger.PrintToUser("Importing genesis")
		genesisBytes, err = os.ReadFile(genesisPath)
		if err != nil {
			return []byte{}, &models.Sidecar{}, err
		}

		spacesVMVersion, err = getVMVersion(app, "Spaces VM", constants.SpacesVMRepoName, spacesVMVersion)
		if err != nil {
			return []byte{}, &models.Sidecar{}, err
		}

		sc = &models.Sidecar{
			Name:      subnetName,
			VM:        models.SpacesVM,
			VMVersion: spacesVMVersion,
			Subnet:    subnetName,
		}
	}

	return genesisBytes, sc, nil
}

func getMagic(app *application.Avalanche) (uint64, error) {
	ux.Logger.PrintToUser("Enter your spacevm's Magic. It should be a positive integer.")

    magic, err := app.Prompt.CaptureUint64("Magic [Default: 1]", "1")
	if err != nil {
		return 0, err
	}

	return magic, nil
}

func createSpacesVMGenesis(app *application.Avalanche, subnetName string, spacesVMVersion string) ([]byte, *models.Sidecar, error) {
	ux.Logger.PrintToUser("creating subnet %s", subnetName)

	customAllocs := []*chain.CustomAllocation{
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

	magic, err := getMagic(app)
	if err != nil {
		return []byte{}, &models.Sidecar{}, err
	}
	genesis.Magic = magic

	//genesis.AirdropHash = "0xccbf8e430b30d08b5b3342208781c40b373d1b5885c1903828f367230a2568da"
	//genesis.AirdropUnits = 10000
	genesis.CustomAllocation = customAllocs

	spacesVMVersion, err = getVMVersion(app, "Spaces VM", constants.SpacesVMRepoName, spacesVMVersion)
	if err != nil {
		return []byte{}, &models.Sidecar{}, err
	}

	jsonBytes, err := json.MarshalIndent(genesis, "", "    ")
	if err != nil {
		return []byte{}, nil, err
	}

	sc := &models.Sidecar{
		Name:      subnetName,
		VM:        models.SpacesVM,
		VMVersion: spacesVMVersion,
		Subnet:    subnetName,
	}

	return jsonBytes, sc, nil
}
