// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"os"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/app"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/ux"
)

func CreateCustomGenesis(name string, app *app.Avalanche) ([]byte, *models.Sidecar, error) {
	ux.Logger.PrintToUser("creating custom VM subnet %s", name)

	genesisPath, err := prompts.CaptureExistingFilepath("Enter path to custom genesis")
	if err != nil {
		return []byte{}, nil, err
	}

	sc := &models.Sidecar{
		Name:      name,
		Vm:        models.CustomVm,
		Subnet:    name,
		TokenName: "",
	}

	genesisBytes, err := os.ReadFile(genesisPath)
	return genesisBytes, sc, err
}
