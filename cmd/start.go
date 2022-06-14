// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"
	"path"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [snapshotName]",
	Short: "Starts a stopped local network",
	Long: `The network start command starts a local, multi-node Avalanche network
on your machine. If "snapshotName" is provided, that snapshot will be used for starting the network
if it can be found. Otherwise, the last saved unnamed (default) snapshot will be used. The command may fail if the local network
is already running or if no subnets have been deployed.`,

	RunE:         startNetwork,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
}

func startNetwork(cmd *cobra.Command, args []string) error {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}

	var snapshotName, startMsg string
	if len(args) > 0 {
		snapshotName = args[0]
		startMsg = fmt.Sprintf("Starting previously deployed and stopped snapshot %s...", snapshotName)
	} else {
		snapshotName = defaultSnapshotName
		startMsg = "Starting previously deployed and stopped snapshot"
	}

	ctx := binutils.GetAsyncContext()

	ux.Logger.PrintToUser(startMsg)
	rootDataDir := path.Join(app.GetBaseDir(), "runs")
	_, err = cli.LoadSnapshot(ctx, snapshotName, rootDataDir)
	if err != nil {
		return fmt.Errorf("failed to start network with the persisted snapshot: %s", err)
	}

	// TODO: this should probably be extracted from the deployer and
	// used as an independent helper
	sd := subnet.NewLocalSubnetDeployer(app)
	endpoints, err := sd.WaitForHealthy(ctx, cli, healthCheckInterval)
	if err != nil {
		return fmt.Errorf("failed waiting for network to become healthy: %s", err)
	}

	fmt.Println()
	ux.Logger.PrintToUser("Network ready to use. Local network node endpoints:")
	for _, u := range endpoints {
		ux.Logger.PrintToUser(u)
	}
	return nil
}
