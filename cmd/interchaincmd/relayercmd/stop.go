// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var stopNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Cluster,
	networkoptions.Fuji,
}

type StopFlags struct {
	Network networkoptions.NetworkFlags
}

var stopFlags StopFlags

// avalanche interchain relayer stop
func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stops AWM relayer",
		Long:  `Stops AWM relayer on the specified network (Currently only for local network, cluster).`,
		RunE:  stop,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &stopFlags.Network, true, stopNetworkOptions)
	return cmd
}

func stop(_ *cobra.Command, args []string) error {
	return CallStop(args, stopFlags, models.UndefinedNetwork)
}

func CallStop(_ []string, flags StopFlags, network models.Network) error {
	var err error
	if network == models.UndefinedNetwork {
		network, err = networkoptions.GetNetworkFromCmdLineFlags(
			app,
			"",
			flags.Network,
			false,
			false,
			stopNetworkOptions,
			"",
		)
		if err != nil {
			return err
		}
	}
	switch {
	case network.ClusterName != "":
		host, err := node.GetICMRelayerHost(app, network.ClusterName)
		if err != nil {
			return err
		}
		if err := ssh.RunSSHStopICMRelayerService(host); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Remote AWM Relayer on %s successfully stopped", host.GetCloudID())
	default:
		b, _, _, err := interchain.RelayerIsUp(
			app.GetLocalRelayerRunPath(network.Kind),
		)
		if err != nil {
			return err
		}
		if !b {
			return fmt.Errorf("there is no CLI-managed local AWM relayer running for %s", network.Kind)
		}
		if err := interchain.RelayerCleanup(
			app.GetLocalRelayerRunPath(network.Kind),
			app.GetLocalRelayerLogPath(network.Kind),
			app.GetLocalRelayerStorageDir(network.Kind),
		); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Local AWM Relayer successfully stopped for %s", network.Kind)
	}
	return nil
}
