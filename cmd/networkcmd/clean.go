// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
	app.Log.Info("killing gRPC server process...")

	if err := binutils.KillgRPCServerProcess(
		app,
		binutils.LocalNetworkGRPCServerEndpoint,
		constants.ServerRunFileLocalNetworkPrefix,
	); err != nil {
		app.Log.Warn("failed killing server process", zap.Error(err))
	} else {
		ux.Logger.PrintToUser("Process terminated.")
	}

	if err := interchain.RelayerCleanup(
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
		_ = killAllBackendsByName()
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

	return node.DestroyLocalNetworkConnectedCluster(app)
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
	}
	ux.Logger.PrintToUser("All existing binaries removed.")
}

func killAllBackendsByName() error {
	procs, err := process.Processes()
	if err != nil {
		return err
	}
	regex := regexp.MustCompile(".* " + constants.BackendCmd + ".*")
	for _, p := range procs {
		name, err := p.Cmdline()
		if err != nil {
			// ignore errors for processes that just died (macos implementation)
			continue
		}
		if regex.MatchString(name) {
			if err := p.Terminate(); err != nil {
				return err
			}
		}
	}
	return nil
}
