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
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

var (
	mapping map[string]string
	lock    = &sync.Mutex{}
)

type VersionMapper interface {
	GetCompatURL(vmType models.VMType) string
	GetAvagoURL() string
	GetApp() *application.Avalanche
	GetLatestAvagoByProtoVersion(app *application.Avalanche, rpcVersion int, url string) (string, error)
}

func NewVersionMapper(app *application.Avalanche) VersionMapper {
	return &versionMapper{
		app: app,
	}
}

type versionMapper struct {
	app *application.Avalanche
}

func (m *versionMapper) GetLatestAvagoByProtoVersion(app *application.Avalanche, rpcVersion int, url string) (string, error) {
	return vm.GetLatestAvalancheGoByProtocolVersion(app, rpcVersion, url)
}

func (m *versionMapper) GetApp() *application.Avalanche {
	return m.app
}

func (m *versionMapper) GetCompatURL(vmType models.VMType) string {
	switch vmType {
	case models.SubnetEvm:
		return constants.SubnetEVMRPCCompatibilityURL
	case models.SpacesVM:
		return constants.SpacesVMRPCCompatibilityURL
	case models.CustomVM:
		// TODO: unclear yet what we should return here
		return ""
	default:
		return ""
	}
}

func (m *versionMapper) GetAvagoURL() string {
	return constants.AvalancheGoCompatibilityURL
}

func GetVersionMapping(mapper VersionMapper) (map[string]string, error) {
	lock.Lock()
	defer lock.Unlock()
	if mapping != nil {
		return mapping, nil
	}
	subnetEVMversions, subnetEVMmapping, err := getVersions(mapper, models.SubnetEvm)
	if err != nil {
		return nil, err
	}
	// subnet-evm publishes its upcoming new version in the compatibility json
	// before the new version is actually a downloadable release
	subnetEVMversions = subnetEVMversions[1:]

	avagoCompat, err := getAvagoCompatibility(mapper)
	if err != nil {
		return nil, err
	}

	mapping = make(map[string]string)

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
			sort.Sort(sort.Reverse(sort.StringSlice(vers)))
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
	mapping[OnlyAvagoKey] = OnlyAvagoValue

	for i, ver := range subnetEVMversions {
		// safety check, should not happen
		if i+1 == len(subnetEVMversions) {
			return nil, errors.New("no compatible versions for subsecuent SubnetEVM found")
		}
		first := ver
		second := subnetEVMversions[i+1]
		// first and second are compatible
		soloAvago, err := mapper.GetLatestAvagoByProtoVersion(mapper.GetApp(), subnetEVMmapping[first], mapper.GetAvagoURL())
		if err != nil {
			return nil, err
		}
		if mapping[LatestEVM2AvagoKey] == "" {
			mapping[LatestEVM2AvagoKey] = first
			mapping[LatestAvago2EVMKey] = soloAvago
		}
		if subnetEVMmapping[first] == subnetEVMmapping[second] {
			mapping[SoloSubnetEVMKey1] = first
			mapping[SoloSubnetEVMKey2] = second
			mapping[SoloAvagoKey] = soloAvago
			break
		}
	}

	mapper.GetApp().Log.Debug("mapping:",
		zap.String("SoloSubnetEVM1", mapping[SoloSubnetEVMKey1]),
		zap.String("SoloSubnetEVM2", mapping[SoloSubnetEVMKey2]),
		zap.String("SoloAvago", mapping[SoloAvagoKey]),
		zap.String("MultiAvago1", mapping[MultiAvago1Key]),
		zap.String("MultiAvago2", mapping[MultiAvago2Key]),
		zap.String("MultiAvagoSubnetEVM", mapping[MultiAvagoSubnetEVMKey]),
		zap.String("LatestEVM2Avago", mapping[LatestEVM2AvagoKey]),
		zap.String("LatestAvago2EVM", mapping[LatestAvago2EVMKey]),
	)

	return mapping, nil
}

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

	semver.Sort(versions)
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))
	return versions, mapping, nil
}

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
