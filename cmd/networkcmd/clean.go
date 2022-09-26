// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
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
		Long: `The network clean command shuts down your local, multi-node network. All
the deployed subnets will shutdown and delete their state. The network
may be started again by deploying a new subnet configuration.`,
		RunE:         clean,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}

	cmd.Flags().BoolVar(
		&hard,
		"hard",
		false,
		"Also clean downloaded avalanchego and plugin binaries",
	)

	return cmd
}

func clean(cmd *cobra.Command, args []string) error {
	app.Log.Info("killing gRPC server process...")

	if err := subnet.SetDefaultSnapshot(app.GetSnapshotsDir(), true); err != nil {
		app.Log.Warn("failed resetting default snapshot", zap.Error(err))
	}

	if err := binutils.KillgRPCServerProcess(app); err != nil {
		app.Log.Warn("failed killing server process", zap.Error(err))
	} else {
		ux.Logger.PrintToUser("Process terminated.")
	}

	if hard {
		ux.Logger.PrintToUser("hard clean requested via flag, removing all downloaded avalanchego and plugin binaries")
		binDir := filepath.Join(app.GetBaseDir(), constants.AvalancheCliBinDir)
		cleanBins(binDir)
		_ = killAllBackendsByName()
	} else {
		// Iterate over all installed avalanchego versions and remove all plugins from their
		// plugin dirs except for the c-chain plugin

		// Check if dir exists. If not, no work to be done
		if _, err := os.Stat(app.GetAvalanchegoBinDir()); errors.Is(err, os.ErrNotExist) {
			// path/to/whatever does *not* exist
			return nil
		}

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
