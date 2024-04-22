// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package primarycmd

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const art = `
   _____       _____ _           _         _____
  / ____|     / ____| |         (_)       |  __ \
 | |   ______| |    | |__   __ _ _ _ __   | |__) |_ _ _ __ __ _ _ __ ___  ___ 
 | |  |______| |    | '_ \ / _  | | '_ \  |  ___/ _  | '__/ _  | '_   _ \/ __|
 | |____     | |____| | | | (_| | | | | | | |  | (_| | | | (_| | | | | | \__ \
  \_____|     \_____|_| |_|\__,_|_|_| |_| |_|   \__,_|_|  \__,_|_| |_| |_|___/
`

var describeSupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster}

// avalanche primary describe
func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Print details of the primary network configuration",
		Long:  `The subnet describe command prints details of the primary network configuration to the console.`,
		RunE:  describe,
		Args:  cobrautils.ExactArgs(0),
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
	var (
		teleporterMessengerAddress string
		teleporterRegistryAddress  string
	)
	if network.Kind == models.Local {
		extraLocalNetworkDataPath := app.GetExtraLocalNetworkDataPath()
		if utils.FileExists(extraLocalNetworkDataPath) {
			if extraLocalNetworkData, err := subnet.GetExtraLocalNetworkData(app); err != nil {
				return err
			} else {
				teleporterMessengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
				teleporterRegistryAddress = extraLocalNetworkData.CChainTeleporterRegistryAddress
			}
		}
	} else if network.ClusterName != "" {
		if clusterConfig, err := app.GetClusterConfig(network.ClusterName); err != nil {
			return err
		} else {
			teleporterMessengerAddress = clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress
			teleporterRegistryAddress = clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress
		}
	}
	blockchainID, err := subnet.GetChainID(network, "C")
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			networkUpMsg := ""
			if network.Kind != models.Fuji && network.Kind != models.Mainnet {
				networkUpMsg = fmt.Sprintf(" Is the %s up?", network.Name())
			}
			ux.Logger.RedXToUser("Could not connect to Primary Network at %s.%s", network.Endpoint, networkUpMsg)
			return nil
		}
		return err
	}
	blockchainIDHexEncoding := "0x" + hex.EncodeToString(blockchainID[:])
	rpcURL := network.CChainEndpoint()
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return err
	}
	evmChainID, err := evm.GetChainID(client)
	if err != nil {
		return err
	}
	k, err := key.LoadEwoq(network.ID)
	if err != nil {
		return err
	}
	address := k.C()
	privKey := hex.EncodeToString(k.Raw())
	balance, err := evm.GetAddressBalance(client, address)
	if err != nil {
		return err
	}
	balance = balance.Div(balance, big.NewInt(int64(units.Avax)))
	balanceStr := fmt.Sprintf("%.9f", float64(balance.Uint64())/float64(units.Avax))
	table := tablewriter.NewWriter(os.Stdout)
	header := []string{"Parameter", "Value"}
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoMergeCellsByColumnIndex([]int{0})
	table.Append([]string{"RPC URL", rpcURL})
	fmt.Println(rpcURL)
	codespaceURL, err := utils.GetCodespaceURL(rpcURL)
	if err != nil {
		return err
	}
	if codespaceURL != "" {
		table.Append([]string{"Codespace RPC URL", codespaceURL})
	}
	table.Append([]string{"EVM Chain ID", fmt.Sprint(evmChainID)})
	table.Append([]string{"TOKEN SYMBOL", "AVAX"})
	table.Append([]string{"Address", address})
	table.Append([]string{"Balance", balanceStr})
	table.Append([]string{"Private Key", privKey})
	table.Append([]string{"BlockchainID", blockchainID.String()})
	table.Append([]string{"BlockchainID", blockchainIDHexEncoding})
	if teleporterMessengerAddress != "" {
		table.Append([]string{"Teleporter Messenger Address", teleporterMessengerAddress})
	}
	if teleporterRegistryAddress != "" {
		table.Append([]string{"Teleporter Registry Address", teleporterRegistryAddress})
	}
	table.Render()
	return nil
}
