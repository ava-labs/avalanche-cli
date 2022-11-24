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
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"golang.org/x/mod/semver"
)

var (
	mapping map[string]string
	lock    = &sync.Mutex{}
)

func GetVersionMapping(app *application.Avalanche) (map[string]string, error) {
	lock.Lock()
	defer lock.Unlock()
	if mapping != nil {
		return mapping, nil
	}
	subnetEVMversions, subnetEVMmapping, err := getVersions(constants.SubnetEVMRPCCompatibilityURL, app)
	if err != nil {
		return nil, err
	}

	avagoCompat, err := getAvagoCompatibility(app)
	if err != nil {
		return nil, err
	}

	mapping := make(map[string]string)

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

	for _, v := range rpcs {
		strv := strconv.Itoa(v)
		vers := avagoCompat[strv]
		if len(vers) > 1 {
			semver.Sort(vers)
			mapping[MultiAvago1Key] = vers[0]
			mapping[MultiAvago2Key] = vers[1]

			for _, evmVer := range subnetEVMversions {
				if subnetEVMmapping[evmVer] == v {
					mapping[MultiAvagoSubnetEVMKey] = evmVer
					break
				}
			}
			break
		}
	}

	// when running Avago only, always use latest
	mapping[OnlyAvagoKey] = "latest"

	for i, ver := range subnetEVMversions {
		// safety check, should not happen
		if i == 0 {
			continue
		}
		if i+1 == len(subnetEVMversions) {
			return nil, errors.New("no compatible versions for subsecuent SubnetEVM found")
		}
		first := ver
		second := subnetEVMversions[i+1]
		// first and second are compatible
		if subnetEVMmapping[first] == subnetEVMmapping[second] {
			soloAvago, err := vm.GetLatestAvalancheGoByProtocolVersion(app, subnetEVMmapping[first])
			if err != nil {
				return nil, err
			}
			mapping[SoloSubnetEVMKey1] = first
			mapping[SoloSubnetEVMKey2] = second
			mapping[SoloAvagoKey] = soloAvago
			break
		}
	}

	return mapping, nil
}

func getVersions(url string, app *application.Avalanche) ([]string, map[string]int, error) {
	compat, err := getCompatibility(constants.SubnetEVMRPCCompatibilityURL, app)
	if err != nil {
		return nil, nil, err
	}
	mapping := compat.RPCChainVMProtocolVersion
	versions := make([]string, len(mapping))
	i := 0
	for v := range mapping {
		versions[i] = v
		i++
	}

	semver.Sort(versions)
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))
	return versions, mapping, nil
}

func getCompatibility(url string, app *application.Avalanche) (models.VMCompatibility, error) {
	compatibilityBytes, err := app.Downloader.Download(url)
	if err != nil {
		return models.VMCompatibility{}, err
	}

	var parsedCompat models.VMCompatibility
	if err = json.Unmarshal(compatibilityBytes, &parsedCompat); err != nil {
		return models.VMCompatibility{}, err
	}
	return parsedCompat, nil
}

func getAvagoCompatibility(app *application.Avalanche) (models.AvagoCompatiblity, error) {
	avagoBytes, err := app.Downloader.Download(constants.AvalancheGoCompatibilityURL)
	if err != nil {
		return nil, err
	}

	var avagoCompat models.AvagoCompatiblity
	if err = json.Unmarshal(avagoBytes, &avagoCompat); err != nil {
		return nil, err
	}

	return avagoCompat, nil
}
