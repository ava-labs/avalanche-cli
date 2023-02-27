// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func GetLocallyDeployedSubnetsFromFile(app *application.Avalanche) ([]string, error) {
	allSubnetDirs, err := os.ReadDir(app.GetSubnetDir())
	if err != nil {
		return nil, err
	}

	deployedSubnets := []string{}

	for _, subnetDir := range allSubnetDirs {
		// read sidecar file
		sc, err := app.LoadSidecar(subnetDir.Name())
		if err == os.ErrNotExist {
			// don't fail on missing sidecar file, just warn
			ux.Logger.PrintToUser("warning: inconsistent subnet directory. No sidecar file found for subnet %s", subnetDir.Name())
			continue
		}
		if err != nil {
			return nil, err
		}

		// check if sidecar contains local deployment info in Networks map
		// if so, add to list of deployed subnets
		if _, ok := sc.Networks[models.Local.String()]; ok {
			deployedSubnets = append(deployedSubnets, sc.Name)
		}
	}

	return deployedSubnets, nil
}
