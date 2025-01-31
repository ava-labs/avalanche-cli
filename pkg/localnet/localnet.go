// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/ava-labs/avalanchego/utils/logging"
)

var ErrNetworkNotBootstrapped = errors.New("network is not bootstrapped")

// Indicates if all, some or none of the local network nodes are alive
func LocalNetworkBootstrappingStatus(app *application.Avalanche) (BootstrappingStatus, error) {
	if LocalNetworkMetaExists(app) {
		meta, err := GetLocalNetworkMeta(app)
		if err != nil {
			return UndefinedBootstrappingStatus, err
		}
		if sdkutils.DirExists(meta.NetworkDir) {
			status, err := GetTmpNetBootstrappingStatus(meta.NetworkDir)
			if err != nil {
				return status, err
			}
			if status == NotBootstrapped {
				if err := RemoveLocalNetworkMeta(app); err != nil {
					return NotBootstrapped, err
				}
			}
			return status, nil
		}
	}
	return NotBootstrapped, nil
}

// Returns true if all local network nodes are alive
func LocalNetworkIsBootstrapped(app *application.Avalanche) (bool, error) {
	status, err := LocalNetworkBootstrappingStatus(app)
	if err != nil {
		return false, err
	}
	return status == FullyBootstrapped, nil
}

// Returns the tmpnet directory associated to the local network
// If the network is not alive it errors
func GetLocalNetworkDir(app *application.Avalanche) (string, error) {
	isBootstrapped, err := LocalNetworkIsBootstrapped(app)
	if err != nil {
		return "", err
	}
	if !isBootstrapped {
		return "", ErrNetworkNotBootstrapped
	}
	meta, err := GetLocalNetworkMeta(app)
	if err != nil {
		return "", err
	}
	return meta.NetworkDir, nil
}

// Returns the tmpnet associated to the local network
// If the network is not alive it errors
func GetLocalNetwork(app *application.Avalanche) (*tmpnet.Network, error) {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return nil, err
	}
	return GetTmpNetNetwork(networkDir)
}

// Returns the endpoint associated to the local network
// If the network is not alive it errors
func GetLocalNetworkEndpoint(app *application.Avalanche) (string, error) {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return "", err
	}
	return GetTmpNetEndpoint(networkDir)
}

// Returns blockchain info for all non standard blockchains deployed into the local network
func GetLocalNetworkBlockchainInfo(app *application.Avalanche) ([]BlockchainInfo, error) {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return nil, err
	}
	return GetTmpNetBlockchainInfo(networkDir)
}

// Returns avalanchego version and RPC version for the local network
func GetLocalNetworkAvalancheGoVersion(app *application.Avalanche) (bool, string, int, error) {
	// not actually an error, network just not running
	if isBootstrapped, err := LocalNetworkIsBootstrapped(app); err != nil {
		return true, "", 0, err
	} else if !isBootstrapped {
		return false, "", 0, nil
	}
	endpoint, err := GetLocalNetworkEndpoint(app)
	if err != nil {
		return true, "", 0, err
	}
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	infoClient := info.NewClient(endpoint)
	versionResponse, err := infoClient.GetNodeVersion(ctx)
	if err != nil {
		return true, "", 0, err
	}
	// version is in format avalanche/x.y.z, need to turn to semantic
	splitVersion := strings.Split(versionResponse.Version, "/")
	if len(splitVersion) != 2 {
		return true, "", 0, fmt.Errorf("unable to parse avalanchego version " + versionResponse.Version)
	}
	// index 0 should be avalanche, index 1 will be version
	parsedVersion := "v" + splitVersion[1]
	return true, parsedVersion, int(versionResponse.RPCProtocolVersion), nil
}

// Stops the local network
func LocalNetworkStop(app *application.Avalanche) error {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return err
	}
	if err := TmpNetStop(networkDir); err != nil {
		return err
	}
	return RemoveLocalNetworkMeta(app)
}

// Returns a context large enough to support all local network operations
func GetLocalNetworkDefaultContext() (context.Context, context.CancelFunc) {
	return sdkutils.GetTimedContext(2 * time.Minute)
}

// Indicates if the local network validates a subnet at all
func LocalNetworkHasValidatorsForSubnet(
	app *application.Avalanche,
	subnetID ids.ID,
) (bool, error) {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return false, err
	}
	return TmpNetHasValidatorsForSubnet(networkDir, subnetID)
}

// Indicates if a blockchain is bootstrapped on the local network
// If the network has no validators for the blockchain, it fails
func IsLocalNetworkBlockchainBootstrapped(
	app *application.Avalanche,
	blockchainID string,
	subnetID ids.ID,
) (bool, error) {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return false, err
	}
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	return IsTmpNetBlockchainBootstrapped(ctx, networkDir, blockchainID, subnetID)
}

// Indicates if P-Chain is bootstrapped on the network, and also if
// all blockchain that have validators on the network, are bootstrapped
func LocalNetworkHealth(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
) (bool, bool, error) {
	pChainBootstrapped, err := IsLocalNetworkBlockchainBootstrapped(app, "P", ids.Empty)
	if err != nil {
		return false, false, err
	}
	blockchains, err := GetLocalNetworkBlockchainInfo(app)
	if err != nil {
		return pChainBootstrapped, false, err
	}
	for _, blockchain := range blockchains {
		hasValidators, err := LocalNetworkHasValidatorsForSubnet(app, blockchain.SubnetID)
		if err != nil {
			return pChainBootstrapped, false, err
		}
		if !hasValidators {
			printFunc(logging.Red.Wrap("local network has no validators for subnet %s. l1 check is not implemented yet"), blockchain.SubnetID)
			printFunc("")
			return pChainBootstrapped, false, err
		}
		blockchainBootstrapped, err := IsLocalNetworkBlockchainBootstrapped(app, blockchain.ID.String(), blockchain.SubnetID)
		if err != nil {
			return pChainBootstrapped, false, err
		}
		if !blockchainBootstrapped {
			return pChainBootstrapped, false, nil
		}
	}
	return pChainBootstrapped, true, nil
}
