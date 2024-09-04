// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var (
	startNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster, networkoptions.Fuji}
	globalNetworkFlags  networkoptions.NetworkFlags
)

// avalanche teleporter relayer start
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "starts AWM relayer",
		Long:  `Starts AWM relayer on the specified network (Currently only for local network).`,
		RunE:  start,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, startNetworkOptions)
	return cmd
}

func start(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		false,
		startNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	switch {
	case network.Kind == models.Local || network.Kind == models.Fuji:
		if relayerIsUp, _, _, err := teleporter.RelayerIsUp(
			app.GetLocalRelayerRunPath(network.Kind),
		); err != nil {
			return err
		} else if relayerIsUp {
			return fmt.Errorf("local AWM relayer is already running for %s", network.Kind)
		}
		localNetworkRootDir := ""
		if network.Kind == models.Local {
			clusterInfo, err := localnet.GetClusterInfo()
			if err != nil {
				return err
			}
			localNetworkRootDir = clusterInfo.GetRootDataDir()
		}
		relayerConfigPath := app.GetLocalRelayerConfigPath(network.Kind, localNetworkRootDir)
		if !utils.FileExists(relayerConfigPath) {
			return fmt.Errorf("there is no relayer configuration available")
		} else if err := teleporter.DeployRelayer(
			"latest",
			app.GetAWMRelayerBinDir(),
			relayerConfigPath,
			app.GetLocalRelayerLogPath(network.Kind),
			app.GetLocalRelayerRunPath(network.Kind),
			app.GetLocalRelayerStorageDir(network.Kind),
		); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Local AWM Relayer successfully started for %s", network.Kind)
		ux.Logger.PrintToUser("Logs can be found at %s", app.GetLocalRelayerLogPath(network.Kind))
	case network.ClusterName != "":
		host, err := node.GetAWMRelayerHost(app, network.ClusterName)
		if err != nil {
			return err
		}
		if err := ssh.RunSSHStartAWMRelayerService(host); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Remote AWM Relayer on %s successfully started", host.GetCloudID())
	}
	return nil
}
