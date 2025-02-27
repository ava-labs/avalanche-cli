// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanchego/utils/logging"
	"golang.org/x/mod/semver"
)

var (
	binaryToVersion map[string]string
	lock            = &sync.Mutex{}

	_ VersionMapper = &versionMapper{}
)

/*
VersionMapper keys and their usage:
 * OnlyAvagoKey: 					Used when running one avalanchego only (no compatibility required)

 * MultiAvago1Key					Used for the update scenario where avalanchego is updated and
 * MultiAvago2Key    			both avalanchego versions need to be compatible.
 * MultiAvagoSubnetEVMKey	This is the Subnet-EVM version compatible to the above scenario.

 * LatestEVM2AvagoKey 	  Latest subnet-evm version
 * LatestAvago2EVMKey     while this is the latest avalanchego compatible with that subnet-evm

 * SoloSubnetEVMKey1 			This is used when we want to test subnet-evm versions where compatibility
 * SoloSubnetEVMKey2      needs to be between the two subnet-evm versions
 													(latest might not be compatible with second latest)


*/

// VersionMapper is an abstraction for retrieving version compatibility URLs
// allowing unit tests without requiring external http calls.
// The idea is to finally calculate which VM is compatible with which Avalanchego,
// so that the e2e tests can always download and run the latest compatible versions,
// without having to manually update the e2e tests periodically.
type VersionMapper interface {
	GetCompatURL(vmType models.VMType) string
	GetAvagoURL() string
	GetApp() *application.Avalanche
	GetLatestAvagoByProtoVersion(app *application.Avalanche, rpcVersion int, url string) (string, error)
	GetEligibleVersions(sortedVersions []string, repoName string, app *application.Avalanche) ([]string, error)
	FilterAvailableVersions(versions []string) []string
}

// NewVersionMapper returns the default VersionMapper for e2e tests
func NewVersionMapper() VersionMapper {
	app := &application.Avalanche{
		Downloader: application.NewDownloader(),
		Log:        logging.NoLog{},
	}
	return &versionMapper{
		app: app,
	}
}

// versionMapper is the default implementation for version mapping.
// It downloads compatibility URLs from the actual github endpoints
type versionMapper struct {
	app *application.Avalanche
}

// GetLatestAvagoByProtoVersion returns the latest Avalanchego version which
// runs with the specified rpcVersion, or an error if it can't be found
// (or other errors occurred)
func (*versionMapper) GetLatestAvagoByProtoVersion(app *application.Avalanche, rpcVersion int, url string) (string, error) {
	return vm.GetLatestAvalancheGoByProtocolVersion(app, rpcVersion, url)
}

// GetApp returns the Avalanche application instance
func (m *versionMapper) GetApp() *application.Avalanche {
	return m.app
}

// GetCompatURL returns the compatibility URL for the given VM type
func (*versionMapper) GetCompatURL(vmType models.VMType) string {
	switch vmType {
	case models.SubnetEvm:
		return constants.SubnetEVMRPCCompatibilityURL
	case models.CustomVM:
		// TODO: unclear yet what we should return here
		return ""
	default:
		return ""
	}
}

// GetAvagoURL returns the compatibility URL for Avalanchego
func (*versionMapper) GetAvagoURL() string {
	return constants.AvalancheGoCompatibilityURL
}

func (*versionMapper) GetEligibleVersions(sortedVersions []string, repoName string, app *application.Avalanche) ([]string, error) {
	// get latest avago release to make sure we're not picking a release currently in progress but not available for download
	latest, err := app.Downloader.GetLatestReleaseVersion(
		constants.AvaLabsOrg,
		repoName,
		"",
	)
	if err != nil {
		return nil, err
	}

	var eligible []string
	for i, ver := range sortedVersions {
		versionComparison := semver.Compare(ver, latest)
		if versionComparison != 0 {
			continue
		}
		eligible = sortedVersions[i:]
		break
	}

	return eligible, nil
}

func (*versionMapper) FilterAvailableVersions(versions []string) []string {
	availableVersions := []string{}
	for _, v := range versions {
		resp, err := binutils.CheckReleaseVersion(logging.NoLog{}, constants.SubnetEVMRepoName, v)
		if err != nil {
			continue
		}
		availableVersions = append(availableVersions, v)
		resp.Body.Close()
	}
	return availableVersions
}

