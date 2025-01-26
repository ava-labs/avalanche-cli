// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"

	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

const dirTimestampFormat = "20060102_150405"

type StartFlags struct {
	UserProvidedAvagoVersion string
	SnapshotName             string
	AvagoBinaryPath          string
	RelayerBinaryPath        string
	RelayerVersion           string
	NumNodes                 uint32
}

var startFlags StartFlags

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a local network",
		Long: `The network start command starts a local, multi-node Avalanche network on your machine.

By default, the command loads the default snapshot. If you provide the --snapshot-name
flag, the network loads that snapshot instead. The command fails if the local network is
already running.`,

		RunE: start,
		Args: cobrautils.ExactArgs(0),
	}

	cmd.Flags().StringVar(
		&startFlags.UserProvidedAvagoVersion,
		"avalanchego-version",
		constants.DefaultAvalancheGoVersion,
		"use this version of avalanchego (ex: v1.17.12)",
	)
	cmd.Flags().StringVar(&startFlags.AvagoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().StringVar(&startFlags.RelayerBinaryPath, "relayer-path", "", "use this relayer binary path")
	cmd.Flags().StringVar(&startFlags.SnapshotName, "snapshot-name", constants.DefaultSnapshotName, "name of snapshot to use to start the network from")
	cmd.Flags().Uint32Var(&startFlags.NumNodes, "num-nodes", constants.LocalNetworkNumNodes, "number of nodes to be created on local network")
	cmd.Flags().StringVar(
		&startFlags.RelayerVersion,
		"relayer-version",
		constants.LatestPreReleaseVersionTag,
		"use this relayer version",
	)

	return cmd
}

func start(*cobra.Command, []string) error {
	return Start(startFlags, true)
}

