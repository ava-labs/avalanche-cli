// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"fmt"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/spf13/cobra"
)

var (
	userProvidedAvagoVersion string
	snapshotName             string
	avagoBinaryPath          string
	numNodes                 uint32
)

const (
	latest  = "latest"
	jsonExt = ".json"
)

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a local network",
		Long: `The network start command starts a local, multi-node Avalanche network on your machine.

By default, the command loads the default snapshot. If you provide the --snapshot-name
flag, the network loads that snapshot instead. The command fails if the local network is
already running.`,

		RunE: StartNetwork,
		Args: cobrautils.ExactArgs(0),
	}

	cmd.Flags().StringVar(&userProvidedAvagoVersion, "avalanchego-version", latest, "use this version of avalanchego (ex: v1.17.12)")
	cmd.Flags().StringVar(&avagoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().StringVar(&snapshotName, "snapshot-name", constants.DefaultSnapshotName, "name of snapshot to use to start the network from")
	cmd.Flags().Uint32Var(&numNodes, "num-nodes", 1, "number of nodes to be created on local network")

	return cmd
}

func StartNetwork(*cobra.Command, []string) error {
	var (
		err          error
		avagoVersion string
	)

	if avagoBinaryPath == "" {
		avagoVersion, err = determineAvagoVersion(userProvidedAvagoVersion)
		if err != nil {
			return err
		}
	}

	sd := subnet.NewLocalDeployer(app, avagoVersion, avagoBinaryPath, "", false)

	// this takes about 2 secs
	if err := sd.StartServer(
		constants.ServerRunFileLocalNetworkPrefix,
		binutils.LocalNetworkGRPCServerPort,
		binutils.LocalNetworkGRPCGatewayPort,
		app.GetSnapshotsDir(),
		"",
	); err != nil {
		return err
	}

	// this takes about 1 secs
	needsRestart, avalancheGoBinPath, err := sd.SetupLocalEnv()
	if err != nil {
		return err
	}

	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}

	ctx, cancel := utils.GetANRContext()
	defer cancel()

	bootstrapped, err := localnet.CheckNetworkIsAlreadyBootstrapped(ctx, cli)
	if err != nil {
		return err
	}

	if bootstrapped {
		if !needsRestart {
			ux.Logger.PrintToUser("Network has already been booted.")
			return nil
		}
		if _, err := cli.Stop(ctx); err != nil {
			return err
		}
		if err := app.ResetPluginsDir(); err != nil {
			return err
		}
	}

	autoSave := app.Conf.GetConfigBoolValue(constants.ConfigSnapshotsAutoSaveKey)

	tmpDir, err := anrutils.MkDirWithTimestamp(filepath.Join(app.GetRunDir(), "network"))
	if err != nil {
		return err
	}
	rootDir := ""
	logDir := ""
	pluginDir := app.GetPluginsDir()
	nodeConfig, err := app.Conf.LoadNodeConfig()
	if err != nil {
		return err
	}
	if nodeConfig == "" {
		nodeConfig = "{}"
	}

	snapshotPath := filepath.Join(app.GetSnapshotsDir(), "anr-snapshot-"+snapshotName)
	if utils.DirectoryExists(snapshotPath) {
		var startMsg string
		if snapshotName == constants.DefaultSnapshotName {
			startMsg = "Starting previously deployed and stopped snapshot"
		} else {
			startMsg = fmt.Sprintf("Starting previously deployed and stopped snapshot %s...", snapshotName)
		}
		ux.Logger.PrintToUser(startMsg)

		if !autoSave {
			rootDir = tmpDir
		} else {
			logDir = tmpDir
		}

		ux.Logger.PrintToUser("Booting Network. Wait until healthy...")
		if _, err := cli.LoadSnapshot(
			ctx,
			snapshotName,
			autoSave,
			client.WithExecPath(avalancheGoBinPath),
			client.WithRootDataDir(rootDir),
			client.WithLogRootDir(logDir),
			client.WithReassignPortsIfUsed(true),
			client.WithPluginDir(pluginDir),
			client.WithGlobalNodeConfig(nodeConfig),
		); err != nil {
			return fmt.Errorf("failed to start network with the persisted snapshot: %w", err)
		}

		if b, relayerConfigPath, err := subnet.GetLocalNetworkRelayerConfigPath(app); err != nil {
			return err
		} else if b {
			ux.Logger.PrintToUser("")
			if err := teleporter.DeployRelayer(
				"latest",
				app.GetAWMRelayerBinDir(),
				relayerConfigPath,
				app.GetLocalRelayerLogPath(models.Local),
				app.GetLocalRelayerRunPath(models.Local),
				app.GetLocalRelayerStorageDir(models.Local),
			); err != nil {
				return err
			}
		}
	} else {
		// starting a new network from scratch
		if snapshotName != constants.DefaultSnapshotName {
			return fmt.Errorf("snapshot %s does not exists", snapshotName)
		}
		if autoSave {
			rootDir = snapshotPath
			logDir = tmpDir
		}
		ux.Logger.PrintToUser("Booting Network. Wait until healthy...")
		if _, err := cli.Start(
			ctx,
			avalancheGoBinPath,
			client.WithNumNodes(numNodes),
			client.WithExecPath(avalancheGoBinPath),
			client.WithRootDataDir(rootDir),
			client.WithLogRootDir(logDir),
			client.WithReassignPortsIfUsed(true),
			client.WithPluginDir(pluginDir),
			client.WithGlobalNodeConfig(nodeConfig),
		); err != nil {
			return err
		}
	}

	resp, err := cli.Status(ctx)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Node logs directory: %s/node<i>/logs", resp.ClusterInfo.LogRootDir)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Network ready to use.")
	ux.Logger.PrintToUser("")

	if err := localnet.PrintEndpoints(ux.Logger.PrintToUser, ""); err != nil {
		return err
	}

	return nil
}