// GetVersionMapping returns a map of specific VMs resp. Avalanchego e2e context keys
// to the actual version which corresponds to that key.
// This allows the e2e test to know what version to download and run.
// Returns an error if there was a problem reading the URL compatibility json
// or some other issue.
func GetVersionMapping(mapper VersionMapper) (map[string]string, error) {
	// ginkgo can run tests in parallel. However, we really just need this mapping to be
	// performed once for the whole duration of a test.
	// Therefore we store the result in a global variable, and then lock
	// the access to it.
	lock.Lock()
	defer lock.Unlock()
	// if mapping has already been done, return it right away
	if binaryToVersion != nil {
		return binaryToVersion, nil
	}
	// get compatible versions for subnetEVM
	// subnetEVMversions is a list of sorted EVM versions,
	// subnetEVMmapping maps EVM versions to their RPC versions
	subnetEVMversions, subnetEVMmapping, err := getVersions(mapper, models.SubnetEvm)
	if err != nil {
		return nil, err
	}

	// subnet-evm publishes its upcoming new version in the compatibility json
	// before the new version is actually a downloadable release
	subnetEVMversions, err = mapper.GetEligibleVersions(subnetEVMversions, constants.SubnetEVMRepoName, mapper.GetApp())
	if err != nil {
		return nil, err
	}

	subnetEVMversions = mapper.FilterAvailableVersions(subnetEVMversions)

	// now get the avalanchego compatibility object
	avagoCompat, err := getAvagoCompatibility(mapper)
	if err != nil {
		return nil, err
	}

	// create the global mapping variable
	binaryToVersion = make(map[string]string)

	// sort avago compatibility by highest available RPC versions
	// to lowest (the map can not be iterated in a sorted way)
	rpcs := make([]int, 0, len(avagoCompat))
	for k := range avagoCompat {
		// cannot use string sort
		kint, err := strconv.Atoi(k)
		if err != nil {
			return nil, err
		}
		rpcs = append(rpcs, kint)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(rpcs)))

	// iterate the rpc versions
	// evaluate two avalanchego versions which are consecutive
	// and run with the same RPC version.
	// This is required for the for the "can deploy with multiple avalanchego versions" test
	for _, rpcVersion := range rpcs {
		versionAsString := strconv.Itoa(rpcVersion)
		versionsForRPC := avagoCompat[versionAsString]
		// we will take either the same, or two consecutive versions for the same RPC version
		if len(versionsForRPC) > 0 {
			versionsForRPC = reverseSemverSort(versionsForRPC)
			binaryToVersion[MultiAvago1Key] = versionsForRPC[0]
			if len(versionsForRPC) == 1 {
				binaryToVersion[MultiAvago2Key] = versionsForRPC[0]
			} else {
				binaryToVersion[MultiAvago2Key] = versionsForRPC[1]
			}

			// now iterate the subnetEVMversions and find a
			// subnet-evm version which is compatible with that RPC version.
			// The above-mentioned test runs with this as well.
			for _, evmVer := range subnetEVMversions {
				if subnetEVMmapping[evmVer] == rpcVersion {
					// we know there already exists at least one such combination.
					// unless the compatibility JSON will start to be shortened in some way,
					// we should always be able to find a matching subnet-evm
					binaryToVersion[MultiAvagoSubnetEVMKey] = evmVer
					// found the version, break
					break
				}
			}
			// all good, don't need to look more
			break
		}
	}

	// when running Avago only, always use latest
	binaryToVersion[OnlyAvagoKey] = OnlyAvagoValue

	// now let's look for subnet-evm versions which are fit for the
	// "can deploy multiple subnet-evm versions" test.
	// We need two subnet-evm versions which run the same RPC version,
	// and then a compatible Avalanchego
	//
	// To avoid having to iterate again, we'll also fill the values
	// for the **latest** compatible Avalanchego and Subnet-EVM
	for i, ver := range subnetEVMversions {
		// safety check, should not happen, as we already know
		// compatible versions exist
		secondIndex := i + 1
		if secondIndex == len(subnetEVMversions) {
			secondIndex = i
		}
		first := ver
		second := subnetEVMversions[secondIndex]
		// we should be able to safely assume that for a given subnet-evm RPC version,
		// there exists at least one compatible Avalanchego.
		// This means we can in any case use this to set the **latest** compatibility
		soloAvago, err := mapper.GetLatestAvagoByProtoVersion(mapper.GetApp(), subnetEVMmapping[first], mapper.GetAvagoURL())
		if err != nil {
			return nil, err
		}
		// Once latest compatibility has been set, we can skip this
		if binaryToVersion[LatestEVM2AvagoKey] == "" {
			binaryToVersion[LatestEVM2AvagoKey] = first
			binaryToVersion[LatestAvago2EVMKey] = soloAvago
		}
		if i+1 == len(subnetEVMversions) {
			// no compatible versions for subsequent SubnetEVM found, but we have no options anyway
			binaryToVersion[SoloSubnetEVMKey1] = binaryToVersion[LatestEVM2AvagoKey]
			binaryToVersion[SoloSubnetEVMKey2] = binaryToVersion[LatestEVM2AvagoKey]
			binaryToVersion[SoloAvagoKey] = binaryToVersion[LatestAvago2EVMKey]
			break
		}
		// first and second are compatible
		if subnetEVMmapping[first] == subnetEVMmapping[second] {
			binaryToVersion[SoloSubnetEVMKey1] = first
			binaryToVersion[SoloSubnetEVMKey2] = second
			binaryToVersion[SoloAvagoKey] = soloAvago
			break
		}
	}

	return binaryToVersion, nil
}

