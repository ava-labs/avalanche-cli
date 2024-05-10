// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var stopRelayerNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster}

// avalanche teleporter relayer stop
func newStopRelayerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stops AWM relayer",
		Long:  `Stops AWM relayer on the specified network (Currently only for local network, cluster).`,
		RunE:  stopRelayer,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, stopRelayerNetworkOptions)
	return cmd
}

func stopRelayer(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		false,
		stopRelayerNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	switch {
	case network.Kind == models.Local:
		b, _, _, err := teleporter.RelayerIsUp(
			app.GetAWMRelayerRunPath(),
		)
		if err != nil {
			return err
		}
		if !b {
			return fmt.Errorf("there is no CLI-managed local AWM relayer running")
		}
		if err := teleporter.RelayerCleanup(
			app.GetAWMRelayerRunPath(),
			app.GetAWMRelayerStorageDir(),
		); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Local AWM Relayer successfully stopped")
	case network.ClusterName != "":
		host, err := node.GetAWMRelayerHost(app, network.ClusterName)
		if err != nil {
			return err
		}
		if err := ssh.RunSSHStopAWMRelayerService(host); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Remote AWM Relayer on %s successfully stopped", host.GetCloudID())
	}
	return nil
}
