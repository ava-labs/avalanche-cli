// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var (
	startNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster}
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
	case network.Kind == models.Local:
		if relayerIsUp, _, _, err := teleporter.RelayerIsUp(
			app.GetAWMRelayerRunPath(),
		); err != nil {
			return err
		} else if relayerIsUp {
			return fmt.Errorf("local AWM relayer is already running")
		}
		if b, relayerConfigPath, err := subnet.GetAWMRelayerConfigPath(); err != nil {
			return err
		} else if !b {
			return fmt.Errorf("there is no relayer configuration available")
		} else if err := teleporter.DeployRelayer(
			"latest",
			app.GetAWMRelayerBinDir(),
			relayerConfigPath,
			app.GetAWMRelayerLogPath(),
			app.GetAWMRelayerRunPath(),
			app.GetAWMRelayerStorageDir(),
		); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Local AWM Relayer successfully started")
		ux.Logger.PrintToUser("Logs can be found at %s", app.GetAWMRelayerLogPath())
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
