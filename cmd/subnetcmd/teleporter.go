// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/vms/platformvm"

	"github.com/spf13/cobra"
)

// avalanche subnet teleporter
func newTeleporterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "teleporter",
		Short:             "Deploys teleporter to local network cchain",
		Long:              `Deploys teleporter to a local network cchain.`,
		SilenceUsage:      true,
		RunE:              deployTeleporter,
		PersistentPostRun: handlePostRun,
		Args:              cobra.ExactArgs(0),
	}
	return cmd
}

func getSubnetInfos(endpoint string, registryMap map[string]string) ([]teleporter.RelayerSubnetInfo, error) {
	subnetsInfo := []teleporter.RelayerSubnetInfo{}
	pClient := platformvm.NewClient(endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	blockChains, err := pClient.GetBlockchains(ctx)
	if err != nil {
		return nil, err
	}
	for _, chain := range blockChains {
		if chain.Name == "X-Chain" {
			continue
		}
		subnetsInfo = append(subnetsInfo, teleporter.RelayerSubnetInfo{
			SubnetID:                  chain.SubnetID.String(),
			BlockchainID:              chain.ID.String(),
			TeleporterRegistryAddress: registryMap[chain.Name],
		})
	}
	return subnetsInfo, nil
}

func getCChainSubnetInfo(endpoint string, teleporterRegistryAddress string) (*teleporter.RelayerSubnetInfo, error) {
	pClient := platformvm.NewClient(endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	blockChains, err := pClient.GetBlockchains(ctx)
	if err != nil {
		return nil, err
	}
	for _, chain := range blockChains {
		if chain.Name == "C-Chain" {
			return &teleporter.RelayerSubnetInfo{
				SubnetID:                  chain.SubnetID.String(),
				BlockchainID:              chain.ID.String(),
				TeleporterRegistryAddress: teleporterRegistryAddress,
			}, nil
		}
	}
	return nil, fmt.Errorf("C-Chain not found on primary network blockchains")
}

func deployTeleporter(cmd *cobra.Command, args []string) error {
	teleporterContractAddress := "0xF7cBd95f1355f0d8d659864b92e2e9fbfaB786f7"
	registryMap := map[string]string{
		"C-Chain": "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
		"pp1":     "0xcb65EF152B10ae00500EfDC7E4CD20358e64b233",
	}
	cChainSubnetInfo, err := getCChainSubnetInfo(constants.LocalAPIEndpoint, registryMap["C-Chain"])
	if err != nil {
		return err
	}
	_ = cChainSubnetInfo
	subnetsInfo, err := getSubnetInfos(constants.LocalAPIEndpoint, registryMap)
	if err != nil {
		return err
	}
	return teleporter.DeployRelayer(app, "v0.2.12", models.LocalNetwork, subnetsInfo, teleporterContractAddress)
}
