// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

type AddSubnetToRelayerServiceFlags struct {
	Network     networkoptions.NetworkFlags
	CloudNodeID string
}

var (
	addSubnetToRelayerServiceSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster, networkoptions.Fuji, networkoptions.Mainnet, networkoptions.Devnet}
	addSubnetToRelayerServiceFlags                   AddSubnetToRelayerServiceFlags
)

// avalanche teleporter relayer addSubnetToService
func newAddSubnetToRelayerServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addSubnetToService [subnetName]",
		Short: "Adds a subnet to the AWM relayer service configuration",
		Long:  `Adds a subnet to the AWM relayer service configuration".`,
		RunE:  addSubnetToRelayerService,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &addSubnetToRelayerServiceFlags.Network, true, addSubnetToRelayerServiceSupportedNetworkOptions)
	cmd.Flags().StringVar(&addSubnetToRelayerServiceFlags.CloudNodeID, "cloud-node-id", "", "generate a config to be used on given cloud node")
	return cmd
}

func addSubnetToRelayerService(_ *cobra.Command, args []string) error {
	return CallAddSubnetToRelayerService(args[0], addSubnetToRelayerServiceFlags)
}

func CallAddSubnetToRelayerService(subnetName string, flags AddSubnetToRelayerServiceFlags) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		flags.Network,
		true,
		addSubnetToRelayerServiceSupportedNetworkOptions,
		subnetName,
	)
	if err != nil {
		return err
	}

	relayerAddress, relayerPrivateKey, err := teleporter.GetRelayerKeyInfo(app.GetKeyPath(constants.AWMRelayerKeyName))
	if err != nil {
		return err
	}

	subnetID, chainID, messengerAddress, registryAddress, _, err := getSubnetParams(network, "c-chain")
	if err != nil {
		return err
	}

	configBasePath := ""
	storageBasePath := ""
	if flags.CloudNodeID != "" {
		storageBasePath = constants.CloudNodeCLIConfigBasePath
		configBasePath = app.GetNodeInstanceDirPath(flags.CloudNodeID)
	}

	configPath := app.GetAWMRelayerServiceConfigPath(configBasePath)
	if err := os.MkdirAll(filepath.Dir(configPath), constants.DefaultPerms755); err != nil {
		return err
	}
	ux.Logger.PrintToUser("updating configuration file %s", configPath)

	if err = teleporter.UpdateRelayerConfig(
		configPath,
		app.GetAWMRelayerServiceStorageDir(storageBasePath),
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
		configPath,
		app.GetAWMRelayerServiceStorageDir(storageBasePath),
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
