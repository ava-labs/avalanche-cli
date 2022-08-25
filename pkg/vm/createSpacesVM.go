// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"encoding/json"
	"math/big"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/spacesvm/chain"
)

const defaultSpacesVMAirdropAmount = "1000000"

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

	genesis := chain.DefaultGenesis()

	magic, err := getMagic(app)
	if err != nil {
		return []byte{}, &models.Sidecar{}, err
	}
	genesis.Magic = magic

	spacesVMVersion, err = getVMVersion(app, "Spaces VM", constants.SpacesVMRepoName, spacesVMVersion)
	if err != nil {
		return []byte{}, &models.Sidecar{}, err
	}

	allocs, _, err := getAllocation(app, defaultSpacesVMAirdropAmount, new(big.Int).SetUint64(1), "Amount to airdrop")
	if err != nil {
		return []byte{}, &models.Sidecar{}, err
	}

	customAllocs := []*chain.CustomAllocation{}
	for address, account := range allocs {
		alloc := chain.CustomAllocation{
			Address: address,
			Balance: account.Balance.Uint64(),
		}
		customAllocs = append(customAllocs, &alloc)
	}

	genesis.CustomAllocation = customAllocs

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
