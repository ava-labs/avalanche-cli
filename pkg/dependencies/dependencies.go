// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package dependencies

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/mod/semver"

	"github.com/ava-labs/avalanche-cli/pkg/models"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

var ErrNoAvagoVersion = errors.New("unable to find a compatible avalanchego version")

func GetLatestAvalancheGoByProtocolVersion(app *application.Avalanche, rpcVersion int) (string, error) {
	useVersion, err := GetAvailableAvalancheGoVersions(app, rpcVersion, constants.AvalancheGoCompatibilityURL)
	if err != nil {
		return "", err
	}
	return useVersion[0], nil
}

func GetLatestCLISupportedDependencyVersion(app *application.Avalanche, dependencyName string, network models.Network, rpcVersion *int) (string, error) {
	dependencyBytes, err := app.Downloader.Download(constants.CLILatestDependencyURL)
	if err != nil {
		return "", err
	}

	var parsedDependency models.CLIDependencyMap
	if err = json.Unmarshal(dependencyBytes, &parsedDependency); err != nil {
		return "", err
	}

	switch dependencyName {
	case constants.AvalancheGoRepoName:
		// if the user is using RPC that is lower than the latest RPC supported by CLI, user will get latest AvalancheGo version for that RPC
		// based on "https://raw.githubusercontent.com/ava-labs/avalanchego/master/version/compatibility.json"
		if rpcVersion != nil && parsedDependency.RPC > *rpcVersion {
			return GetLatestAvalancheGoByProtocolVersion(
				app,
				*rpcVersion,
			)
		}
		return parsedDependency.AvalancheGo[network.Name()].LatestVersion, nil
	case constants.SubnetEVMRepoName:
		// Currently we get latest subnet evm version during blockchain create, which we then use to update the sidecar.
		// From the latest subnet evm version obtained, we then get the rpc version, which is also updated in the sidecar during blockchain create.
		// Getting latest subnet evm version therefore is independent of rpc version and therefore there is no need for any rpc version checks here.
		// We default to local network if network is undefined in argument.
		if network == models.UndefinedNetwork {
			network = models.NewLocalNetwork()
		}
		return parsedDependency.SubnetEVM[network.Name()].LatestVersion, nil
	default:
		return "", fmt.Errorf("unsupported dependency: %s", dependencyName)
	}
}

// GetAvalancheGoVersionsForRPC returns list of compatible avalanche go versions for a specified rpcVersion
func GetAvalancheGoVersionsForRPC(app *application.Avalanche, rpcVersion int, url string) ([]string, error) {
	compatibilityBytes, err := app.Downloader.Download(url)
	if err != nil {
		return nil, err
	}

	var parsedCompat models.AvagoCompatiblity
	if err = json.Unmarshal(compatibilityBytes, &parsedCompat); err != nil {
		return nil, err
	}

	eligibleVersions, ok := parsedCompat[strconv.Itoa(rpcVersion)]
	if !ok {
		return nil, ErrNoAvagoVersion
	}

	// versions are not necessarily sorted, so we need to sort them, tho this puts them in ascending order
	semver.Sort(eligibleVersions)
	return eligibleVersions, nil
}

// GetAvailableAvalancheGoVersions returns list of only available for download avalanche go versions,
// with latest version in first index
func GetAvailableAvalancheGoVersions(app *application.Avalanche, rpcVersion int, url string) ([]string, error) {
	eligibleVersions, err := GetAvalancheGoVersionsForRPC(app, rpcVersion, url)
	if err != nil {
		return nil, ErrNoAvagoVersion
	}
	// get latest avago release to make sure we're not picking a release currently in progress but not available for download
	latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
		"",
	)
	if err != nil {
		return nil, err
	}
	var availableVersions []string
	for i := len(eligibleVersions) - 1; i >= 0; i-- {
		versionComparison := semver.Compare(eligibleVersions[i], latestAvagoVersion)
		if versionComparison != 1 {
			availableVersions = append(availableVersions, eligibleVersions[i])
		}
	}
	if len(availableVersions) == 0 {
		return nil, ErrNoAvagoVersion
	}
	return availableVersions, nil
}
