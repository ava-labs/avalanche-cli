// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func CreateCustomGenesis(name string, app *application.Avalanche) ([]byte, *models.Sidecar, error) {
	ux.Logger.PrintToUser("creating custom VM subnet %s", name)

	genesisPath, err := app.Prompt.CaptureExistingFilepath("Enter path to custom genesis")
	if err != nil {
		return []byte{}, nil, err
	}

	sc := &models.Sidecar{
		Name:      name,
		VM:        models.CustomVM,
		Subnet:    name,
		TokenName: "",
	}

	genesisBytes, err := os.ReadFile(genesisPath)
	return genesisBytes, sc, err
}
