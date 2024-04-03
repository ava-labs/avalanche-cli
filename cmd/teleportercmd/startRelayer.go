// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var startRelayerNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster}

// avalanche teleporter relayer start
func newStartRelayerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "starts AWM relayer",
		Long:         `Starts AWM relayer on the specified network (Currently only for local network).`,
		SilenceUsage: true,
		RunE:         startRelayer,
		Args:         cobra.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, startRelayerNetworkOptions)
	return cmd
}

func startRelayer(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		globalNetworkFlags,
		false,
		startRelayerNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	switch {
	case network.Kind == models.Local:
		relayerIsUp, _, _, err := teleporter.RelayerIsUp(
			app.GetAWMRelayerRunPath(),
		)
		if err != nil {
			return err
		}
		if relayerIsUp {
			return fmt.Errorf("local AWM relayer is already running")
		}
		if err := teleporter.DeployRelayer(
			app.GetAWMRelayerBinDir(),
			app.GetAWMRelayerConfigPath(),
			app.GetAWMRelayerLogPath(),
			app.GetAWMRelayerRunPath(),
			app.GetAWMRelayerStorageDir(),
		); err != nil {
			return err
		}
		ux.Logger.PrintToUser("Local AWM Relayer successfully started")
		ux.Logger.PrintToUser("Logs can be found at %s", app.GetAWMRelayerLogPath())
	case network.ClusterName != "":
		host, err := node.GetAWMRelayerHost(app, network.ClusterName)
		if err != nil {
			return err
		}
		if err := ssh.RunSSHStartAWMRelayerService(host); err != nil {
			return err
		}
		ux.Logger.PrintToUser("Remote AWM Relayer on %s successfully started", host.GetCloudID())
	}
	return nil
}
