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
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/node"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
)

func GetEndpoint() (string, error) {
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		return "", err
	}
	node1, ok := clusterInfo.NodeInfos["node1"]
	if !ok {
		return "", fmt.Errorf("node1 not found on local network")
	}
	return node1.Uri, nil
}

func GetClusterInfo() (*rpcpb.ClusterInfo, error) {
	return GetClusterInfoWithEndpoint(binutils.LocalNetworkGRPCServerEndpoint)
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

func GetExtraLocalNetworkData(rootDataDir string) (bool, ExtraLocalNetworkData, error) {
	extraLocalNetworkData := ExtraLocalNetworkData{}
	if rootDataDir == "" {
		clusterInfo, err := GetClusterInfo()
		if err != nil {
			return false, extraLocalNetworkData, err
		}
		rootDataDir = clusterInfo.GetRootDataDir()
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
	rootDataDir string,
	avalancheGoPath string,
	relayerPath string,
	cchainICMMessengerAddress string,
	cchainICMRegistryAddress string,
) error {
	if rootDataDir == "" {
		fmt.Println("ACA NO ENTRO")
		clusterInfo, err := GetClusterInfo()
		if err != nil {
			return err
		}
		rootDataDir = clusterInfo.GetRootDataDir()
	}
	extraLocalNetworkDataPath := filepath.Join(rootDataDir, constants.ExtraLocalNetworkDataFilename)
	extraLocalNetworkData := ExtraLocalNetworkData{}
	if utils.FileExists(extraLocalNetworkDataPath) {
		var err error
		_, extraLocalNetworkData, err = GetExtraLocalNetworkData(rootDataDir)
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

func IsBootstrapped(app *application.Avalanche) (bool, error) {
	someNodeIsUp := false
	if InfoExists(app) {
		currentLocalNetworkDir, err := ReadInfo(app)
		if err != nil {
			return false, err
		}
		if sdkUtils.DirExists(currentLocalNetworkDir) {
			someNodeIsUp, err = NetworkIsBootstrapped(currentLocalNetworkDir)
			if err != nil {
				return false, err
			}
		}
		if !someNodeIsUp {
			if err := RemoveInfo(app); err != nil {
				return false, err
			}
		}
	}
	return someNodeIsUp, nil
}

func NetworkIsBootstrapped(networkDir string) (bool, error) {
	network, err := tmpnet.ReadNetwork(networkDir)
	if err != nil {
		return false, err
	}
	for _, nod := range network.Nodes {
		processPath := filepath.Join(networkDir, nod.NodeID.String(), config.DefaultProcessContextFilename)
		if utils.FileExists(processPath) {
			bs, err := os.ReadFile(processPath)
			if err != nil {
				return false, err
			}
			processContext := node.ProcessContext{}
			if err := json.Unmarshal(bs, &processContext); err != nil {
				return false, fmt.Errorf("failed to unmarshal node process context: %w", err)
			}
			if _, err := utils.GetProcess(processContext.PID); err == nil {
				return true, nil
			}
		}
	}
	return false, nil
}

// server can be up or down
func GetVersion() (bool, string, int, error) {
	// not actually an error, network just not running
	_, err := GetClusterInfo()
	if err != nil {
		return false, "", 0, nil
	}
	endpoint, err := GetEndpoint()
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
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		return nil, err
	}
	blockchainNames := []string{}
	for _, chainInfo := range clusterInfo.CustomChains {
		blockchainNames = append(blockchainNames, chainInfo.ChainName)
	}
	return blockchainNames, nil
}

func GetDefaultTimeout() (context.Context, context.CancelFunc) {
	return utils.GetTimedContext(2 * time.Minute)
}
