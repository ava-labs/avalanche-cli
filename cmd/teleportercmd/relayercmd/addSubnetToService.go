// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/cmd/teleportercmd/bridgecmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

type AddSubnetToServiceFlags struct {
	Network     networkoptions.NetworkFlags
	CloudNodeID string
}

var (
	addSubnetToServiceSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster, networkoptions.Fuji, networkoptions.Mainnet, networkoptions.Devnet}
	addSubnetToServiceFlags                   AddSubnetToServiceFlags
)

// avalanche teleporter relayer addSubnetToService
func newAddSubnetToServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addSubnetToService [subnetName]",
		Short: "Adds a subnet to the AWM relayer service configuration",
		Long:  `Adds a subnet to the AWM relayer service configuration".`,
		RunE:  addSubnetToService,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &addSubnetToServiceFlags.Network, true, addSubnetToServiceSupportedNetworkOptions)
	cmd.Flags().StringVar(&addSubnetToServiceFlags.CloudNodeID, "cloud-node-id", "", "generate a config to be used on given cloud node")
	return cmd
}

func addSubnetToService(_ *cobra.Command, args []string) error {
	return CallAddSubnetToService(args[0], addSubnetToServiceFlags)
}

func CallAddSubnetToService(subnetName string, flags AddSubnetToServiceFlags) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		flags.Network,
		true,
		false,
		addSubnetToServiceSupportedNetworkOptions,
		subnetName,
	)
	if err != nil {
		return err
	}

	relayerAddress, relayerPrivateKey, err := teleporter.GetRelayerKeyInfo(app.GetKeyPath(constants.AWMRelayerKeyName))
	if err != nil {
		return err
	}

	_, subnetID, chainID, messengerAddress, registryAddress, _, err := bridgecmd.GetSubnetParams(network, "", true)
	if err != nil {
		return err
	}

	configBasePath := ""
	storageBasePath := ""
	if flags.CloudNodeID != "" {
		storageBasePath = constants.AWMRelayerDockerDir
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

	_, subnetID, chainID, messengerAddress, registryAddress, _, err = bridgecmd.GetSubnetParams(network, subnetName, false)
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