// getVersions gets compatible versions for the given VM type.
// Returns a correctly ordered list of semantic version strings,
// from latest to oldest, and a map of version to rpc
func getVersions(mapper VersionMapper, vmType models.VMType) ([]string, map[string]int, error) {
	compat, err := getCompatibility(mapper, vmType)
	if err != nil {
		return nil, nil, err
	}
	mapping := compat.RPCChainVMProtocolVersion
	if len(mapping) == 0 {
		return nil, nil, errors.New("zero length rpcs")
	}
	versions := make([]string, len(mapping))
	if len(versions) == 0 {
		return nil, nil, errors.New("zero length versions")
	}
	i := 0
	for v := range mapping {
		versions[i] = v
		i++
	}

	versions = reverseSemverSort(versions)
	return versions, mapping, nil
}

// getCompatibility returns the compatibility object for the given VM type
func getCompatibility(mapper VersionMapper, vmType models.VMType) (models.VMCompatibility, error) {
	compatibilityBytes, err := mapper.GetApp().GetDownloader().Download(mapper.GetCompatURL(vmType))
	if err != nil {
		return models.VMCompatibility{}, err
	}

	var parsedCompat models.VMCompatibility
	if err = json.Unmarshal(compatibilityBytes, &parsedCompat); err != nil {
		return models.VMCompatibility{}, err
	}
	return parsedCompat, nil
}

// getAvagoCompatibility returns the compatibility for Avalanchego
func getAvagoCompatibility(mapper VersionMapper) (models.AvagoCompatiblity, error) {
	avagoBytes, err := mapper.GetApp().GetDownloader().Download(mapper.GetAvagoURL())
	if err != nil {
		return nil, err
	}

	var avagoCompat models.AvagoCompatiblity
	if err = json.Unmarshal(avagoBytes, &avagoCompat); err != nil {
		return nil, err
	}

	return avagoCompat, nil
}

// For semantic version slices, we can't just reverse twice:
// the semver packages only has increasing `Sort`, while
// `sort.Sort(sort.Reverse(sort.StringSlice(sliceSortedWithSemverSort)))`
// again fails to sort correctly (as it will sort again with string sorting
// instead of semantic versioning)
func reverseSemverSort(slice []string) []string {
	semver.Sort(slice)
	reverse := make([]string, len(slice))
	for i, s := range slice {
		idx := len(slice) - (1 + i)
		reverse[idx] = s
	}
	return reverse
}
