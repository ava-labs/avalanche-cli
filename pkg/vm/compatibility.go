package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"golang.org/x/mod/semver"
)

var NoAvagoVersion = errors.New("unable to find a compatible avalanchego version")

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

	fmt.Println("Parsed compat:", parsedCompat)

	eligibleVersions, ok := parsedCompat[strconv.Itoa(rpcVersion)]
	if !ok {
		return "", NoAvagoVersion
	}

	fmt.Println("Eligible versions:", eligibleVersions)

	// versions are not necessarily sorted, so we need to sort them
	sort.Sort(sort.Reverse(sort.StringSlice(eligibleVersions)))

	// get latest avago release to make sure we're not picking a release currently in progress but not available for download
	latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
	))

	fmt.Println("All versions", eligibleVersions)

	var useVersion string
	for _, proposedVersion := range []string(eligibleVersions) {
		versionComparison := semver.Compare(proposedVersion, latestAvagoVersion)
		fmt.Println("Proposed version:", proposedVersion)
		fmt.Println("Comparison:", versionComparison)
		if versionComparison != 1 {
			useVersion = proposedVersion
			break
		}
	}

	if useVersion == "" {
		return "", NoAvagoVersion
	}

	return useVersion, nil
}
