// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var (
	startNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Cluster,
		networkoptions.Fuji,
	}
	globalNetworkFlags networkoptions.NetworkFlags
	binPath            string
	version            string
)

// avalanche interchain relayer start
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "starts AWM relayer",
		Long:  `Starts AWM relayer on the specified network (Currently only for local network).`,
		RunE:  start,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, startNetworkOptions)
	cmd.Flags().StringVar(&binPath, "bin-path", "", "use the given relayer binary")
	cmd.Flags().StringVar(
		&version,
		"version",
		constants.LatestPreReleaseVersionTag,
		"version to use",
	)
	return cmd
}

func start(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		false,
		false,
		startNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	switch {
	case network.ClusterName != "":
		host, err := node.GetICMRelayerHost(app, network.ClusterName)
		if err != nil {
			return err
		}
		if err := ssh.RunSSHStartICMRelayerService(host); err != nil {
			return err
		}
		ux.Logger.GreenCheckmarkToUser("Remote AWM Relayer on %s successfully started", host.GetCloudID())
	default:
		if relayerIsUp, _, _, err := interchain.RelayerIsUp(
			app.GetLocalRelayerRunPath(network.Type),
		); err != nil {
			return err
		} else if relayerIsUp {
			return fmt.Errorf("local AWM relayer is already running for %s", network.Type)
		}
		localNetworkRootDir := ""
		if network.Type == models.Local {
			clusterInfo, err := localnet.GetClusterInfo()
			if err != nil {
				return err
			}
			localNetworkRootDir = clusterInfo.GetRootDataDir()
		}
		relayerConfigPath := app.GetLocalRelayerConfigPath(network.Type, localNetworkRootDir)
		if network.Type == models.Local && binPath == "" {
			if b, extraLocalNetworkData, err := localnet.GetExtraLocalNetworkData(""); err != nil {
				return err
			} else if b {
				binPath = extraLocalNetworkData.RelayerPath
			}
		}
		if !utils.FileExists(relayerConfigPath) {
			return fmt.Errorf("there is no relayer configuration available")
		} else if binPath, err := interchain.DeployRelayer(
			version,
			binPath,
			app.GetICMRelayerBinDir(),
			relayerConfigPath,
			app.GetLocalRelayerLogPath(network.Type),
			app.GetLocalRelayerRunPath(network.Type),
			app.GetLocalRelayerStorageDir(network.Type),
		); err != nil {
			return err
		} else if network.Type == models.Local {
			if err := localnet.WriteExtraLocalNetworkData("", binPath, "", ""); err != nil {
				return err
			}
		}
		ux.Logger.GreenCheckmarkToUser("Local AWM Relayer successfully started for %s", network.Type)
		ux.Logger.PrintToUser("Logs can be found at %s", app.GetLocalRelayerLogPath(network.Type))
	}
	return nil
}
