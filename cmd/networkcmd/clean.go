// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/interchain/relayer"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/spf13/cobra"
)

var hard bool

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

	cmd.Flags().BoolVar(
		&hard,
		"hard",
		false,
		"Also clean downloaded avalanchego and plugin binaries",
	)

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

	if hard {
		ux.Logger.PrintToUser("hard clean requested via flag, removing all downloaded avalanchego and plugin binaries")
		binDir := filepath.Join(app.GetBaseDir(), constants.AvalancheCliBinDir)
		cleanBins(binDir)
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
	deployedSubnets, err := subnet.GetLocallyDeployedSubnetsFromFile(app)
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
