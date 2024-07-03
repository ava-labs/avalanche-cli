// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
)

func GetClusterInfo() (*rpcpb.ClusterInfo, error) {
	cli, err := binutils.NewGRPCClient()
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
	CChainTeleporterMessengerAddress string
	CChainTeleporterRegistryAddress  string
}

func GetExtraLocalNetworkData() (bool, ExtraLocalNetworkData, error) {
	extraLocalNetworkData := ExtraLocalNetworkData{}
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		return false, extraLocalNetworkData, err
	}
	extraLocalNetworkDataPath := filepath.Join(clusterInfo.GetRootDataDir(), constants.ExtraLocalNetworkDataFilename)
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

func WriteExtraLocalNetworkData(cchainTeleporterMessengerAddress string, cchainTeleporterRegistryAddress string) error {
	clusterInfo, err := GetClusterInfo()
	if err != nil {
		return err
	}
	extraLocalNetworkDataPath := filepath.Join(clusterInfo.GetRootDataDir(), constants.ExtraLocalNetworkDataFilename)
	extraLocalNetworkData := ExtraLocalNetworkData{}
	if utils.FileExists(extraLocalNetworkDataPath) {
		var err error
		_, extraLocalNetworkData, err = GetExtraLocalNetworkData()
		if err != nil {
			return err
		}
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
