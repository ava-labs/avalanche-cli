// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"

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

func deployTeleporter(cmd *cobra.Command, args []string) error {
	teleporterContractAddress := "0xF7cBd95f1355f0d8d659864b92e2e9fbfaB786f7"
	registryMap := map[string]string{
		"C-Chain": "0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
		"pp1":     "0xcb65EF152B10ae00500EfDC7E4CD20358e64b233",
	}
	subnetID, blockchainID, err := subnet.GetChainIDs(constants.LocalAPIEndpoint, "C-Chain")
	if err != nil {
		return err
	}
	teleporterRegistryAddress := registryMap["C-Chain"]
	err = teleporter.UpdateRelayerConfig(
		app.GetAWMRelayerConfigPath(),
		app.GetAWMRelayerStorageDir(),
		models.LocalNetwork,
		subnetID,
		blockchainID,
		teleporterContractAddress,
		teleporterRegistryAddress,
	)
	if err != nil {
		return err
	}
	subnetID, blockchainID, err = subnet.GetChainIDs(constants.LocalAPIEndpoint, "pp1")
	if err != nil {
		return err
	}
	teleporterRegistryAddress = registryMap["pp1"]
	err = teleporter.UpdateRelayerConfig(
		app.GetAWMRelayerConfigPath(),
		app.GetAWMRelayerStorageDir(),
		models.LocalNetwork,
		subnetID,
		blockchainID,
		teleporterContractAddress,
		teleporterRegistryAddress,
	)
	if err != nil {
		return err
	}
	return nil
}
