// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package dependencies

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

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
		return parsedDependency.SubnetEVM, nil
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

type AvalancheGoVersionSettings struct {
	UseCustomAvalanchegoVersion           string
	UseLatestAvalanchegoReleaseVersion    bool
	UseLatestAvalanchegoPreReleaseVersion bool
	UseAvalanchegoVersionFromSubnet       string
}

// GetAvalancheGoVersion asks users whether they want to install the newest Avalanche Go version
// or if they want to use the newest Avalanche Go Version that is still compatible with Subnet EVM
// version of their choice
func GetAvalancheGoVersion(app *application.Avalanche, avagoVersion AvalancheGoVersionSettings, network models.Network) (string, error) {
	// skip this logic if custom-avalanchego-version flag is set
	if avagoVersion.UseCustomAvalanchegoVersion != "" {
		return avagoVersion.UseCustomAvalanchegoVersion, nil
	}
	latestReleaseVersion, err := GetLatestCLISupportedDependencyVersion(app, constants.AvalancheGoRepoName, network, nil)
	if err != nil {
		return "", err
	}
	latestPreReleaseVersion, err := app.Downloader.GetLatestPreReleaseVersion(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
		"",
	)
	if err != nil {
		return "", err
	}

	if !avagoVersion.UseLatestAvalanchegoReleaseVersion && !avagoVersion.UseLatestAvalanchegoPreReleaseVersion && avagoVersion.UseCustomAvalanchegoVersion == "" && avagoVersion.UseAvalanchegoVersionFromSubnet == "" {
		avagoVersion, err = promptAvalancheGoVersionChoice(app, latestReleaseVersion, latestPreReleaseVersion)
		if err != nil {
			return "", err
		}
	}

	var version string
	switch {
	case avagoVersion.UseLatestAvalanchegoReleaseVersion:
		version = latestReleaseVersion
	case avagoVersion.UseLatestAvalanchegoPreReleaseVersion:
		version = latestPreReleaseVersion
	case avagoVersion.UseCustomAvalanchegoVersion != "":
		version = avagoVersion.UseCustomAvalanchegoVersion
	case avagoVersion.UseAvalanchegoVersionFromSubnet != "":
		sc, err := app.LoadSidecar(avagoVersion.UseAvalanchegoVersionFromSubnet)
		if err != nil {
			return "", err
		}
		version, err = GetLatestCLISupportedDependencyVersion(app, constants.AvalancheGoRepoName, network, &sc.RPCVersion)
		if err != nil {
			return "", err
		}
	}
	return version, nil
}

// promptAvalancheGoVersionChoice sets flags for either using the latest Avalanche Go
// version or using the latest Avalanche Go version that is still compatible with the subnet that user
// wants the cloud server to track
func promptAvalancheGoVersionChoice(app *application.Avalanche, latestReleaseVersion string, latestPreReleaseVersion string) (AvalancheGoVersionSettings, error) {
	versionComments := map[string]string{
		"v1.11.0-fuji": " (recommended for fuji durango)",
	}
	latestReleaseVersionOption := "Use latest Avalanche Go Release Version" + versionComments[latestReleaseVersion]
	latestPreReleaseVersionOption := "Use latest Avalanche Go Pre-release Version" + versionComments[latestPreReleaseVersion]
	subnetBasedVersionOption := "Use the deployed Subnet's VM version that the node will be validating"
	customOption := "Custom"

	txt := "What version of Avalanche Go would you like to install in the node?"
	versionOptions := []string{latestReleaseVersionOption, subnetBasedVersionOption, customOption}
	if latestPreReleaseVersion != latestReleaseVersion {
		versionOptions = []string{latestPreReleaseVersionOption, latestReleaseVersionOption, subnetBasedVersionOption, customOption}
	}
	versionOption, err := app.Prompt.CaptureList(txt, versionOptions)
	if err != nil {
		return AvalancheGoVersionSettings{}, err
	}

	switch versionOption {
	case latestReleaseVersionOption:
		return AvalancheGoVersionSettings{UseLatestAvalanchegoReleaseVersion: true}, nil
	case latestPreReleaseVersionOption:
		return AvalancheGoVersionSettings{UseLatestAvalanchegoPreReleaseVersion: true}, nil
	case customOption:
		useCustomAvalanchegoVersion, err := app.Prompt.CaptureVersion("Which version of AvalancheGo would you like to install? (Use format v1.10.13)")
		if err != nil {
			return AvalancheGoVersionSettings{}, err
		}
		return AvalancheGoVersionSettings{UseCustomAvalanchegoVersion: useCustomAvalanchegoVersion}, nil
	default:
		useAvalanchegoVersionFromSubnet := ""
		for {
			useAvalanchegoVersionFromSubnet, err = app.Prompt.CaptureString("Which Subnet would you like to use to choose the avalanche go version?")
			if err != nil {
				return AvalancheGoVersionSettings{}, err
			}
			_, err = subnet.ValidateSubnetNameAndGetChains(app, []string{useAvalanchegoVersionFromSubnet})
			if err == nil {
				break
			}
			ux.Logger.PrintToUser(fmt.Sprintf("no blockchain named as %s found", useAvalanchegoVersionFromSubnet))
		}
		return AvalancheGoVersionSettings{UseAvalanchegoVersionFromSubnet: useAvalanchegoVersionFromSubnet}, nil
	}
}
