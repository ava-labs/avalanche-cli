// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"encoding/json"
	"errors"
	"sort"
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
	err = json.Unmarshal(compatibilityBytes, &parsedCompat)
	if err != nil {
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
	err = json.Unmarshal(compatibilityBytes, &parsedCompat)
	if err != nil {
		return "", err
	}

	eligibleVersions, ok := parsedCompat[strconv.Itoa(rpcVersion)]
	if !ok {
		return "", ErrNoAvagoVersion
	}

	// versions are not necessarily sorted, so we need to sort them
	sort.Sort(sort.Reverse(sort.StringSlice(eligibleVersions)))

	// get latest avago release to make sure we're not picking a release currently in progress but not available for download
	latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
	))
	if err != nil {
		return "", err
	}

	var useVersion string
	for _, proposedVersion := range eligibleVersions {
		versionComparison := semver.Compare(proposedVersion, latestAvagoVersion)
		if versionComparison != 1 {
			useVersion = proposedVersion
			break
		}
	}

	if useVersion == "" {
		return "", ErrNoAvagoVersion
	}

	return useVersion, nil
}
