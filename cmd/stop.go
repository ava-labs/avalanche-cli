// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/ux"
)

var stopCmd = &cobra.Command{
	Use:   "stop [snapshotName]",
	Short: "Stop the running local network and preserve state",
	Long: `The network stop command shuts down your local, multi-node network. 
The deployed subnet will shutdown gracefully and save its state. 
If "snapshotName" is provided, the state will be saved under this named snapshot, which then can be
restarted with "network start <snapshotName>". Otherwise, the default snapshot will be created, or overwritten 
if it exists. The default snapshot can then be restarted without parameter ("network start").`,

	RunE:         stopNetwork,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
}

func stopNetwork(cmd *cobra.Command, args []string) error {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}

	var snapshotName string
	if len(args) > 0 {
		snapshotName = args[0]
	} else {
		snapshotName = constants.DefaultSnapshotName
	}

	ctx := binutils.GetAsyncContext()

	_, err = cli.RemoveSnapshot(ctx, snapshotName)
	if err != nil {
		if strings.Contains(err.Error(), "not bootstrapped") {
			ux.Logger.PrintToUser("Network already stopped.")
			return nil
		}
		// TODO: when removing an existing snapshot we get an error, but in this case it is expected
		// It might be nicer to have some special field set in the response though rather than having to parse
		// the error string which is error prone
		if !strings.Contains(err.Error(), fmt.Sprintf("snapshot %q does not exist", snapshotName)) {
			return fmt.Errorf("failed stop network with a snapshot: %s", err)
		}
	}

	_, err = cli.SaveSnapshot(ctx, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to stop network with a snapshot: %s", err)
	}
	ux.Logger.PrintToUser("Network stopped successfully.")
	return nil
}
