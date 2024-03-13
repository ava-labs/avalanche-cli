// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"

	"github.com/spf13/cobra"
)

var addSubnetToRelayerServiceSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Devnet, networkoptions.Fuji, networkoptions.Mainnet}

// avalanche teleporter relayer addSubnetToService
func newAddSubnetToRelayerServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "addSubnetToService [subnetName]",
		Short:        "Adds a subnet to the AWM relayer service configuration",
		Long:         `Adds a subnet to the AWM relayer service configuration".`,
		SilenceUsage: true,
		RunE:         addSubnetToRelayerService,
		Args:         cobra.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, addSubnetToRelayerServiceSupportedNetworkOptions)
	return cmd
}

func addSubnetToRelayerService(_ *cobra.Command, args []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		globalNetworkFlags,
		true,
		addSubnetToRelayerServiceSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	subnetName := args[0]

	relayerAddress, relayerPrivateKey, err := teleporter.GetRelayerKeyInfo(app.GetKeyPath(constants.AWMRelayerKeyName))
	if err != nil {
		return err
	}

	subnetID, chainID, messengerAddress, registryAddress, _, err := getSubnetParams(network, "c-chain")
	if err != nil {
		return err
	}

	if err = teleporter.UpdateRelayerConfig(
		app.GetAWMRelayerServiceConfigPath(),
		app.GetAWMRelayerStorageDir(),
		relayerAddress,
		relayerPrivateKey,
		network,
		subnetID.String(),
		chainID.String(),
		messengerAddress,
		registryAddress,
	); err != nil {
		return err
	}

	subnetID, chainID, messengerAddress, registryAddress, _, err = getSubnetParams(network, subnetName)
	if err != nil {
		return err
	}

	if err = teleporter.UpdateRelayerConfig(
		app.GetAWMRelayerServiceConfigPath(),
		app.GetAWMRelayerStorageDir(),
		relayerAddress,
		relayerPrivateKey,
		network,
		subnetID.String(),
		chainID.String(),
		messengerAddress,
		registryAddress,
	); err != nil {
		return err
	}

	return nil
}
