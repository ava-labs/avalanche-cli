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

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanche-network-runner/server"
)

func GetClusterInfo() (*rpcpb.ClusterInfo, error) {
	cli, err := binutils.NewGRPCClient(
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
	avalancheGoPath string,
	relayerPath string,
	cchainTeleporterMessengerAddress string,
	cchainTeleporterRegistryAddress string,
) error {
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		return err
	}
	extraLocalNetworkDataPath := filepath.Join(clusterInfo.GetRootDataDir(), constants.ExtraLocalNetworkDataFilename)
	extraLocalNetworkData := ExtraLocalNetworkData{}
	if utils.FileExists(extraLocalNetworkDataPath) {
		var err error
		_, extraLocalNetworkData, err = GetExtraLocalNetworkData("")
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
	if cchainTeleporterMessengerAddress != "" {
		extraLocalNetworkData.CChainTeleporterMessengerAddress = cchainTeleporterMessengerAddress
	}
	if cchainTeleporterRegistryAddress != "" {
		extraLocalNetworkData.CChainTeleporterRegistryAddress = cchainTeleporterRegistryAddress
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

func CheckNetworkIsAlreadyBootstrapped(ctx context.Context, cli client.Client) (bool, error) {
	_, err := cli.Status(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			return false, nil
		}
		return false, fmt.Errorf("failed trying to get network status: %w", err)
	}
	return true, nil
}
