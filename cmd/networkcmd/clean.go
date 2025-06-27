// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"errors"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/interchain/relayer"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Stop the running local network and delete state",
		Long: `The network clean command shuts down your local, multi-node network. All deployed Subnets
shutdown and delete their state. You can restart the network by deploying a new Subnet
configuration.`,
		RunE: clean,
		Args: cobrautils.ExactArgs(0),
	}

	return cmd
}

func clean(*cobra.Command, []string) error {
	if err := localnet.LocalNetworkStop(app); err != nil && !errors.Is(err, localnet.ErrNetworkNotRunning) {
		return err
	} else if err == nil {
		ux.Logger.PrintToUser("Process terminated.")
	} else {
		ux.Logger.PrintToUser(logging.Red.Wrap("No network is running."))
	}

	if err := relayer.RelayerCleanup(
		app.GetLocalRelayerRunPath(models.Local),
		app.GetLocalRelayerLogPath(models.Local),
		app.GetLocalRelayerStorageDir(models.Local),
	); err != nil {
		return err
	}

	// Clean up signature aggregator
	if err := signatureaggregator.SignatureAggregatorCleanup(app, models.NewLocalNetwork()); err != nil {
		return err
	}

	if err := app.ResetPluginsDir(); err != nil {
		return err
	}

	if err := removeLocalDeployInfoFromSidecars(); err != nil {
		return err
	}

	snapshotPath := app.GetSnapshotPath(constants.DefaultSnapshotName)
	if err := os.RemoveAll(snapshotPath); err != nil {
		return err
	}

	clusterNames, err := localnet.GetRunningLocalClustersConnectedToLocalNetwork(app)
	if err != nil {
		return err
	}
	for _, clusterName := range clusterNames {
		if err := localnet.LocalClusterRemove(app, clusterName); err != nil {
			return err
		}
	}
	return nil
}

func removeLocalDeployInfoFromSidecars() error {
	// Remove all local deployment info from sidecar files
	deployedSubnets, err := subnet.GetDeployedSubnetsFromFile(app, models.Local.String())
	if err != nil {
		return err
	}

	for _, subnet := range deployedSubnets {
		sc, err := app.LoadSidecar(subnet)
		if err != nil {
			return err
		}

		delete(sc.Networks, models.Local.String())
		if err = app.UpdateSidecar(&sc); err != nil {
			return err
		}
	}
	return nil
}

func cleanBins(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		ux.Logger.PrintToUser("Removal failed: %s", err)
	} else {
		ux.Logger.PrintToUser("All existing binaries removed.")
	}
}
