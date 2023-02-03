// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/local"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/spf13/cobra"
)

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

		RunE:         StopNetwork,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&snapshotName, "snapshot-name", constants.DefaultSnapshotName, "name of snapshot to use to save network state into")
	return cmd
}

func StopNetwork(*cobra.Command, []string) error {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}

	ctx := binutils.GetAsyncContext()

	_, err = cli.RemoveSnapshot(ctx, snapshotName)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			ux.Logger.PrintToUser("Network already stopped.")
			return nil
		}
		// it we try to stop a network with a new snapshot name, remove snapshot
		// will fail, so we cover here that expected case
		if !server.IsServerError(err, local.ErrSnapshotNotFound) {
			return fmt.Errorf("failed stop network with a snapshot: %w", err)
		}
	}

	_, err = cli.SaveSnapshot(ctx, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to stop network with a snapshot: %w", err)
	}
	ux.Logger.PrintToUser("Network stopped successfully.")
	return nil
}
