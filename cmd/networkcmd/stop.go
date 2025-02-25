// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

type StopFlags struct {
	snapshotName string
	dontSave     bool
}

var stopFlags StopFlags

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the running local network and preserve state",
		Long: `The network stop command shuts down your local, multi-node network.

All deployed Subnets shutdown gracefully and save their state. If you provide the
--snapshot-name flag, the network saves its state under this named snapshot. You can
reload this snapshot with network start --snapshot-name <snapshotName>. Otherwise, the
network saves to the default snapshot, overwriting any existing state. You can reload the
default snapshot with network start.`,

		RunE: stop,
		Args: cobrautils.ExactArgs(0),
	}
	cmd.Flags().StringVar(&stopFlags.snapshotName, "snapshot-name", constants.DefaultSnapshotName, "name of snapshot to use to save network state into")
	cmd.Flags().BoolVar(&stopFlags.dontSave, "dont-save", false, "do not save snapshot, just stop the network")
	return cmd
}

func stop(*cobra.Command, []string) error {
	return Stop(stopFlags)
}

func Stop(flags StopFlags) error {
	if err := stopAndSaveNetwork(flags); err != nil {
		return err
	}

	if err := interchain.RelayerCleanup(
		app.GetLocalRelayerRunPath(models.Local),
		app.GetLocalRelayerLogPath(models.Local),
		app.GetLocalRelayerStorageDir(models.Local),
	); err != nil {
		return err
	}

	return nil
}

func stopAndSaveNetwork(flags StopFlags) error {
	networkDir, err := localnet.GetLocalNetworkDir(app)
	if err != nil {
		return err
	}

	if err := localnet.LocalNetworkStop(app); err != nil {
		return err
	}

	autoSave := app.Conf.GetConfigBoolValue(constants.ConfigSnapshotsAutoSaveKey)
	dontSave := autoSave || flags.dontSave

	if !dontSave {
		snapshotPath := app.GetSnapshotPath(flags.snapshotName)
		if err := localnet.TmpNetMigrate(networkDir, snapshotPath); err != nil {
			return err
		}
	}

	clusterNames, err := localnet.GetLocalNetworkClusters(app)
	if err != nil {
		return err
	}
	for _, clusterName := range clusterNames {
		if err := localnet.LocalClusterStop(app, clusterName); err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser("Network stopped successfully.")

	return nil
}
