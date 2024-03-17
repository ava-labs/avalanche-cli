// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package primarycmd

import (
	"os"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const art = `
  _____      _                              _   _      _                      _      _____
 |  __ \    (_)                            | \ | |    | |                    | |    |  __ \
 | |__) | __ _ _ __ ___   __ _ _ __ _   _  |  \| | ___| |___      _____  _ __| | __ | |__) |_ _ _ __ __ _ _ __ ___  ___
 |  ___/ '__| | '_   _ \ / _  | '__| | | | | .   |/ _ \ __\ \ /\ / / _ \| '__| |/ / |  ___/ _  | '__/ _  | '_   _ \/ __|
 | |   | |  | | | | | | | (_| | |  | |_| | | |\  |  __/ |_ \ V  V / (_) | |  |   <  | |  | (_| | | | (_| | | | | | \__ \
 |_|   |_|  |_|_| |_| |_|\__,_|_|   \__, | |_| \_|\___|\__| \_/\_/ \___/|_|  |_|\_\ |_|   \__,_|_|  \__,_|_| |_| |_|___/
                                     __/ |
                                    |___/
`

var describeSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster}

// avalanche primary describe
func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "describe",
		Short:        "Print details of the primary network configuration",
		Long:         `The subnet describe command prints details of the primary network configuration to the console.`,
		SilenceUsage: true,
		RunE:         describe,
		Args:         cobra.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, describeSupportedNetworkOptions)
	return cmd
}

func describe(_ *cobra.Command, _ []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		globalNetworkFlags,
		false,
		describeSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}
	fmt.Println(logging.LightBlue.Wrap(art))
	var (
		teleporterMessengerAddress string
		teleporterRegistryAddress  string
	)
	if network.Kind == models.Local {
		extraLocalNetworkData, err := subnet.GetExtraLocalNetworkData(app)
		if err != nil {
			return err
		}
		teleporterMessengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
		teleporterRegistryAddress = extraLocalNetworkData.CChainTeleporterRegistryAddress
	} else if network.ClusterName != "" {
		clusterConfig, err := app.GetClusterConfig(network.ClusterName)
		if err != nil {
			return err
		}
		teleporterMessengerAddress = clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress
		teleporterRegistryAddress = clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress
	}
	table := tablewriter.NewWriter(os.Stdout)
	header := []string{"Parameter", "Value"}
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.Append([]string{"C-Chain Teleporter Messenger Address", teleporterMessengerAddress})
	table.Append([]string{"C-Chain Teleporter Registry Address", teleporterRegistryAddress})
	table.Render()
	return nil
}
