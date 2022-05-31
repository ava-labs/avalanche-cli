// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [subnetName]",
	Short: "Starts a stopped local network",
	Long: `The network start command starts a local, multi-node Avalanche network
on your machine. The named subnet (that has been previously deployed) will be
resumed with its old state. The command may fail if the local network
is already running or if no subnets have been deployed.`,

	RunE: startNetwork,
	Args: cobra.ExactArgs(1),
}

func startNetwork(cmd *cobra.Command, args []string) error {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}

	// snapshotName is currently the subnetName
	snapshotName := args[0]

	ctx := binutils.GetAsyncContext()

	ux.Logger.PrintToUser("Starting previously deployed but stopped subnet %s...", snapshotName)
	_, err = cli.LoadSnapshot(ctx, snapshotName)
	if err != nil {
		return fmt.Errorf("failed to start network with the persisted snapshot: %s", err)
	}

	// TODO: this should probably be extracted from the deployer and
	// used as an independent helper
	sd := subnet.NewLocalSubnetDeployer(log, baseDir)
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
