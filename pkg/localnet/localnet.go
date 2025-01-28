// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/ava-labs/avalanchego/utils/logging"
)

var ErrNetworkNotBootstrapped = errors.New("network is not bootstrapped")

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

func LocalNetworkIsBootstrapped(app *application.Avalanche) (bool, error) {
	status, err := LocalNetworkBootstrappingStatus(app)
	if err != nil {
		return false, err
	}
	return status == FullyBootstrapped, nil
}

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

func GetLocalNetwork(app *application.Avalanche) (*tmpnet.Network, error) {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return nil, err
	}
	return GetTmpNetNetwork(networkDir)
}

func GetLocalNetworkEndpoint(app *application.Avalanche) (string, error) {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return "", err
	}
	return GetTmpNetEndpoint(networkDir)
}

func GetLocalNetworkBlockchainInfo(app *application.Avalanche) ([]BlockchainInfo, error) {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return nil, err
	}
	return GetTmpNetBlockchainInfo(networkDir)
}

func GetClusterInfoWithEndpoint(grpcServerEndpoint string) (*rpcpb.ClusterInfo, error) {
	cli, err := binutils.NewGRPCClientWithEndpoint(
		grpcServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return nil, err
	}
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	resp, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}
	return resp.GetClusterInfo(), nil
}

type ExtraLocalNetworkData struct {
	AvalancheGoPath                  string
	RelayerPath                      string
	CChainTeleporterMessengerAddress string
	CChainTeleporterRegistryAddress  string
}

func GetExtraLocalNetworkData(app *application.Avalanche, rootDataDir string) (bool, ExtraLocalNetworkData, error) {
	extraLocalNetworkData := ExtraLocalNetworkData{}
	if rootDataDir == "" {
		var err error
		rootDataDir, err = GetLocalNetworkDir(app)
		if err != nil {
			return false, extraLocalNetworkData, err
		}
	}
	extraLocalNetworkDataPath := filepath.Join(rootDataDir, constants.ExtraLocalNetworkDataFilename)
	if !utils.FileExists(extraLocalNetworkDataPath) {
		return false, extraLocalNetworkData, nil
	}
	bs, err := os.ReadFile(extraLocalNetworkDataPath)
	if err != nil {
		return false, extraLocalNetworkData, err
	}
	if err := json.Unmarshal(bs, &extraLocalNetworkData); err != nil {
		return false, extraLocalNetworkData, err
	}
	return true, extraLocalNetworkData, nil
}

func WriteExtraLocalNetworkData(
	app *application.Avalanche,
	rootDataDir string,
	avalancheGoPath string,
	relayerPath string,
	cchainICMMessengerAddress string,
	cchainICMRegistryAddress string,
) error {
	if rootDataDir == "" {
		var err error
		rootDataDir, err = GetLocalNetworkDir(app)
		if err != nil {
			return err
		}
	}
	extraLocalNetworkData := ExtraLocalNetworkData{}
	extraLocalNetworkDataPath := filepath.Join(rootDataDir, constants.ExtraLocalNetworkDataFilename)
	if utils.FileExists(extraLocalNetworkDataPath) {
		var err error
		_, extraLocalNetworkData, err = GetExtraLocalNetworkData(app, rootDataDir)
		if err != nil {
			return err
		}
	}
	if avalancheGoPath != "" {
		extraLocalNetworkData.AvalancheGoPath = utils.ExpandHome(avalancheGoPath)
	}
	if relayerPath != "" {
		extraLocalNetworkData.RelayerPath = utils.ExpandHome(relayerPath)
	}
	if cchainICMMessengerAddress != "" {
		extraLocalNetworkData.CChainTeleporterMessengerAddress = cchainICMMessengerAddress
	}
	if cchainICMRegistryAddress != "" {
		extraLocalNetworkData.CChainTeleporterRegistryAddress = cchainICMRegistryAddress
	}
	bs, err := json.Marshal(&extraLocalNetworkData)
	if err != nil {
		return err
	}
	return os.WriteFile(extraLocalNetworkDataPath, bs, constants.WriteReadReadPerms)
}

// assumes server is up
func IsBootstrappedOld(ctx context.Context, cli client.Client) (bool, error) {
	_, err := cli.Status(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			return false, nil
		}
		return false, fmt.Errorf("failed trying to get network status: %w", err)
	}
	return true, nil
}

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

func GetLocalNetworkDefaultContext() (context.Context, context.CancelFunc) {
	return sdkutils.GetTimedContext(2 * time.Minute)
}

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