func Start(flags StartFlags, printEndpoints bool) error {
	// verify is local network is bootstrapped
	if isBootstrapped, err := localnet.LocalNetworkIsBootstrapped(app); err != nil {
		return err
	} else if isBootstrapped {
		ux.Logger.PrintToUser("Network has already been booted.")
		return nil
	}

	// setup (install if needed) avalanchego binary
	avalancheGoBinPath, err := localnet.SetupAvalancheGoBinary(app, flags.UserProvidedAvagoVersion, flags.AvagoBinaryPath)
	if err != nil {
		return err
	}

	// do we want to continuously persist network onto snapshot
	autoSave := app.Conf.GetConfigBoolValue(constants.ConfigSnapshotsAutoSaveKey)

	if flags.SnapshotName == "" {
		flags.SnapshotName = constants.DefaultSnapshotName
	}
	snapshotPath := app.GetSnapshotPath(flags.SnapshotName)

	networkDir := ""
	if sdkutils.DirExists(snapshotPath) {
		ux.Logger.PrintToUser("Starting previously deployed and stopped snapshot")

		if autoSave {
			networkDir = snapshotPath
		} else {
			// create new tmp network directory on runs
			networkDir, err = mkDirWithTimestamp(filepath.Join(app.GetRunDir(), "network"))
			if err != nil {
				return err
			}
			if err := localnet.TmpNetMigrate(snapshotPath, networkDir); err != nil {
				return err
			}
		}

		_, extraLocalNetworkData, err := localnet.GetExtraLocalNetworkData(app, snapshotPath)
		if err != nil {
			return err
		}
		if extraLocalNetworkData.AvalancheGoPath == "" {
			if flags.SnapshotName == constants.DefaultSnapshotName {
				return fmt.Errorf("incompatible snapshot version. please cleanup with 'avalanche network clean'")
			} else {
				return fmt.Errorf("incompatible snapshot version. please cleanup dir under '%s'", snapshotPath)
			}
		}
		if flags.AvagoBinaryPath == "" &&
			flags.UserProvidedAvagoVersion == constants.DefaultAvalancheGoVersion &&
			extraLocalNetworkData.AvalancheGoPath != "" {
			avalancheGoBinPath = extraLocalNetworkData.AvalancheGoPath
		}

		ux.Logger.PrintToUser("AvalancheGo path: %s\n", avalancheGoBinPath)
		ux.Logger.PrintToUser("Booting Network. Wait until healthy...")

		// save network directory previous to execution
		if err := localnet.SaveLocalNetworkMeta(app, networkDir); err != nil {
			return err
		}
		// local network
		ctx, cancel := localnet.GetLocalNetworkDefaultContext()
		defer cancel()
		if _, err := localnet.TmpNetLoad(ctx, app.Log, networkDir, avalancheGoBinPath); err != nil {
			return err
		}
		if err := startLocalCluster(avalancheGoBinPath); err != nil {
			return err
		}
		if err := localnet.TmpNetSetDefaultAliases(ctx, networkDir); err != nil {
			return err
		}
		if b, relayerConfigPath, err := localnet.GetLocalNetworkRelayerConfigPath(app, networkDir); err != nil {
			return err
		} else if b {
			ux.Logger.PrintToUser("")
			relayerBinPath := flags.RelayerBinaryPath
			if relayerBinPath == "" {
				relayerBinPath = extraLocalNetworkData.RelayerPath
			}
			if relayerBinPath, err := interchain.DeployRelayer(
				flags.RelayerVersion,
				relayerBinPath,
				app.GetICMRelayerBinDir(),
				relayerConfigPath,
				app.GetLocalRelayerLogPath(models.Local),
				app.GetLocalRelayerRunPath(models.Local),
				app.GetLocalRelayerStorageDir(models.Local),
			); err != nil {
				return err
			} else if err := localnet.WriteExtraLocalNetworkData(app, "", "", relayerBinPath, "", ""); err != nil {
				return err
			}
		}
	} else {
		if flags.SnapshotName != constants.DefaultSnapshotName {
			return fmt.Errorf("snapshot %s does not exists", flags.SnapshotName)
		}

		// starting a new network from scratch
		if autoSave {
			networkDir = snapshotPath
			if err := os.MkdirAll(networkDir, constants.DefaultPerms755); err != nil {
				return err
			}
		} else {
			// create new tmp network directory on runs
			networkDir, err = mkDirWithTimestamp(filepath.Join(app.GetRunDir(), "network"))
			if err != nil {
				return err
			}
		}

		// get default network conf for NumNodes
		unparsedGenesis, upgradeBytes, defaultFlags, nodes, err := localnet.GetDefaultNetworkConf(flags.NumNodes)
		if err != nil {
			return err
		}
		// add node flags on CLI config info default network flags
		flagsFromCLIConfigJSON, err := app.Conf.LoadNodeConfig()
		if err != nil {
			return err
		}
		var flagsFromCLIConfig map[string]interface{}
		if err := json.Unmarshal([]byte(flagsFromCLIConfigJSON), &flagsFromCLIConfig); err != nil {
			return fmt.Errorf("invalid common node config JSON: %w", err)
		}
		maps.Copy(defaultFlags, flagsFromCLIConfig)
		// get plugins dir
		pluginDir := app.GetPluginsDir()
		// create local network
		ux.Logger.PrintToUser("AvalancheGo path: %s\n", avalancheGoBinPath)
		ux.Logger.PrintToUser("Booting Network. Wait until healthy...")
		// save network directory previous to execution
		if err := localnet.SaveLocalNetworkMeta(app, networkDir); err != nil {
			return err
		}
		// create network
		ctx, cancel := localnet.GetLocalNetworkDefaultContext()
		defer cancel()
		_, err = localnet.TmpNetCreate(
			ctx,
			app.Log,
			networkDir,
			avalancheGoBinPath,
			pluginDir,
			nodes,
			defaultFlags,
			unparsedGenesis,
			upgradeBytes,
		)
		if err != nil {
			return err
		}
	}

	if err := localnet.WriteExtraLocalNetworkData(app, networkDir, avalancheGoBinPath, "", "", ""); err != nil {
		return err
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Node logs directory: %s/<NodeID>/logs", networkDir)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Network ready to use.")
	ux.Logger.PrintToUser("")

	if printEndpoints {
		if err := localnet.PrintEndpoints(app, ux.Logger.PrintToUser, ""); err != nil {
			return err
		}
	}

	return nil
}

func startLocalCluster(avalancheGoBinPath string) error {
	blockchains, err := localnet.GetLocalNetworkBlockchainInfo(app)
	if err != nil {
		return err
	}
	if len(blockchains) > 0 {
		blockchainName := blockchains[0].Name
		clusterName := blockchainName + "-local-node-local-network"
		isLocal, err := node.CheckClusterIsLocal(app, clusterName)
		if err != nil {
			return err
		}
		if isLocal {
			if err = node.StartLocalNode(
				app,
				clusterName,
				avalancheGoBinPath,
				0,
				nil,
				node.ANRSettings{},
				node.AvalancheGoVersionSettings{},
				models.NewLocalNetwork(),
				networkoptions.NetworkFlags{},
				nil,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func mkDirWithTimestamp(dirPrefix string) (string, error) {
	currentTime := time.Now().Format(dirTimestampFormat)
	dirName := dirPrefix + "_" + currentTime
	return dirName, os.MkdirAll(dirName, constants.DefaultPerms755)
}
