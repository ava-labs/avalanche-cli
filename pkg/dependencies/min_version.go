// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package dependencies

import (
	"encoding/json"
	"fmt"
	"golang.org/x/mod/semver"

	"github.com/ava-labs/avalanche-cli/pkg/models"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func CheckVersionIsOverMin(app *application.Avalanche, dependencyName string, network models.Network, version string) error {
	dependencyBytes, err := app.Downloader.Download(constants.CLILatestDependencyURL)
	if err != nil {
		return err
	}

	var parsedDependency models.CLIDependencyMap
	if err = json.Unmarshal(dependencyBytes, &parsedDependency); err != nil {
		return err
	}

	switch dependencyName {
	case constants.AvalancheGoRepoName:
		// version has to be at least higher than minimum version specified for the dependency
		minVersion := parsedDependency.AvalancheGo[network.Name()].MinimumVersion
		versionComparison := semver.Compare(version, minVersion)
		if versionComparison == -1 {
			return fmt.Errorf("minimum version of %s that is supported by CLI is %s, current version provided is %s", dependencyName, minVersion, version)
		}
		return nil
	default:
		return fmt.Errorf("minimum version check is unsupported %s dependency", dependencyName)
	}
}
