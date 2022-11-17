// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"golang.org/x/mod/semver"
)

var ErrNoAvagoVersion = errors.New("unable to find a compatible avalanchego version")

func GetRPCProtocolVersion(app *application.Avalanche, vmType models.VMType, vmVersion string) (int, error) {
	var url string

	switch vmType {
	case models.SubnetEvm:
		url = constants.SubnetEVMRPCCompatibilityURL
	case models.SpacesVM:
		url = constants.SpacesVMRPCCompatibilityURL
	default:
		return 0, errors.New("unknown VM type")
	}

	compatibilityBytes, err := app.Downloader.Download(url)
	if err != nil {
		return 0, err
	}

	var parsedCompat models.VMCompatibility
	if err = json.Unmarshal(compatibilityBytes, &parsedCompat); err != nil {
		return 0, err
	}

	version, ok := parsedCompat.RPCChainVMProtocolVersion[vmVersion]
	if !ok {
		return 0, errors.New("no RPC version found")
	}

	return version, nil
}

func GetLatestAvalancheGoByProtocolVersion(app *application.Avalanche, rpcVersion int) (string, error) {
	compatibilityBytes, err := app.Downloader.Download(constants.AvalancheGoCompatibilityURL)
	if err != nil {
		return "", err
	}

	var parsedCompat models.AvagoCompatiblity
	if err = json.Unmarshal(compatibilityBytes, &parsedCompat); err != nil {
		return "", err
	}

	eligibleVersions, ok := parsedCompat[strconv.Itoa(rpcVersion)]
	if !ok {
		return "", ErrNoAvagoVersion
	}

	// versions are not necessarily sorted, so we need to sort them, tho this puts them in ascending order
	semver.Sort(eligibleVersions)

	// get latest avago release to make sure we're not picking a release currently in progress but not available for download
	latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
	))
	if err != nil {
		return "", err
	}

	// we need to iterate in reverse order to start with latest version
	var useVersion string
	for i := len(eligibleVersions) - 1; i >= 0; i-- {
		versionComparison := semver.Compare(eligibleVersions[i], latestAvagoVersion)
		if versionComparison != 1 {
			useVersion = eligibleVersions[i]
			break
		}
	}

	if useVersion == "" {
		return "", ErrNoAvagoVersion
	}

	return useVersion, nil
}
