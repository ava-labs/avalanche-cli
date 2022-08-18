// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func newCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Stop the running local network and delete state",
		Long: `The network clean command shuts down your local, multi-node network. All
the deployed subnets will shutdown and delete their state. The network
may be started again by deploying a new subnet configuration.`,
		RunE:         clean,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}
}

func clean(cmd *cobra.Command, args []string) error {
	app.Log.Info("killing gRPC server process...")

	if err := subnet.SetDefaultSnapshot(app.GetSnapshotsDir(), true); err != nil {
		app.Log.Warn("failed resetting default snapshot: %s\n", err)
	}

	if err := binutils.KillgRPCServerProcess(app); err != nil {
		app.Log.Warn("failed killing server process: %s\n", err)
	} else {
		ux.Logger.PrintToUser("Process terminated.")
	}

	// iterate over all installed avalanchego versions and remove all plugins from their
	// plugin dirs except for the c-chain plugin
	installedVersions, err := os.ReadDir(app.GetAvalanchegoBinDir())
	if err != nil {
		return err
	}

	for _, avagoDir := range installedVersions {
		pluginDir := filepath.Join(app.GetAvalanchegoBinDir(), avagoDir.Name(), "plugins")
		installedPlugins, err := os.ReadDir(pluginDir)
		if err != nil {
			return err
		}
		for _, plugin := range installedPlugins {
			if plugin.Name() != constants.EVMPlugin {
				if err = os.Remove(filepath.Join(pluginDir, plugin.Name())); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
