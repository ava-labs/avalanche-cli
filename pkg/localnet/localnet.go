// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	sdkUtils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
)

func GetLocalNetworkEndpoint(app *application.Avalanche) (string, error) {
	network, err := GetLocalNetworkInfo(app)
	if err != nil {
		return "", err
	}
	if len(network.Nodes) == 0 {
		return "", fmt.Errorf("no node found on local network")
	}
	return network.Nodes[0].URI, nil
}

func GetLocalNetworkInfo(app *application.Avalanche) (*tmpnet.Network, error) {
	status, err := LocalnetBootstrappingStatus(app)
	if err != nil {
		return nil, err
	}
	if status != FullyBootstrapped {
		return nil, fmt.Errorf("network is not bootstrapped")
	}
	meta, err := GetExecutingLocalnetMeta(app)
	if err != nil {
		return nil, err
	}
	return GetTmpNetNetwork(meta.NetworkDir)
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
	ctx, cancel := utils.GetAPIContext()
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
		network, err := GetLocalNetworkInfo(app)
		if err != nil {
			return false, extraLocalNetworkData, err
		}
		rootDataDir = network.Dir
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
		network, err := GetLocalNetworkInfo(app)
		if err != nil {
			return err
		}
		rootDataDir = network.Dir
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

func Deployed(subnetName string) (bool, error) {
	if _, err := utils.GetChainID(models.NewLocalNetwork().Endpoint, subnetName); err != nil {
		if !strings.Contains(err.Error(), "connection refused") && !strings.Contains(err.Error(), "there is no ID with alias") {
			return false, err
		}
		return false, nil
	}
	return true, nil
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

func LocalnetBootstrappingStatus(app *application.Avalanche) (BootstrappingStatus, error) {
	if ExecutingLocalnetMetaExists(app) {
		executingLocalnetMeta, err := GetExecutingLocalnetMeta(app)
		if err != nil {
			return UndefinedBootstrappingStatus, err
		}
		if sdkUtils.DirExists(executingLocalnetMeta.NetworkDir) {
			status, err := GetTmpNetBootstrappingStatus(executingLocalnetMeta.NetworkDir)
			if err != nil {
				return status, err
			}
			if status == NotBootstrapped {
				if err := RemoveExecutingLocalnetMeta(app); err != nil {
					return NotBootstrapped, err
				}
			}
			return status, nil
		}
	}
	return NotBootstrapped, nil
}

func GetVersion(app *application.Avalanche) (bool, string, int, error) {
	// not actually an error, network just not running
	_, err := GetLocalNetworkInfo(app)
	if err != nil {
		return false, "", 0, nil
	}
	endpoint, err := GetLocalNetworkEndpoint(app)
	if err != nil {
		return true, "", 0, err
	}
	ctx := context.Background()
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

func GetBlockchainNames() ([]string, error) {
	return nil, nil
	/*
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		return nil, err
	}
	blockchainNames := []string{}
	for _, chainInfo := range clusterInfo.CustomChains {
		blockchainNames = append(blockchainNames, chainInfo.ChainName)
	}
	return blockchainNames, nil
	*/
}

func GetDefaultTimeout() (context.Context, context.CancelFunc) {
	return utils.GetTimedContext(2 * time.Minute)
}
