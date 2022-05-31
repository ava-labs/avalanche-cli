// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/ux"
)

var stopCmd = &cobra.Command{
	Use:   "stop [subnetName]",
	Short: "Stop the running local network and preserve state",
	Long: `The network stop command shuts down your local, multi-node network. 
The deployed named subnet will shutdown gracefully and save its state. The
network may be started again with network start [subnetName].`,

	RunE: stopNetwork,
	Args: cobra.ExactArgs(1),
}

func stopNetwork(cmd *cobra.Command, args []string) error {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}

	// snapshotName is currently the subnetName
	snapshotName := args[0]

	ctx := binutils.GetAsyncContext()

	_, err = cli.RemoveSnapshot(ctx, snapshotName)
	if err != nil {
		// TODO: when removing an existing snapshot we get an error, but in this case it is expected
		// It might be nicer to have some special field set in the response though rather than having to parse
		// the error string which is error prone
		if !strings.Contains(err.Error(), fmt.Sprintf("snapshot %s already exists", snapshotName)) {
			return fmt.Errorf("failed op network with a snapshot: %s", err)
		}
	}

	_, err = cli.SaveSnapshot(ctx, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to stop network with a snapshot: %s", err)
	}
	ux.Logger.PrintToUser("Network stopped successfully.")
	return nil
}
