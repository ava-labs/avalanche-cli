// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var startRelayerNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local}

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
	if network.Kind == models.Local {
		b, _, _, err := teleporter.RelayerIsUp(
			app.GetAWMRelayerRunPath(),
		)
		if err != nil {
			return err
		}
		if b {
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
	}
	return nil
}
